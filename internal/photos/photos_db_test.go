package photos_test

import (
	"context"
	"testing"

	"github.com/fiveace-Merill/photobooth/internal/platform/database"
	"github.com/fiveace-Merill/photobooth/internal/platform/testdb"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestPhotosNoTenantContextZeroRows(t *testing.T) {
	tdb := testdb.Open(t)
	ctx := context.Background()
	var pid, cid, sid, photoID pgtype.UUID
	if err := tdb.Owner.QueryRow(ctx, `INSERT INTO photographers (email,password_hash,business_name,slug) VALUES ('p@example.com','h','P','studio-p') RETURNING id`).Scan(&pid); err != nil {
		t.Fatal(err)
	}
	if err := database.WithTenant(ctx, tdb.App, pid, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `INSERT INTO clients (photographer_id,name,phone) VALUES ($1,'C','254700000001') RETURNING id`, pid).Scan(&cid); err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, `INSERT INTO shoots (photographer_id,client_id,title,status,price_minor,currency) VALUES ($1,$2,'S','shot',0,'KES') RETURNING id`, pid, cid).Scan(&sid); err != nil {
			return err
		}
		return tx.QueryRow(ctx, `INSERT INTO photos (photographer_id,shoot_id,filename,content_type,size_bytes,original_key,status) VALUES ($1,$2,'a.jpg','image/jpeg',1,'k','ready') RETURNING id`, pid, sid).Scan(&photoID)
	}); err != nil {
		t.Fatal(err)
	}

	err := database.WithTenant(ctx, tdb.App, pgtype.UUID{}, func(tx pgx.Tx) error {
		var n int
		if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM photos WHERE photographer_id=$1`, pid).Scan(&n); err != nil {
			return err
		}
		if n != 0 {
			t.Fatalf("photos visible without tenant context: %d", n)
		}
		if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM shoot_events WHERE photographer_id=$1`, pid).Scan(&n); err != nil {
			return err
		}
		if n != 0 {
			t.Fatalf("shoot_events visible without tenant context: %d", n)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
