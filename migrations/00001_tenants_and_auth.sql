-- +goose Up
-- Slice 1: photographers (tenants), plans, sessions.
--
-- Conventions used in every migration:
--   * UUID primary keys (gen_random_uuid is built in from Postgres 13).
--   * Money is BIGINT in the smallest unit (KES cents). Never floats.
--   * Every tenant-owned table has photographer_id and Row-Level Security.
--   * The app connects as a NON-owner role (e.g. photobooth_app). Table owners
--     bypass RLS, so migrations run as the owner and the API must not.

CREATE EXTENSION IF NOT EXISTS citext;

-- Plans are configuration in the database, not constants in Go.
-- Seed values are PLACEHOLDERS pending market research.
CREATE TABLE plans (
    id                  text PRIMARY KEY,
    name                text   NOT NULL,
    monthly_fee_minor   bigint NOT NULL CHECK (monthly_fee_minor >= 0),
    commission_bps      int    NOT NULL CHECK (commission_bps BETWEEN 0 AND 10000), -- 800 = 8%
    storage_quota_bytes bigint NOT NULL CHECK (storage_quota_bytes > 0),
    retention_days      int    NOT NULL CHECK (retention_days > 0),
    created_at          timestamptz NOT NULL DEFAULT now()
);

INSERT INTO plans (id, name, monthly_fee_minor, commission_bps, storage_quota_bytes, retention_days) VALUES
    ('free', 'Free', 0,      800, 5368709120,   30),   -- 5 GB
    ('pro',  'Pro',  100000, 300, 107374182400, 180);  -- KES 1,000/month, 100 GB

-- The tenant root. No photographer_id column and no RLS: this IS the tenant.
CREATE TABLE photographers (
    id                 uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email              citext NOT NULL UNIQUE,
    password_hash      text   NOT NULL,
    business_name      text   NOT NULL,
    slug               text   NOT NULL UNIQUE,       -- becomes slug.yourapp.com
    phone              text,
    plan_id            text   NOT NULL REFERENCES plans(id) DEFAULT 'free',
    storage_used_bytes bigint NOT NULL DEFAULT 0 CHECK (storage_used_bytes >= 0),
    status             text   NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'suspended')),
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT slug_format CHECK (slug ~ '^[a-z0-9][a-z0-9-]{1,30}[a-z0-9]$')
);

-- Store only a hash of the session token, never the token itself.
CREATE TABLE sessions (
    token_hash      bytea PRIMARY KEY,
    photographer_id uuid NOT NULL REFERENCES photographers(id) ON DELETE CASCADE,
    expires_at      timestamptz NOT NULL,
    created_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX sessions_photographer_idx ON sessions (photographer_id);
CREATE INDEX sessions_expires_idx ON sessions (expires_at);

-- +goose Down
DROP TABLE sessions;
DROP TABLE photographers;
DROP TABLE plans;
