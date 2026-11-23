-- +goose Up
-- Slice 3: photos in R2 and magic-link access for clients.

CREATE TABLE photos (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    photographer_id uuid NOT NULL,
    shoot_id        uuid NOT NULL,
    filename        text   NOT NULL,
    content_type    text   NOT NULL,
    size_bytes      bigint NOT NULL CHECK (size_bytes > 0),
    width           int,
    height          int,
    -- R2 object keys. The original is untouched; clients browse the others.
    original_key    text NOT NULL UNIQUE,
    preview_key     text,   -- watermarked, reduced size
    thumb_key       text,
    status          text NOT NULL DEFAULT 'uploading'
        CHECK (status IN ('uploading', 'processing', 'ready', 'failed')),
    deleted_at      timestamptz,  -- set by the expiry job after files are removed
    created_at      timestamptz NOT NULL DEFAULT now(),
    UNIQUE (photographer_id, id),
    FOREIGN KEY (photographer_id, shoot_id)
        REFERENCES shoots (photographer_id, id) ON DELETE CASCADE
);
CREATE INDEX photos_shoot_idx ON photos (photographer_id, shoot_id) WHERE deleted_at IS NULL;

-- Magic links. Only a hash of the token is stored.
CREATE TABLE gallery_links (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    photographer_id  uuid NOT NULL,
    shoot_id         uuid NOT NULL,
    token_hash       bytea NOT NULL UNIQUE,
    access_code_hash text,           -- optional extra code for sensitive shoots
    expires_at       timestamptz NOT NULL,
    revoked_at       timestamptz,
    last_viewed_at   timestamptz,
    created_at       timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (photographer_id, shoot_id)
        REFERENCES shoots (photographer_id, id) ON DELETE CASCADE
);
CREATE INDEX gallery_links_shoot_idx ON gallery_links (photographer_id, shoot_id);

ALTER TABLE photos ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON photos
    USING (photographer_id = NULLIF(current_setting('app.photographer_id', true), '')::uuid);

ALTER TABLE gallery_links ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON gallery_links
    USING (photographer_id = NULLIF(current_setting('app.photographer_id', true), '')::uuid);

-- A client opening a link has no session, so we don't yet know the tenant.
-- This narrow function is the ONLY way to resolve a link before tenant context
-- is set. It runs as the table owner (bypassing RLS) and returns just the IDs.
-- The API then sets app.photographer_id from the result and continues normally.
-- +goose StatementBegin
CREATE FUNCTION resolve_gallery_link(p_token_hash bytea)
RETURNS TABLE (photographer_id uuid, shoot_id uuid)
LANGUAGE sql SECURITY DEFINER SET search_path = public AS $$
    SELECT gl.photographer_id, gl.shoot_id
    FROM gallery_links gl
    WHERE gl.token_hash = p_token_hash
      AND gl.revoked_at IS NULL
      AND gl.expires_at > now();
$$;
-- +goose StatementEnd
REVOKE ALL ON FUNCTION resolve_gallery_link(bytea) FROM PUBLIC;
-- Then, once the app role exists:
--   GRANT EXECUTE ON FUNCTION resolve_gallery_link(bytea) TO photobooth_app;

-- +goose Down
DROP FUNCTION resolve_gallery_link(bytea);
DROP TABLE gallery_links;
DROP TABLE photos;
