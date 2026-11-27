# 0002: Photo processing memory defaults and River recovery

Status: accepted (Nov 2026).

## Context

Slice 3a processes originals into watermarked preview/thumb derivatives in River workers. The first pure-Go implementation decoded large JPEGs fully in memory. A 24MP (6000x4000) test image measured approximately:

- RSS: 276 MB
- heap_sys: 295 MB
- Go sys: 301 MB
- alloc delta: ~260 MB

This is acceptable for small instances only with conservative defaults. A kill -9 during processing also exposed River defaults that left jobs running until rescue after 1 hour.

## Decisions

- **Pure Go image processing remains default** for now. It has no native dependency and is simple to deploy.
- **Default `MAX_IMAGE_MEGAPIXELS=50`**. Reject larger decoded images as `invalid_image`. Operators can raise it only with measured memory headroom.
- **Default `RIVER_MAX_WORKERS=1`** in the API process to avoid concurrent decode peaks.
- **Document `GOMEMLIMIT`**. Recommend ~70% of available RAM for image workloads (example: `GOMEMLIMIT=700MiB` on 1GB-ish budget after OS overhead).
- **River recovery configured explicitly in `internal/jobs/river.go`**:
  - `JobTimeout=3m` (default 1m is tight for large photos)
  - `RescueStuckJobsAfter=5m` (default 1h; must be > JobTimeout)
  This makes kill -9/crash recovery observable and bounded.
- **No photo stays `processing` forever**. `process_photo` has a River WorkEnd finalize hook that marks discarded/cancelled jobs failed (`processing_failed`/`processing_timeout`). A periodic `reap_stale_processing_photos` safety net marks processing photos older than 30 minutes with no active River job as `processing_timeout`.
- **`process_photo` is idempotent**. Re-running overwrites preview/thumb objects at deterministic keys and marks the photo ready; it does not change quota.

## Upgrade path

If memory becomes the limit, move decode/resize to libvips (or a dedicated worker with larger memory limits) behind the same `ObjectStore`/worker interface. Keep the same keys, status transitions, and idempotency guarantees.
