-- +goose Up
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION reap_stale_processing_photos()
RETURNS integer
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = public
AS $$
DECLARE
  n integer;
BEGIN
  EXECUTE $q$
    WITH stale AS (
      SELECT p.id
      FROM photos p
      WHERE p.status = 'processing'
        AND p.deleted_at IS NULL
        AND p.updated_at < now() - interval '30 minutes'
        AND NOT EXISTS (
          SELECT 1 FROM river_job j
          WHERE j.kind = 'process_photo'
            AND j.state IN ('available','running','retryable','scheduled','pending')
            AND (j.args->>'photoId') = p.id::text
        )
      FOR UPDATE SKIP LOCKED
    ),
    upd AS (
      UPDATE photos p
      SET status = 'failed',
          error = 'processing_timeout',
          updated_at = now()
      FROM stale
      WHERE p.id = stale.id
        AND p.status = 'processing'
      RETURNING 1
    )
    SELECT COUNT(*)::integer FROM upd
  $q$ INTO n;
  RETURN COALESCE(n, 0);
END;
$$;

REVOKE ALL ON FUNCTION reap_stale_processing_photos() FROM PUBLIC;
GRANT EXECUTE ON FUNCTION reap_stale_processing_photos() TO photobooth_app;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP FUNCTION IF EXISTS reap_stale_processing_photos();
-- +goose StatementEnd
