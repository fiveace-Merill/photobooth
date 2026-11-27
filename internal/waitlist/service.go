package waitlist

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/fiveace-Merill/photobooth/internal/platform/phone"
	"github.com/fiveace-Merill/photobooth/internal/waitlist/store"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const ConsentVersion = "2026-05-28-v1"

var ErrNotFound = errors.New("waitlist entry not found")
var ErrInvalidInput = errors.New("invalid input")
var ErrContactRequired = errors.New("contact required")

type Service struct {
	q    *store.Queries
	pool interface {
		Begin(context.Context) (pgx.Tx, error)
	}
}

func NewService(q *store.Queries, pool interface {
	Begin(context.Context) (pgx.Tx, error)
}) *Service {
	return &Service{q: q, pool: pool}
}

type SubmitInput struct {
	Name            string
	Contact         string
	PhotographyType string
	ShootsPerMonth  string
	Consent         bool
	Website         string // honeypot
}

type SubmitResult struct {
	Accepted bool // always true for non-validation cases handled by handler
	Silent   bool
}

func normalizeEmail(v string) string { return strings.ToLower(strings.TrimSpace(v)) }

func (s *Service) Submit(ctx context.Context, in SubmitInput) (SubmitResult, error) {
	if strings.TrimSpace(in.Website) != "" {
		return SubmitResult{Accepted: true, Silent: true}, nil
	}
	in.Name = strings.TrimSpace(in.Name)
	in.Contact = strings.TrimSpace(in.Contact)
	in.PhotographyType = strings.TrimSpace(in.PhotographyType)
	in.ShootsPerMonth = strings.TrimSpace(in.ShootsPerMonth)
	if in.Name == "" || in.Contact == "" || in.PhotographyType == "" || in.ShootsPerMonth == "" {
		return SubmitResult{}, fmt.Errorf("%w: missing required fields", ErrInvalidInput)
	}
	if len(in.Name) > 120 || len(in.Contact) > 254 || len(in.PhotographyType) > 120 || len(in.ShootsPerMonth) > 120 {
		return SubmitResult{}, fmt.Errorf("%w: field too long", ErrInvalidInput)
	}
	if !in.Consent {
		return SubmitResult{}, fmt.Errorf("%w: consent required", ErrInvalidInput)
	}

	var emailVal, phoneVal pgtype.Text
	if strings.Contains(in.Contact, "@") {
		e := normalizeEmail(in.Contact)
		if !looksLikeEmail(e) {
			return SubmitResult{}, fmt.Errorf("%w: invalid email", ErrInvalidInput)
		}
		emailVal = pgtype.Text{String: e, Valid: true}
		if _, err := s.q.GetWaitlistByEmail(ctx, emailVal); err == nil {
			return SubmitResult{Accepted: true}, nil
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return SubmitResult{}, fmt.Errorf("lookup waitlist email: %w", err)
		}
	} else {
		norm, err := phone.Normalize(in.Contact)
		if err != nil {
			return SubmitResult{}, fmt.Errorf("%w: invalid contact", ErrInvalidInput)
		}
		phoneVal = pgtype.Text{String: norm, Valid: true}
		if _, err := s.q.GetWaitlistByPhone(ctx, phoneVal); err == nil {
			return SubmitResult{Accepted: true}, nil
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return SubmitResult{}, fmt.Errorf("lookup waitlist phone: %w", err)
		}
	}

	_, err := s.q.CreateWaitlistEntry(ctx, store.CreateWaitlistEntryParams{
		Name:            in.Name,
		ContactEmail:    emailVal,
		ContactPhone:    phoneVal,
		PhotographyType: in.PhotographyType,
		ShootsPerMonth:  in.ShootsPerMonth,
		ConsentAt:       pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
		ConsentVersion:  ConsentVersion,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return SubmitResult{Accepted: true}, nil
		}
		return SubmitResult{}, fmt.Errorf("create waitlist entry: %w", err)
	}
	return SubmitResult{Accepted: true}, nil
}

func looksLikeEmail(v string) bool {
	if len(v) > 254 || strings.Count(v, "@") != 1 {
		return false
	}
	parts := strings.Split(v, "@")
	if parts[0] == "" || parts[1] == "" || !strings.Contains(parts[1], ".") {
		return false
	}
	return true
}

func isUniqueViolation(err error) bool {
	type pgErr interface{ SQLState() string }
	var pe pgErr
	if errors.As(err, &pe) {
		return pe.SQLState() == "23505"
	}
	return false
}

