package clients

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/fiveace-Merill/photobooth/internal/clients/store"
	"github.com/fiveace-Merill/photobooth/internal/platform/database"
	platformphone "github.com/fiveace-Merill/photobooth/internal/platform/phone"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound     = errors.New("client not found")
	ErrInvalidInput = errors.New("invalid input")
	ErrPhoneExists  = errors.New("client phone exists")
	ErrHasShoots    = errors.New("client has shoots")
)

type PhoneConflictError struct {
	ExistingID string
}

func (e *PhoneConflictError) Error() string { return ErrPhoneExists.Error() }
func (e *PhoneConflictError) Unwrap() error { return ErrPhoneExists }

// NameLookup is the small interface other packages can use to resolve client names.
// Keeping it narrow avoids leaking sqlc/store types across domain boundaries.
type NameLookup interface {
	NamesByIDs(ctx context.Context, photographerID pgtype.UUID, ids []pgtype.UUID) (map[string]string, error)
}

// ClientLookup is the small interface other packages can use to verify ownership.
type ClientLookup interface {
	Exists(ctx context.Context, photographerID pgtype.UUID, id pgtype.UUID) (bool, error)
}

type Service struct {
	pool *pgxpool.Pool
}

func NewService(pool *pgxpool.Pool) *Service { return &Service{pool: pool} }

type CreateInput struct {
	Name  string
	Phone *string
	Email *string
	Notes *string
}

type PatchInput struct {
	Name  *string
	Phone *string
	Email *string
	Notes *string
}

type ListResult struct {
	Items  []store.ListClientsRow
	Total  int64
	Limit  int
	Offset int
}

func (s *Service) List(ctx context.Context, photographerID pgtype.UUID, q string, limit, offset int) (ListResult, error) {
	limit, offset = normalizePage(limit, offset)
	var out ListResult
	err := database.WithTenant(ctx, s.pool, photographerID, func(tx pgx.Tx) error {
		rows, err := store.New(tx).ListClients(ctx, store.ListClientsParams{
			PhotographerID: photographerID,
			Limit:          int32(limit),
			Offset:         int32(offset),
			Q:              textArg(q),
		})
		if err != nil {
			return err
		}
		out.Items = rows
		out.Limit = limit
		out.Offset = offset
		if len(rows) > 0 {
			out.Total = rows[0].Total
		} else if offset == 0 {
			out.Total = 0
		} else {
			// count query for empty non-first page not necessary; keep simple by a count
			// fallback: requery with limit 1 offset 0 not ideal. Instead do separate count.
			c, err := countClients(ctx, tx, photographerID, q)
			if err != nil {
				return err
			}
			out.Total = c
		}
		return nil
	})
	return out, err
}

func countClients(ctx context.Context, tx pgx.Tx, photographerID pgtype.UUID, q string) (int64, error) {
	var total int64
	err := tx.QueryRow(ctx, `
SELECT COUNT(*)
FROM clients
WHERE photographer_id = $1
  AND (
    $2::text IS NULL
    OR name ILIKE '%' || $2::text || '%'
    OR COALESCE(phone, '') ILIKE '%' || $2::text || '%'
    OR COALESCE(email::text, '') ILIKE '%' || $2::text || '%'
  )`, photographerID, textArg(q)).Scan(&total)
	return total, err
}

func (s *Service) Create(ctx context.Context, photographerID pgtype.UUID, in CreateInput) (store.Client, error) {
	normalized, err := normalizeClientInput(in.Name, in.Phone, in.Email, in.Notes)
	if err != nil {
		return store.Client{}, err
	}
	var created store.Client
	err = database.WithTenant(ctx, s.pool, photographerID, func(tx pgx.Tx) error {
		q := store.New(tx)
		if normalized.Phone.Valid {
			if existing, e := q.GetClientByPhone(ctx, store.GetClientByPhoneParams{PhotographerID: photographerID, Phone: normalized.Phone}); e == nil {
				return &PhoneConflictError{ExistingID: uuidString(existing.ID)}
			}
		}
		if _, err := tx.Exec(ctx, "SAVEPOINT client_write"); err != nil {
			return err
		}
		c, err := q.CreateClient(ctx, store.CreateClientParams{
			PhotographerID: photographerID,
			Name:           normalized.Name,
			Phone:          normalized.Phone,
			Email:          normalized.Email,
			Notes:          normalized.Notes,
		})
		if err != nil {
			_, _ = tx.Exec(ctx, "ROLLBACK TO SAVEPOINT client_write")
			return mapClientWriteErr(ctx, q, photographerID, normalized.Phone, err)
		}
		_, _ = tx.Exec(ctx, "RELEASE SAVEPOINT client_write")
		created = c
		return nil
	})
	return created, err
}

