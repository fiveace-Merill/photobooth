package database_test

import (
	"context"
	"testing"

	"github.com/fiveace-Merill/photobooth/internal/clients/store"
	"github.com/fiveace-Merill/photobooth/internal/platform/database"
	"github.com/fiveace-Merill/photobooth/internal/platform/testdb"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// TestWithTenant_Isolation proves the tenant transaction helper sets
// app.photographer_id correctly and that RLS prevents cross-tenant reads.
//
// This test uses real Postgres as the non-owner photobooth_app role, because
// table owners bypass RLS and would give a false sense of safety.
func TestWithTenant_Isolation(t *testing.T) {
	tdb := testdb.Open(t)
	ctx := context.Background()

	// Create two photographers as owner. The photographers table is the tenant
	// root and has no RLS, but we still use owner here because this is test setup.
	var p1, p2 pgtype.UUID
	err := tdb.Owner.QueryRow(ctx, `
		INSERT INTO photographers (email, password_hash, business_name, slug)
		VALUES ('a@example.com', 'hash', 'A', 'photog-a')
		RETURNING id`).Scan(&p1)
	if err != nil {
		t.Fatalf("insert photographer 1: %v", err)
	}
	err = tdb.Owner.QueryRow(ctx, `
		INSERT INTO photographers (email, password_hash, business_name, slug)
		VALUES ('b@example.com', 'hash', 'B', 'photog-b')
		RETURNING id`).Scan(&p2)
	if err != nil {
		t.Fatalf("insert photographer 2: %v", err)
	}

	// Each photographer creates a client through the clients sqlc queries,
	// running inside the shared tenant helper. This exercises the real query path.
	err = database.WithTenant(ctx, tdb.App, p1, func(tx pgx.Tx) error {
		q := store.New(tx)
		_, err := q.CreateClient(ctx, store.CreateClientParams{
			PhotographerID: p1,
			Name:           "Alice Client",
			Phone:          pgtype.Text{String: "+254700000001", Valid: true},
		})
		return err
	})
	if err != nil {
		t.Fatalf("create client for p1: %v", err)
	}

	err = database.WithTenant(ctx, tdb.App, p2, func(tx pgx.Tx) error {
		q := store.New(tx)
		_, err := q.CreateClient(ctx, store.CreateClientParams{
			PhotographerID: p2,
			Name:           "Bob Client",
			Phone:          pgtype.Text{String: "+254700000002", Valid: true},
		})
		return err
	})
	if err != nil {
		t.Fatalf("create client for p2: %v", err)
	}

	// Photographer 1 sees only their own client.
	err = database.WithTenant(ctx, tdb.App, p1, func(tx pgx.Tx) error {
		q := store.New(tx)
		clients, err := q.ListClients(ctx, p1)
		if err != nil {
			return err
		}
		if len(clients) != 1 {
			t.Fatalf("p1 saw %d clients, want 1", len(clients))
		}
		if clients[0].Name != "Alice Client" {
			t.Fatalf("p1 saw client %q, want Alice Client", clients[0].Name)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("list clients for p1: %v", err)
	}

	// Photographer 2 sees only their own client.
	err = database.WithTenant(ctx, tdb.App, p2, func(tx pgx.Tx) error {
		q := store.New(tx)
		clients, err := q.ListClients(ctx, p2)
		if err != nil {
			return err
		}
		if len(clients) != 1 {
			t.Fatalf("p2 saw %d clients, want 1", len(clients))
		}
		if clients[0].Name != "Bob Client" {
			t.Fatalf("p2 saw client %q, want Bob Client", clients[0].Name)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("list clients for p2: %v", err)
	}

	// No tenant context means no rows. We pass an invalid UUID so WithTenant
	// does not set app.photographer_id. RLS should return an empty result.
	err = database.WithTenant(ctx, tdb.App, pgtype.UUID{}, func(tx pgx.Tx) error {
		q := store.New(tx)
		// Pass p1's ID in the query arg: even with an explicit filter, RLS must
		// still prevent access when tenant context is missing.
		clients, err := q.ListClients(ctx, p1)
		if err != nil {
			return err
		}
		if len(clients) != 0 {
			t.Fatalf("no tenant context saw %d clients, want 0", len(clients))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("no tenant context query: %v", err)
	}
}
