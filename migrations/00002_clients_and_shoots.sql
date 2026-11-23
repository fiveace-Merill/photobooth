-- +goose Up
-- Slice 2: the client catalogue and shoots with status tracking.
--
-- TENANT-SAFETY TRICK: each tenant table gets UNIQUE (photographer_id, id), and
-- children reference the parent with a COMPOSITE foreign key. Postgres then
-- refuses to link a shoot to another photographer's client, even if a bug in
-- the Go code tries to.

CREATE TABLE clients (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    photographer_id uuid NOT NULL REFERENCES photographers(id) ON DELETE CASCADE,
    name            text NOT NULL,
    phone           text,
    email           citext,
    notes           text,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    UNIQUE (photographer_id, id),
    CONSTRAINT client_has_contact CHECK (phone IS NOT NULL OR email IS NOT NULL)
);
CREATE INDEX clients_photographer_name_idx ON clients (photographer_id, name);
-- Same phone twice for one photographer is almost certainly a duplicate client.
CREATE UNIQUE INDEX clients_photographer_phone_uniq
    ON clients (photographer_id, phone) WHERE phone IS NOT NULL;

CREATE TABLE shoots (
    id                     uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    photographer_id        uuid NOT NULL,
    client_id              uuid NOT NULL,
    title                  text NOT NULL,
    shoot_date             date,
    -- Answers problem #1 (forgetting to share): a due date plus a status the
    -- reminder job can scan for overdue work.
    status                 text NOT NULL DEFAULT 'shot'
        CHECK (status IN ('shot', 'editing', 'ready', 'delivered', 'expired')),
    due_date               date,
    price_minor            bigint NOT NULL DEFAULT 0 CHECK (price_minor >= 0),
    currency               text   NOT NULL DEFAULT 'KES',
    -- Set ONLY by server code after a verified payment, never from the browser.
    originals_unlocked_at  timestamptz,
    delivered_at           timestamptz,
    created_at             timestamptz NOT NULL DEFAULT now(),
    updated_at             timestamptz NOT NULL DEFAULT now(),
    UNIQUE (photographer_id, id),
    FOREIGN KEY (photographer_id, client_id)
        REFERENCES clients (photographer_id, id) ON DELETE RESTRICT
);
CREATE INDEX shoots_client_idx ON shoots (photographer_id, client_id);
-- Reminder job: "which of my shoots are overdue and not yet delivered?"
CREATE INDEX shoots_status_due_idx ON shoots (photographer_id, status, due_date);

-- Row-Level Security: the API sets app.photographer_id at the start of every
-- transaction (see notes). If it is unset, NULLIF makes it NULL and NO rows match.
ALTER TABLE clients ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON clients
    USING (photographer_id = NULLIF(current_setting('app.photographer_id', true), '')::uuid);

ALTER TABLE shoots ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON shoots
    USING (photographer_id = NULLIF(current_setting('app.photographer_id', true), '')::uuid);

-- +goose Down
DROP TABLE shoots;
DROP TABLE clients;
