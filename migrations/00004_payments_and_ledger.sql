-- +goose Up
-- Slice 5 (design-ahead): payments, append-only ledger, payouts.
-- You can apply this later; it is written now so slices 1-3 never need to be
-- reshaped to fit it.

CREATE TABLE payments (
    id                   uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    photographer_id      uuid NOT NULL,
    shoot_id             uuid NOT NULL,
    amount_minor         bigint NOT NULL CHECK (amount_minor > 0),
    currency             text   NOT NULL DEFAULT 'KES',
    provider             text   NOT NULL DEFAULT 'mpesa',
    payer_phone          text,
    status               text   NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'succeeded', 'failed', 'refunded')),
    -- Daraja's CheckoutRequestID: how we match the callback to this payment.
    provider_checkout_id text UNIQUE,
    -- M-PESA receipt number. UNIQUE is what makes duplicate callbacks harmless.
    provider_txn_id      text,
    -- Rates snapshotted at payment time, so later plan or pricing changes
    -- never rewrite history.
    commission_bps       int    NOT NULL,
    commission_minor     bigint NOT NULL CHECK (commission_minor >= 0),
    provider_fee_minor   bigint NOT NULL DEFAULT 0 CHECK (provider_fee_minor >= 0),
    created_at           timestamptz NOT NULL DEFAULT now(),
    completed_at         timestamptz,
    UNIQUE (photographer_id, id),
    FOREIGN KEY (photographer_id, shoot_id)
        REFERENCES shoots (photographer_id, id) ON DELETE RESTRICT
);
CREATE UNIQUE INDEX payments_provider_txn_uniq
    ON payments (provider, provider_txn_id) WHERE provider_txn_id IS NOT NULL;
CREATE INDEX payments_shoot_idx ON payments (photographer_id, shoot_id);

-- Raw provider callbacks, stored before any processing, for audit and replay.
-- Backend-only; not tenant-scoped because the tenant is unknown on arrival.
CREATE TABLE payment_callbacks (
    id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    provider    text  NOT NULL,
    payload     jsonb NOT NULL,
    payment_id  uuid,                -- filled in once matched
    processed_at timestamptz,
    received_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE payouts (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    photographer_id uuid NOT NULL REFERENCES photographers(id),
    amount_minor    bigint NOT NULL CHECK (amount_minor > 0),
    currency        text   NOT NULL DEFAULT 'KES',
    status          text   NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'sent', 'failed')),
    provider_ref    text,
    created_at      timestamptz NOT NULL DEFAULT now(),
    UNIQUE (photographer_id, id)
);

-- APPEND-ONLY double-entry ledger.
-- Signed amounts: positive = debit, negative = credit. All entries sharing a
-- txn_id MUST sum to zero (enforce in Go and test it; this is where bugs cost money).
--
-- Example, KES 3,000 paid, 8% commission, M-PESA fee passed to the photographer
-- (the amounts below are in cents):
--   provider_clearing    +297000   money we actually received, net of fee
--   platform_revenue      -24000   our commission
--   photographer_payable -273000   what we now owe the photographer
--                          = 0
-- Refunds and corrections are NEW entries with reverses_txn_id set, never edits.
CREATE TABLE ledger_entries (
    id               bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    txn_id           uuid   NOT NULL,
    photographer_id  uuid   NOT NULL,
    payment_id       uuid,
    payout_id        uuid,
    account          text   NOT NULL CHECK (account IN
        ('provider_clearing', 'platform_revenue', 'photographer_payable', 'provider_fees')),
    amount_minor     bigint NOT NULL CHECK (amount_minor <> 0),
    currency         text   NOT NULL DEFAULT 'KES',
    reverses_txn_id  uuid,
    memo             text,
    created_at       timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (photographer_id, payment_id) REFERENCES payments (photographer_id, id),
    FOREIGN KEY (photographer_id, payout_id)  REFERENCES payouts  (photographer_id, id)
);
CREATE INDEX ledger_txn_idx ON ledger_entries (txn_id);
CREATE INDEX ledger_photographer_account_idx ON ledger_entries (photographer_id, account);

-- +goose StatementBegin
CREATE FUNCTION forbid_ledger_mutation() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'ledger_entries is append-only';
END;
$$;
-- +goose StatementEnd
CREATE TRIGGER ledger_no_update_delete
    BEFORE UPDATE OR DELETE ON ledger_entries
    FOR EACH ROW EXECUTE FUNCTION forbid_ledger_mutation();

ALTER TABLE payments ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON payments
    USING (photographer_id = NULLIF(current_setting('app.photographer_id', true), '')::uuid);

ALTER TABLE payouts ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON payouts
    USING (photographer_id = NULLIF(current_setting('app.photographer_id', true), '')::uuid);

ALTER TABLE ledger_entries ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON ledger_entries
    USING (photographer_id = NULLIF(current_setting('app.photographer_id', true), '')::uuid);

-- +goose Down
DROP TRIGGER ledger_no_update_delete ON ledger_entries;
DROP FUNCTION forbid_ledger_mutation();
DROP TABLE ledger_entries;
DROP TABLE payouts;
DROP TABLE payment_callbacks;
DROP TABLE payments;
