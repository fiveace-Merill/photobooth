// Package database owns the pgx connection pool and the shared tenant transaction helper.
//
// The helper starts a transaction and runs
//
//	SELECT set_config('app.photographer_id', $1, true)
//
// so row-level security applies. Every tenant-data request must go through it.
package database