type Entry struct {
	ID              pgtype.UUID
	Name            string
	ContactEmail    string
	ContactPhone    string
	PhotographyType string
	ShootsPerMonth  string
	ConsentAt       time.Time
	ConsentVersion  string
	Status          string
	InvitationID    pgtype.UUID
	InvitedAt       *time.Time
	Notes           string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func toEntry(r store.WaitlistEntry) Entry {
	var email, p, notes string
	if r.ContactEmail.Valid {
		email = r.ContactEmail.String
	}
	if r.ContactPhone.Valid {
		p = r.ContactPhone.String
	}
	if r.Notes.Valid {
		notes = r.Notes.String
	}
	var invitedAt *time.Time
	if r.InvitedAt.Valid {
		t := r.InvitedAt.Time.UTC()
		invitedAt = &t
	}
	return Entry{ID: r.ID, Name: r.Name, ContactEmail: email, ContactPhone: p, PhotographyType: r.PhotographyType, ShootsPerMonth: r.ShootsPerMonth, ConsentAt: r.ConsentAt.Time.UTC(), ConsentVersion: r.ConsentVersion, Status: r.Status, InvitationID: r.InvitationID, InvitedAt: invitedAt, Notes: notes, CreatedAt: r.CreatedAt.Time.UTC(), UpdatedAt: r.UpdatedAt.Time.UTC()}
}

func (s *Service) GetByID(ctx context.Context, id pgtype.UUID) (Entry, error) {
	r, err := s.q.GetWaitlistByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Entry{}, ErrNotFound
		}
		return Entry{}, fmt.Errorf("get waitlist: %w", err)
	}
	return toEntry(r), nil
}

type ListResult struct {
	Items  []Entry
	Total  int64
	Limit  int32
	Offset int32
}