func (s *Service) Get(ctx context.Context, photographerID, id pgtype.UUID) (store.Client, error) {
	var out store.Client
	err := database.WithTenant(ctx, s.pool, photographerID, func(tx pgx.Tx) error {
		c, err := store.New(tx).GetClient(ctx, store.GetClientParams{PhotographerID: photographerID, ID: id})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		out = c
		return nil
	})
	return out, err
}

func (s *Service) Update(ctx context.Context, photographerID, id pgtype.UUID, in PatchInput) (store.Client, error) {
	var out store.Client
	err := database.WithTenant(ctx, s.pool, photographerID, func(tx pgx.Tx) error {
		q := store.New(tx)
		cur, err := q.GetClient(ctx, store.GetClientParams{PhotographerID: photographerID, ID: id})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		merged := mergeClient(cur, in)
		normalized, err := normalizeClientInput(merged.Name, optionalText(merged.Phone), optionalText(merged.Email), optionalText(merged.Notes))
		if err != nil {
			return err
		}
		if normalized.Phone.Valid {
			if existing, e := q.GetClientByPhone(ctx, store.GetClientByPhoneParams{PhotographerID: photographerID, Phone: normalized.Phone}); e == nil && uuidString(existing.ID) != uuidString(id) {
				return &PhoneConflictError{ExistingID: uuidString(existing.ID)}
			}
		}
		if _, err := tx.Exec(ctx, "SAVEPOINT client_write"); err != nil {
			return err
		}
		c, err := q.UpdateClient(ctx, store.UpdateClientParams{
			PhotographerID: photographerID,
			ID:             id,
			Name:           normalized.Name,
			Phone:          normalized.Phone,
			Email:          normalized.Email,
			Notes:          normalized.Notes,
		})
		if err != nil {
			_, _ = tx.Exec(ctx, "ROLLBACK TO SAVEPOINT client_write")
			return mapClientWriteErr(ctx, q, photographerID, normalized.Phone, err)
		}
		_, _ = tx.Exec(ctx, "RELEASE SAVEPOINT client_write")
		out = c
		return nil
	})
	return out, err
}

func (s *Service) Delete(ctx context.Context, photographerID, id pgtype.UUID) error {
	return database.WithTenant(ctx, s.pool, photographerID, func(tx pgx.Tx) error {
		q := store.New(tx)
		rows, err := q.DeleteClient(ctx, store.DeleteClientParams{PhotographerID: photographerID, ID: id})
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23503" {
				return ErrHasShoots
			}
			return err
		}
		if rows == 0 {
			return ErrNotFound
		}
		return nil
	})
}

func (s *Service) Exists(ctx context.Context, photographerID, id pgtype.UUID) (bool, error) {
	var exists bool
	err := database.WithTenant(ctx, s.pool, photographerID, func(tx pgx.Tx) error {
		v, err := store.New(tx).ClientExists(ctx, store.ClientExistsParams{PhotographerID: photographerID, ID: id})
		exists = v
		return err
	})
	return exists, err
}

func (s *Service) NamesByIDs(ctx context.Context, photographerID pgtype.UUID, ids []pgtype.UUID) (map[string]string, error) {
	out := map[string]string{}
	if len(ids) == 0 {
		return out, nil
	}
	err := database.WithTenant(ctx, s.pool, photographerID, func(tx pgx.Tx) error {
		rows, err := store.New(tx).NamesByIDs(ctx, store.NamesByIDsParams{PhotographerID: photographerID, Ids: ids})
		if err != nil {
			return err
		}
		for _, r := range rows {
			out[uuidString(r.ID)] = r.Name
		}
		return nil
	})
	return out, err
}

