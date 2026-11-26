-- +goose Up
-- Slice 3a: photo processing fields and shoot event audit log.

ALTER TABLE photos
    ADD COLUMN error text,
    ADD COLUMN processed_at timestamptz,
    ADD COLUMN sort_key int NOT NULL DEFAULT 0,
    ADD COLUMN updated_at timestamptz NOT NULL DEFAULT now(),
    ADD COLUMN upload_id text;

CREATE INDEX photos_status_idx ON photos (photographer_id, shoot_id, status) WHERE deleted_at IS NULL;

CREATE TABLE shoot_events (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    photographer_id uuid NOT NULL,
    shoot_id        uuid NOT NULL,
    photo_id        uuid,
    kind            text NOT NULL,
    payload         jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at      timestamptz NOT NULL DEFAULT now(),
    UNIQUE (photographer_id, id),
    FOREIGN KEY (photographer_id, shoot_id)
        REFERENCES shoots (photographer_id, id) ON DELETE CASCADE,
    FOREIGN KEY (photo_id)
        REFERENCES photos (id) ON DELETE SET NULL
);
CREATE INDEX shoot_events_shoot_idx ON shoot_events (photographer_id, shoot_id, created_at DESC);

ALTER TABLE shoot_events ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON shoot_events
    USING (photographer_id = NULLIF(current_setting('app.photographer_id', true), '')::uuid);

-- +goose Down
DROP TABLE shoot_events;
DROP INDEX IF EXISTS photos_status_idx;
ALTER TABLE photos
    DROP COLUMN IF EXISTS upload_id,
    DROP COLUMN IF EXISTS updated_at,
    DROP COLUMN IF EXISTS sort_key,
    DROP COLUMN IF EXISTS processed_at,
    DROP COLUMN IF EXISTS error;