func (s *Service) List(ctx context.Context, status, q string, limit, offset int32) (ListResult, error) {
	if limit <= 0 {
		limit = 25
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	status = strings.TrimSpace(strings.ToLower(status))
	q = strings.TrimSpace(q)
	total, err := s.q.CountWaitlist(ctx, store.CountWaitlistParams{Column1: status, Column2: q})
	if err != nil {
		return ListResult{}, fmt.Errorf("count waitlist: %w", err)
	}
	rows, err := s.q.ListWaitlist(ctx, store.ListWaitlistParams{Column1: status, Column2: q, Limit: limit, Offset: offset})
	if err != nil {
		return ListResult{}, fmt.Errorf("list waitlist: %w", err)
	}
	items := make([]Entry, 0, len(rows))
	for _, r := range rows {
		items = append(items, toEntry(r))
	}
	return ListResult{Items: items, Total: total, Limit: limit, Offset: offset}, nil
}

func (s *Service) CountNew(ctx context.Context) (int64, error) {
	n, err := s.q.CountWaitlistNew(ctx)
	if err != nil {
		return 0, fmt.Errorf("count waitlist new: %w", err)
	}
	return n, nil
}

type PatchInput struct {
	Notes *string
	Email *string
	Phone *string
}

type AuditFn func(context.Context, pgx.Tx) error

func (s *Service) Patch(ctx context.Context, id pgtype.UUID, in PatchInput, audit AuditFn) (Entry, error) {
	if s.pool == nil {
		return Entry{}, fmt.Errorf("waitlist service misconfigured: pool required")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Entry{}, fmt.Errorf("begin patch waitlist: %w", err)
	}
	defer tx.Rollback(ctx)
	qtx := s.q.WithTx(tx)
	cur, err := qtx.GetWaitlistByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Entry{}, ErrNotFound
		}
		return Entry{}, err
	}
	notes := cur.Notes
	emailVal := cur.ContactEmail
	phoneVal := cur.ContactPhone
	if in.Notes != nil {
		n := strings.TrimSpace(*in.Notes)
		if len(n) > 2000 {
			return Entry{}, fmt.Errorf("%w: notes too long", ErrInvalidInput)
		}
		if n == "" {
			notes = pgtype.Text{}
		} else {
			notes = pgtype.Text{String: n, Valid: true}
		}
	}
	if in.Email != nil {
		v := normalizeEmail(*in.Email)
		if v == "" {
			emailVal = pgtype.Text{}
		} else {
			if !looksLikeEmail(v) {
				return Entry{}, fmt.Errorf("%w: invalid email", ErrInvalidInput)
			}
			emailVal = pgtype.Text{String: v, Valid: true}
		}
	}
	if in.Phone != nil {
		v := strings.TrimSpace(*in.Phone)
		if v == "" {
			phoneVal = pgtype.Text{}
		} else {
			norm, err := phone.Normalize(v)
			if err != nil {
				return Entry{}, fmt.Errorf("%w: invalid phone", ErrInvalidInput)
			}
			phoneVal = pgtype.Text{String: norm, Valid: true}
		}
	}
	if !emailVal.Valid && !phoneVal.Valid {
		return Entry{}, fmt.Errorf("%w: at least one contact required", ErrInvalidInput)
	}
	row, err := qtx.UpdateWaitlistNotesContact(ctx, store.UpdateWaitlistNotesContactParams{ID: id, Notes: notes, ContactEmail: emailVal, ContactPhone: phoneVal})
	if err != nil {
		if isUniqueViolation(err) {
			return Entry{}, fmt.Errorf("%w: contact already exists", ErrInvalidInput)
		}
		return Entry{}, fmt.Errorf("update waitlist: %w", err)
	}
	if audit != nil {
		if err := audit(ctx, tx); err != nil {
			return Entry{}, fmt.Errorf("audit waitlist patch: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return Entry{}, fmt.Errorf("commit patch waitlist: %w", err)
	}
	return toEntry(row), nil
}

func (s *Service) SetStatus(ctx context.Context, id pgtype.UUID, status string, audit AuditFn) (Entry, error) {
	if s.pool == nil {
		return Entry{}, fmt.Errorf("waitlist service misconfigured: pool required")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Entry{}, fmt.Errorf("begin set waitlist status: %w", err)
	}
	defer tx.Rollback(ctx)
	qtx := s.q.WithTx(tx)
	row, err := qtx.SetWaitlistStatus(ctx, store.SetWaitlistStatusParams{ID: id, Status: status, InvitationID: pgtype.UUID{}})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Entry{}, ErrNotFound
		}
		return Entry{}, fmt.Errorf("set waitlist status: %w", err)
	}
	if audit != nil {
		if err := audit(ctx, tx); err != nil {
			return Entry{}, fmt.Errorf("audit waitlist status: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return Entry{}, fmt.Errorf("commit set waitlist status: %w", err)
	}
	return toEntry(row), nil
}

func (s *Service) MarkInvited(ctx context.Context, id pgtype.UUID, invitationID pgtype.UUID, email string) (Entry, error) {
	return s.MarkInvitedInTx(ctx, nil, id, invitationID, email)
}

func (s *Service) MarkInvitedInTx(ctx context.Context, tx pgx.Tx, id pgtype.UUID, invitationID pgtype.UUID, email string) (Entry, error) {
	q := s.q
	if tx != nil {
		q = q.WithTx(tx)
	}
	var emailArg pgtype.Text
	email = normalizeEmail(email)
	if email != "" {
		emailArg = pgtype.Text{String: email, Valid: true}
	}
	row, err := q.MarkWaitlistInvited(ctx, store.MarkWaitlistInvitedParams{ID: id, InvitationID: invitationID, ContactEmail: emailArg})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Entry{}, ErrNotFound
		}
		if isUniqueViolation(err) {
			return Entry{}, fmt.Errorf("%w: contact already exists", ErrInvalidInput)
		}
		return Entry{}, fmt.Errorf("mark waitlist invited: %w", err)
	}
	return toEntry(row), nil
}

func (s *Service) Delete(ctx context.Context, id pgtype.UUID, audit AuditFn) error {
	if s.pool == nil {
		return fmt.Errorf("waitlist service misconfigured: pool required")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin delete waitlist: %w", err)
	}
	defer tx.Rollback(ctx)
	qtx := s.q.WithTx(tx)
	n, err := qtx.DeleteWaitlist(ctx, id)
	if err != nil {
		return fmt.Errorf("delete waitlist: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	if audit != nil {
		if err := audit(ctx, tx); err != nil {
			return fmt.Errorf("audit waitlist delete: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit delete waitlist: %w", err)
	}
	return nil
}

func (s *Service) ListExport(ctx context.Context, audit AuditFn) ([]Entry, error) {
	if s.pool == nil {
		return nil, fmt.Errorf("waitlist service misconfigured: pool required")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin export waitlist: %w", err)
	}
	defer tx.Rollback(ctx)
	qtx := s.q.WithTx(tx)
	rows, err := qtx.ListWaitlistExport(ctx)
	if err != nil {
		return nil, fmt.Errorf("list waitlist export: %w", err)
	}
	if audit != nil {
		if err := audit(ctx, tx); err != nil {
			return nil, fmt.Errorf("audit waitlist export: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit export waitlist: %w", err)
	}
	items := make([]Entry, 0, len(rows))
	for _, r := range rows {
		items = append(items, toEntry(r))
	}
	return items, nil
}
