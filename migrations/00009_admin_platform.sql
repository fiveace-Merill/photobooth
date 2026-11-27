-- +goose Up
-- Platform admin tables: separate from photographers. No photographer_id, no RLS.
-- Only admin/auth platform code should read/write these tables.

CREATE TABLE admins (
    id                 uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    email              citext      NOT NULL UNIQUE,
    password_hash      text        NOT NULL,
    totp_secret_enc    bytea,
    totp_confirmed_at  timestamptz,
    status             text        NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled')),
    last_login_at      timestamptz,
    created_at         timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE admin_sessions (
    token_hash   bytea       PRIMARY KEY CHECK (octet_length(token_hash) = 32),
    admin_id     uuid        NOT NULL REFERENCES admins(id) ON DELETE CASCADE,
    pending      bool        NOT NULL DEFAULT false,
    created_at   timestamptz NOT NULL DEFAULT now(),
    last_seen_at timestamptz NOT NULL DEFAULT now(),
    expires_at   timestamptz NOT NULL
);
CREATE INDEX admin_sessions_admin_idx ON admin_sessions (admin_id);
CREATE INDEX admin_sessions_expires_idx ON admin_sessions (expires_at);

CREATE TABLE admin_recovery_codes (
    id         uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    admin_id   uuid        NOT NULL REFERENCES admins(id) ON DELETE CASCADE,
    code_hash  bytea       NOT NULL CHECK (octet_length(code_hash) = 32),
    used_at    timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (admin_id, code_hash)
);
CREATE INDEX admin_recovery_codes_admin_idx ON admin_recovery_codes (admin_id);

CREATE TABLE waitlist_entries (
    id               uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    name             text        NOT NULL,
    contact_email    citext,
    contact_phone    text,
    photography_type text        NOT NULL,
    shoots_per_month text        NOT NULL,
    consent_at       timestamptz NOT NULL,
    consent_version  text        NOT NULL,
    status           text        NOT NULL DEFAULT 'new' CHECK (status IN ('new', 'invited', 'declined', 'spam')),
    invitation_id    uuid        REFERENCES invitations(id) ON DELETE SET NULL,
    invited_at       timestamptz,
    notes            text,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT waitlist_contact_required CHECK (contact_email IS NOT NULL OR contact_phone IS NOT NULL)
);
CREATE UNIQUE INDEX waitlist_contact_email_uidx ON waitlist_entries (contact_email) WHERE contact_email IS NOT NULL;
CREATE UNIQUE INDEX waitlist_contact_phone_uidx ON waitlist_entries (contact_phone) WHERE contact_phone IS NOT NULL;
CREATE INDEX waitlist_status_created_idx ON waitlist_entries (status, created_at DESC, id DESC);

CREATE TABLE admin_audit_log (
    id          uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    admin_id    uuid        REFERENCES admins(id) ON DELETE SET NULL,
    actor       text        NOT NULL CHECK (actor IN ('admin', 'cli')),
    action      text        NOT NULL,
    target_type text        NOT NULL,
    target_id   text,
    meta        jsonb       NOT NULL DEFAULT '{}'::jsonb,
    ip          text,
    created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX admin_audit_log_created_idx ON admin_audit_log (created_at DESC, id DESC);
CREATE INDEX admin_audit_log_action_idx ON admin_audit_log (action, created_at DESC);

-- +goose StatementBegin
CREATE FUNCTION forbid_admin_audit_mutation() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'admin_audit_log is append-only';
END;
$$;
-- +goose StatementEnd
CREATE TRIGGER admin_audit_no_update_delete
    BEFORE UPDATE OR DELETE ON admin_audit_log
    FOR EACH ROW EXECUTE FUNCTION forbid_admin_audit_mutation();

ALTER TABLE photographers
    ADD COLUMN last_login_at timestamptz,
    ADD COLUMN suspended_at timestamptz,
    ADD COLUMN suspended_reason text;

ALTER TABLE invitations
    ADD CONSTRAINT invitations_created_by_fkey FOREIGN KEY (created_by) REFERENCES admins(id) ON DELETE SET NULL;

-- +goose StatementBegin
CREATE FUNCTION admin_platform_stats() RETURNS TABLE (
    photographers_total bigint,
    photographers_active bigint,
    photographers_suspended bigint,
    shoots_total bigint,
    photos_total bigint,
    shoots_delivered_last_30_days bigint,
    storage_used_bytes_total bigint
) LANGUAGE sql STABLE SECURITY DEFINER SET search_path = public AS $$
    SELECT
        COUNT(*)::bigint AS photographers_total,
        COUNT(*) FILTER (WHERE status = 'active')::bigint AS photographers_active,
        COUNT(*) FILTER (WHERE status = 'suspended')::bigint AS photographers_suspended,
        (SELECT COUNT(*)::bigint FROM shoots) AS shoots_total,
        (SELECT COUNT(*)::bigint FROM photos) AS photos_total,
        (SELECT COUNT(*)::bigint FROM shoots WHERE status = 'delivered' AND delivered_at >= now() - interval '30 days') AS shoots_delivered_last_30_days,
        COALESCE(SUM(storage_used_bytes), 0)::bigint AS storage_used_bytes_total
    FROM photographers;
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION admin_photographer_usage(ids uuid[]) RETURNS TABLE (
    photographer_id uuid,
    shoots bigint,
    photos bigint,
    delivered bigint
) LANGUAGE sql STABLE SECURITY DEFINER SET search_path = public AS $$
    SELECT
        p.id AS photographer_id,
        (SELECT COUNT(*)::bigint FROM shoots s WHERE s.photographer_id = p.id) AS shoots,
        (SELECT COUNT(*)::bigint FROM photos ph WHERE ph.photographer_id = p.id) AS photos,
        (SELECT COUNT(*)::bigint FROM shoots s WHERE s.photographer_id = p.id AND s.status = 'delivered') AS delivered
    FROM photographers p
    WHERE p.id = ANY(ids);
$$;
-- +goose StatementEnd

REVOKE ALL ON FUNCTION admin_platform_stats() FROM PUBLIC;
REVOKE ALL ON FUNCTION admin_photographer_usage(uuid[]) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION admin_platform_stats() TO photobooth_app;
GRANT EXECUTE ON FUNCTION admin_photographer_usage(uuid[]) TO photobooth_app;

-- +goose Down
REVOKE ALL ON FUNCTION admin_photographer_usage(uuid[]) FROM photobooth_app;
REVOKE ALL ON FUNCTION admin_platform_stats() FROM photobooth_app;
DROP FUNCTION IF EXISTS admin_photographer_usage(uuid[]);
DROP FUNCTION IF EXISTS admin_platform_stats();
ALTER TABLE invitations DROP CONSTRAINT IF EXISTS invitations_created_by_fkey;
ALTER TABLE photographers
    DROP COLUMN IF EXISTS suspended_reason,
    DROP COLUMN IF EXISTS suspended_at,
    DROP COLUMN IF EXISTS last_login_at;
DROP TRIGGER IF EXISTS admin_audit_no_update_delete ON admin_audit_log;
DROP FUNCTION IF EXISTS forbid_admin_audit_mutation();
DROP TABLE IF EXISTS admin_audit_log;
DROP TABLE IF EXISTS waitlist_entries;
DROP TABLE IF EXISTS admin_recovery_codes;
DROP TABLE IF EXISTS admin_sessions;
DROP TABLE IF EXISTS admins;
