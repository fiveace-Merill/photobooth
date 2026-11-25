package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// WithTenant runs fn inside a database transaction after setting the Postgres
// GUC app.photographer_id. Row-Level Security policies compare this value to
// photographer_id columns.
//
// The third argument to set_config is true, which means LOCAL to the current
// transaction. That is critical with connection pools: pooled connections are
// reused, and a session-level setting could leak tenant context to another request.
// A transaction-local setting disappears when the transaction commits/rolls back.
//
// If photographerID is uuid.Nil, we intentionally do not set tenant context.
// With RLS policies written as NULLIF(current_setting(...), ”)::uuid, an unset
// value matches no rows. That gives us "no tenant context means no rows".
func WithTenant(ctx context.Context, pool *pgxpool.Pool, photographerID pgtype.UUID, fn func(pgx.Tx) error) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	// Rollback is safe after Commit; pgx returns pgx.ErrTxClosed and we ignore it.
	// defer ensures we don't leak a transaction when fn returns early.
	defer func() { _ = tx.Rollback(ctx) }()

	if photographerID.Valid {
		// We pass the UUID as text and cast in SQL so the value is unambiguous.
		// set_config's third arg (true) makes it LOCAL to this transaction.
		_, err := tx.Exec(ctx, "SELECT set_config('app.photographer_id', $1::text, true)", photographerID)
		if err != nil {
			return fmt.Errorf("set tenant context: %w", err)
		}
	}

	if err := fn(tx); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}
