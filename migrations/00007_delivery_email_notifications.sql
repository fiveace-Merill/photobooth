-- +goose Up
-- Slice 4: email delivery notifications. No message body or delivery URL/token is stored.

CREATE TABLE notifications (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    photographer_id     uuid NOT NULL,
    shoot_id            uuid NOT NULL,
    client_id           uuid NOT NULL,
    channel             text NOT NULL CHECK (channel IN ('email')),
    kind                text NOT NULL CHECK (kind IN ('delivery_link')),
    recipient           text NOT NULL,
    status              text NOT NULL CHECK (status IN ('queued', 'sent', 'failed')),
    provider_message_id text,
    error_code          text,
    created_at          timestamptz NOT NULL DEFAULT now(),
    sent_at             timestamptz,
    UNIQUE (photographer_id, id),
    FOREIGN KEY (photographer_id, shoot_id)
        REFERENCES shoots (photographer_id, id) ON DELETE CASCADE,
    FOREIGN KEY (photographer_id, client_id)
        REFERENCES clients (photographer_id, id) ON DELETE RESTRICT
);

CREATE INDEX notifications_shoot_created_idx
    ON notifications (photographer_id, shoot_id, created_at DESC);
CREATE INDEX notifications_photographer_created_idx
    ON notifications (photographer_id, channel, created_at DESC);
CREATE INDEX notifications_queued_created_idx
    ON notifications (created_at)
    WHERE status = 'queued';

ALTER TABLE notifications ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON notifications
    USING (photographer_id = NULLIF(current_setting('app.photographer_id', true), '')::uuid);

-- +goose Down
DROP TABLE notifications;
