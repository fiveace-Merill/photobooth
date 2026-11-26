package shoots

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
)

type fakeClients struct{}

func (fakeClients) Exists(context.Context, pgtype.UUID, pgtype.UUID) (bool, error) { return true, nil }
func (fakeClients) NamesByIDs(context.Context, pgtype.UUID, []pgtype.UUID) (map[string]string, error) {
	return map[string]string{}, nil
}

func TestNormalizeStatusFilter(t *testing.T) {
	if _, err := normalizeStatusFilter("shot,editing"); err != nil {
		t.Fatal(err)
	}
	if _, err := normalizeStatusFilter("delivered"); err != nil {
		t.Fatal(err)
	}
	if _, err := normalizeStatusFilter("bogus"); err == nil {
		t.Fatal("expected error")
	}
}

func TestNormalizeCreateValidation(t *testing.T) {
	_, err := normalizeCreate(CreateInput{Title: "T", PriceMinor: -1, Currency: "KES", ClientID: pgtype.UUID{Valid: true}})
	if err == nil {
		t.Fatal("expected error")
	}
}