type normalizedClient struct {
	Name  string
	Phone pgtype.Text
	Email pgtype.Text
	Notes pgtype.Text
}

func normalizeClientInput(name string, phone, email, notes *string) (normalizedClient, error) {
	name = strings.TrimSpace(name)
	if len(name) < 1 || len(name) > 120 {
		return normalizedClient{}, fmt.Errorf("%w: name must be 1-120 characters", ErrInvalidInput)
	}
	var p pgtype.Text
	if phone != nil {
		v := strings.TrimSpace(*phone)
		if v != "" {
			n, err := platformphone.Normalize(v)
			if err != nil {
				return normalizedClient{}, fmt.Errorf("%w: invalid phone", ErrInvalidInput)
			}
			p = pgtype.Text{String: n, Valid: true}
		}
	}
	var e pgtype.Text
	if email != nil {
		v := strings.TrimSpace(*email)
		if v != "" {
			if !strings.Contains(v, "@") || len(v) > 320 {
				return normalizedClient{}, fmt.Errorf("%w: invalid email", ErrInvalidInput)
			}
			e = pgtype.Text{String: strings.ToLower(v), Valid: true}
		}
	}
	var n pgtype.Text
	if notes != nil {
		v := strings.TrimSpace(*notes)
		if len(v) > 2000 {
			return normalizedClient{}, fmt.Errorf("%w: notes must be at most 2000 characters", ErrInvalidInput)
		}
		if v != "" {
			n = pgtype.Text{String: v, Valid: true}
		}
	}
	if !p.Valid && !e.Valid {
		return normalizedClient{}, fmt.Errorf("%w: phone or email is required", ErrInvalidInput)
	}
	return normalizedClient{Name: name, Phone: p, Email: e, Notes: n}, nil
}

func mergeClient(cur store.Client, in PatchInput) store.Client {
	if in.Name != nil {
		cur.Name = *in.Name
	}
	if in.Phone != nil {
		cur.Phone = textFromPtr(in.Phone)
	}
	if in.Email != nil {
		cur.Email = textFromPtr(in.Email)
	}
	if in.Notes != nil {
		cur.Notes = textFromPtr(in.Notes)
	}
	return cur
}

func mapClientWriteErr(ctx context.Context, q *store.Queries, photographerID pgtype.UUID, phone pgtype.Text, err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		if phone.Valid {
			existing, e := q.GetClientByPhone(ctx, store.GetClientByPhoneParams{PhotographerID: photographerID, Phone: phone})
			if e == nil {
				return &PhoneConflictError{ExistingID: uuidString(existing.ID)}
			}
			// Keep a stable conflict code even if lookup fails unexpectedly.
			_ = pgErr
		}
		return ErrPhoneExists
	}
	if errors.As(err, &pgErr) && pgErr.Code == "23514" {
		return fmt.Errorf("%w: phone or email is required", ErrInvalidInput)
	}
	return err
}

func normalizePage(limit, offset int) (int, int) {
	if limit <= 0 {
		limit = 25
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

func textArg(s string) pgtype.Text {
	s = strings.TrimSpace(s)
	if s == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: s, Valid: true}
}

func textFromPtr(s *string) pgtype.Text {
	if s == nil {
		return pgtype.Text{}
	}
	v := strings.TrimSpace(*s)
	if v == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: v, Valid: true}
}

func optionalText(v pgtype.Text) *string {
	if !v.Valid {
		empty := ""
		return &empty
	}
	s := v.String
	return &s
}

func uuidString(id pgtype.UUID) string {
	if !id.Valid {
		return ""
	}
	b := id.Bytes
	hex := "0123456789abcdef"
	out := make([]byte, 36)
	pos := 0
	for i, by := range b {
		if i == 4 || i == 6 || i == 8 || i == 10 {
			out[pos] = '-'
			pos++
		}
		out[pos] = hex[by>>4]
		out[pos+1] = hex[by&0x0f]
		pos += 2
	}
	return string(out)
}
