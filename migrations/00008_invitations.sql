-- +goose Up
-- Invitations are platform auth records. They are not tenant-owned and have no
-- photographer_id RLS scope. Only internal/auth should read/write this table.

CREATE TABLE invitations (
    id              uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    email           citext      NOT NULL,
    token_hash      bytea       NOT NULL UNIQUE,
    plan_id         text        NOT NULL REFERENCES plans(id),
    note            text,
    expires_at      timestamptz NOT NULL,
    created_at      timestamptz NOT NULL DEFAULT now(),
    created_by      uuid,
    used_at         timestamptz,
    photographer_id uuid        REFERENCES photographers(id) ON DELETE SET NULL,
    revoked_at      timestamptz,

    CONSTRAINT invitations_token_hash_len CHECK (octet_length(token_hash) = 32),
    CONSTRAINT invitations_use_state CHECK (
        (used_at IS NULL OR revoked_at IS NULL)
    )
);

-- One active (unused, unrevoked) invitation per email. Expired active-state rows
-- are revoked by CreateInvitation before inserting a replacement.
CREATE UNIQUE INDEX invitations_one_active_email_idx
    ON invitations (email)
    WHERE used_at IS NULL AND revoked_at IS NULL;

CREATE INDEX invitations_expires_at_idx ON invitations (expires_at);
CREATE INDEX invitations_created_at_idx ON invitations (created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS invitations;
