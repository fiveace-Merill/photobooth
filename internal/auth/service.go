package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/fiveace-Merill/photobooth/internal/auth/store"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	// SessionCookieName is the browser cookie that carries the raw session token.
	// The database stores only a SHA-256 hash of this token.
	SessionCookieName = "photobooth_session"

	sessionDuration = 30 * 24 * time.Hour
	sessionTokenLen = 32 // bytes of randomness before base64 encoding
)

var (
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrEmailTaken         = errors.New("email already in use")
	ErrSlugTaken          = errors.New("slug already in use")
	ErrInvalidInput       = errors.New("invalid input")
	ErrSignupClosed       = errors.New("signups are invite-only right now")
	ErrInviteInvalid      = errors.New("invitation not found")
)

// Service contains auth business logic. It depends on sqlc-generated queries,
// not on HTTP types, so it can be tested and reused without a web server.
type Service struct {
	q          *store.Queries
	pool       *pgxpool.Pool
	signupMode string
}

// NewService constructs an auth Service.
// signupMode must be one of: closed, invite, open.
func NewService(q *store.Queries, pool *pgxpool.Pool, signupMode string) *Service {
	return &Service{q: q, pool: pool, signupMode: signupMode}
}

// SignupInput is the data required to create a photographer account.
type SignupInput struct {
	Email        string
	Password     string
	BusinessName string
	Slug         string
	InviteToken  string
}

// SessionResult is returned when a session is created (signup/login).
// RawToken must be sent to the client cookie; TokenHash is stored in DB.
type SessionResult struct {
	RawToken  string
	ExpiresAt time.Time
}

// Signup creates a photographer, hashes their password, and creates a session.
// It returns the created photographer and session token data.
func (s *Service) Signup(ctx context.Context, in SignupInput) (store.Photographer, SessionResult, error) {
	in.Email = normalizeEmail(in.Email)
	in.BusinessName = strings.TrimSpace(in.BusinessName)
	in.Slug = strings.TrimSpace(in.Slug)
	in.InviteToken = strings.TrimSpace(in.InviteToken)

	switch s.signupMode {
	case "closed":
		return store.Photographer{}, SessionResult{}, ErrSignupClosed
	case "invite":
		return s.signupWithInvite(ctx, in)
	case "open":
		if err := validateSignup(in); err != nil {
			return store.Photographer{}, SessionResult{}, err
		}
		hash, err := HashPassword(in.Password)
		if err != nil {
			return store.Photographer{}, SessionResult{}, fmt.Errorf("hash password: %w", err)
		}
		row, err := s.q.CreatePhotographer(ctx, store.CreatePhotographerParams{
			Email:        in.Email,
			PasswordHash: hash,
			BusinessName: in.BusinessName,
			Slug:         in.Slug,
		})
		if err != nil {
			return store.Photographer{}, SessionResult{}, mapCreatePhotographerErr(err)
		}
		photog := photographerFromCreateRow(row)
		sess, err := s.createSession(ctx, photog.ID)
		if err != nil {
			return store.Photographer{}, SessionResult{}, err
		}
		_ = s.q.UpdatePhotographerLastLoginAt(ctx, photog.ID)
		return photog, sess, nil
	default:
		return store.Photographer{}, SessionResult{}, fmt.Errorf("unknown signup mode")
	}
}

// Login verifies credentials and creates a new session.
// It returns ErrInvalidCredentials for unknown email or wrong password to avoid
// leaking which part failed.
func (s *Service) Login(ctx context.Context, email, password string) (store.Photographer, SessionResult, error) {
	email = normalizeEmail(email)
	if email == "" || password == "" {
		return store.Photographer{}, SessionResult{}, ErrInvalidCredentials
	}

	row, err := s.q.GetPhotographerByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return store.Photographer{}, SessionResult{}, ErrInvalidCredentials
		}
		return store.Photographer{}, SessionResult{}, fmt.Errorf("get photographer: %w", err)
	}
	photog := photographerFromGetByEmailRow(row)
	if photog.Status != "active" {
		return store.Photographer{}, SessionResult{}, ErrInvalidCredentials
	}

	ok, err := VerifyPassword(password, photog.PasswordHash)
	if err != nil {
		return store.Photographer{}, SessionResult{}, fmt.Errorf("verify password: %w", err)
	}
	if !ok {
		return store.Photographer{}, SessionResult{}, ErrInvalidCredentials
	}

	sess, err := s.createSession(ctx, photog.ID)
	if err != nil {
		return store.Photographer{}, SessionResult{}, err
	}
	_ = s.q.UpdatePhotographerLastLoginAt(ctx, photog.ID)
	return photog, sess, nil
}

// Logout deletes the session identified by the raw cookie token.
// Missing/unknown sessions are treated as success so logout is idempotent.
func (s *Service) Logout(ctx context.Context, rawToken string) error {
	if strings.TrimSpace(rawToken) == "" {
		return nil
	}
	hash := hashSessionToken(rawToken)
	if err := s.q.DeleteSession(ctx, hash[:]); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

// Authenticate resolves a raw session token to an active photographer.
// It returns pgx.ErrNoRows when the session is missing/expired/inactive.
func (s *Service) Authenticate(ctx context.Context, rawToken string) (store.Photographer, error) {
	if strings.TrimSpace(rawToken) == "" {
		return store.Photographer{}, pgx.ErrNoRows
	}
	hash := hashSessionToken(rawToken)
	row, err := s.q.GetPhotographerBySessionTokenHash(ctx, hash[:])
	if err != nil {
		return store.Photographer{}, err
	}
	return photographerFromSessionRow(row), nil
}

func (s *Service) createSession(ctx context.Context, photographerID pgtype.UUID) (SessionResult, error) {
	raw, err := newSessionToken()
	if err != nil {
		return SessionResult{}, err
	}
	hash := hashSessionToken(raw)
	expiresAt := time.Now().Add(sessionDuration)

	q := s.q
	_, err = q.CreateSession(ctx, store.CreateSessionParams{
		TokenHash:      hash[:],
		PhotographerID: photographerID,
		ExpiresAt:      pgtype.Timestamptz{Time: expiresAt, Valid: true},
	})
	if err != nil {
		return SessionResult{}, fmt.Errorf("create session: %w", err)
	}
	return SessionResult{RawToken: raw, ExpiresAt: expiresAt}, nil
}

func (s *Service) signupWithInvite(ctx context.Context, in SignupInput) (store.Photographer, SessionResult, error) {
	if err := validateSignup(in); err != nil {
		if errors.Is(err, ErrInvalidInput) && strings.Contains(err.Error(), "email") {
			// In invite mode the email comes from the invitation. Still validate other fields.
		}
		// We validate all fields except email presence/match is checked against invite below.
	}
	if err := validateSignupInviteInput(in); err != nil {
		return store.Photographer{}, SessionResult{}, err
	}
	if in.InviteToken == "" {
		return store.Photographer{}, SessionResult{}, ErrInviteInvalid
	}
	pwHash, err := HashPassword(in.Password)
	if err != nil {
		return store.Photographer{}, SessionResult{}, fmt.Errorf("hash password: %w", err)
	}
	tokenHash := hashSessionToken(in.InviteToken)

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return store.Photographer{}, SessionResult{}, fmt.Errorf("begin signup tx: %w", err)
	}
	defer tx.Rollback(ctx)
	qtx := s.q.WithTx(tx)

	inv, err := qtx.LockInvitationByTokenHash(ctx, tokenHash[:])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return store.Photographer{}, SessionResult{}, ErrInviteInvalid
		}
		return store.Photographer{}, SessionResult{}, fmt.Errorf("lock invitation: %w", err)
	}
	if !invitationUsable(inv, time.Now()) {
		return store.Photographer{}, SessionResult{}, ErrInviteInvalid
	}
	if normalizeEmail(inv.Email) != normalizeEmail(in.Email) {
		return store.Photographer{}, SessionResult{}, ErrInviteInvalid
	}

	row, err := qtx.CreatePhotographerWithPlan(ctx, store.CreatePhotographerWithPlanParams{
		Email:        normalizeEmail(inv.Email),
		PasswordHash: pwHash,
		BusinessName: in.BusinessName,
		Slug:         in.Slug,
		PlanID:       inv.PlanID,
	})
	if err != nil {
		return store.Photographer{}, SessionResult{}, mapCreatePhotographerErr(err)
	}
	photog := photographerFromCreateWithPlanRow(row)
	if _, err := qtx.ConsumeInvitation(ctx, store.ConsumeInvitationParams{ID: inv.ID, PhotographerID: photog.ID}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return store.Photographer{}, SessionResult{}, ErrInviteInvalid
		}
		return store.Photographer{}, SessionResult{}, fmt.Errorf("consume invitation: %w", err)
	}
	sess, err := createSessionWithQueries(ctx, qtx, photog.ID)
	if err != nil {
		return store.Photographer{}, SessionResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return store.Photographer{}, SessionResult{}, fmt.Errorf("commit signup: %w", err)
	}
	return photog, sess, nil
}

func createSessionWithQueries(ctx context.Context, q *store.Queries, photographerID pgtype.UUID) (SessionResult, error) {
	raw, err := newSessionToken()
	if err != nil {
		return SessionResult{}, err
	}
	hash := hashSessionToken(raw)
	expiresAt := time.Now().Add(sessionDuration)
	_, err = q.CreateSession(ctx, store.CreateSessionParams{
		TokenHash:      hash[:],
		PhotographerID: photographerID,
		ExpiresAt:      pgtype.Timestamptz{Time: expiresAt, Valid: true},
	})
	if err != nil {
		return SessionResult{}, fmt.Errorf("create session: %w", err)
	}
	return SessionResult{RawToken: raw, ExpiresAt: expiresAt}, nil
}

func validateSignupInviteInput(in SignupInput) error {
	if in.Email == "" || !strings.Contains(in.Email, "@") {
		return fmt.Errorf("%w: valid email is required", ErrInvalidInput)
	}
	if len(in.Password) < 8 {
		return fmt.Errorf("%w: password must be at least 8 characters", ErrInvalidInput)
	}
	if in.BusinessName == "" {
		return fmt.Errorf("%w: business_name is required", ErrInvalidInput)
	}
	if in.Slug == "" {
		return fmt.Errorf("%w: slug is required", ErrInvalidInput)
	}
	if !isValidSlug(in.Slug) {
		return fmt.Errorf("%w: slug must match ^[a-z0-9][a-z0-9-]{1,30}[a-z0-9]$", ErrInvalidInput)
	}
	return nil
}

func mapCreatePhotographerErr(err error) error {
	if isUniqueViolation(err) {
		if strings.Contains(err.Error(), "photographers_slug_key") {
			return ErrSlugTaken
		}
		return ErrEmailTaken
	}
	return fmt.Errorf("create photographer: %w", err)
}

func validateSignup(in SignupInput) error {
	if in.Email == "" || !strings.Contains(in.Email, "@") {
		return fmt.Errorf("%w: valid email is required", ErrInvalidInput)
	}
	if len(in.Password) < 8 {
		return fmt.Errorf("%w: password must be at least 8 characters", ErrInvalidInput)
	}
	if in.BusinessName == "" {
		return fmt.Errorf("%w: business_name is required", ErrInvalidInput)
	}
	if in.Slug == "" {
		return fmt.Errorf("%w: slug is required", ErrInvalidInput)
	}
	if !isValidSlug(in.Slug) {
		return fmt.Errorf("%w: slug must match ^[a-z0-9][a-z0-9-]{1,30}[a-z0-9]$", ErrInvalidInput)
	}
	return nil
}

func isValidSlug(slug string) bool {
	if len(slug) < 3 || len(slug) > 32 {
		return false
	}
	for i, r := range slug {
		isLetter := r >= 'a' && r <= 'z'
		isDigit := r >= '0' && r <= '9'
		isHyphen := r == '-'
		if !isLetter && !isDigit && !isHyphen {
			return false
		}
		if isHyphen && (i == 0 || i == len(slug)-1) {
			return false
		}
	}
	return true
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func newSessionToken() (string, error) {
	b := make([]byte, sessionTokenLen)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate session token: %w", err)
	}
	// RawURLEncoding avoids '+'/'/' and padding, making the token cookie-safe.
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func hashSessionToken(raw string) [32]byte {
	// SHA-256 is fine for high-entropy random tokens (unlike passwords).
	// We store only this hash in the DB so a DB leak doesn't expose live tokens.
	return sha256.Sum256([]byte(raw))
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}

type InvitationStatus string

const (
	InvitationPending InvitationStatus = "pending"
	InvitationUsed    InvitationStatus = "used"
	InvitationExpired InvitationStatus = "expired"
	InvitationRevoked InvitationStatus = "revoked"
)

type Invitation struct {
	ID             pgtype.UUID
	Email          string
	PlanID         string
	Note           string
	ExpiresAt      time.Time
	CreatedAt      time.Time
	CreatedBy      pgtype.UUID
	UsedAt         *time.Time
	PhotographerID pgtype.UUID
	RevokedAt      *time.Time
	Status         InvitationStatus
}

type CreateInvitationResult struct {
	Invitation Invitation
	RawToken   string
}

type PublicInvitation struct {
	Email     string    `json:"email"`
	ExpiresAt time.Time `json:"expiresAt"`
}

func invitationUsable(inv store.Invitation, now time.Time) bool {
	if inv.UsedAt.Valid || inv.RevokedAt.Valid {
		return false
	}
	if !inv.ExpiresAt.Valid || !inv.ExpiresAt.Time.After(now) {
		return false
	}
	return true
}

func invitationStatus(inv store.Invitation, now time.Time) InvitationStatus {
	if inv.RevokedAt.Valid {
		return InvitationRevoked
	}
	if inv.UsedAt.Valid {
		return InvitationUsed
	}
	if !inv.ExpiresAt.Valid || !inv.ExpiresAt.Time.After(now) {
		return InvitationExpired
	}
	return InvitationPending
}

func toInvitation(inv store.Invitation, now time.Time) Invitation {
	var usedAt *time.Time
	if inv.UsedAt.Valid {
		t := inv.UsedAt.Time
		usedAt = &t
	}
	var revokedAt *time.Time
	if inv.RevokedAt.Valid {
		t := inv.RevokedAt.Time
		revokedAt = &t
	}
	note := ""
	if inv.Note.Valid {
		note = inv.Note.String
	}
	return Invitation{
		ID:             inv.ID,
		Email:          inv.Email,
		PlanID:         inv.PlanID,
		Note:           note,
		ExpiresAt:      inv.ExpiresAt.Time,
		CreatedAt:      inv.CreatedAt.Time,
		CreatedBy:      inv.CreatedBy,
		UsedAt:         usedAt,
		PhotographerID: inv.PhotographerID,
		RevokedAt:      revokedAt,
		Status:         invitationStatus(inv, now),
	}
}

func (s *Service) CreateInvitation(ctx context.Context, email, planID string, ttl time.Duration, note string, createdBy pgtype.UUID) (CreateInvitationResult, error) {
	email = normalizeEmail(email)
	planID = strings.TrimSpace(planID)
	note = strings.TrimSpace(note)
	if email == "" || !strings.Contains(email, "@") {
		return CreateInvitationResult{}, fmt.Errorf("%w: valid email is required", ErrInvalidInput)
	}
	if planID == "" {
		return CreateInvitationResult{}, fmt.Errorf("%w: plan_id is required", ErrInvalidInput)
	}
	if ttl <= 0 {
		return CreateInvitationResult{}, fmt.Errorf("%w: ttl must be positive", ErrInvalidInput)
	}
	if s.pool == nil {
		return CreateInvitationResult{}, fmt.Errorf("invitation service requires db pool")
	}

	raw, err := newInviteToken()
	if err != nil {
		return CreateInvitationResult{}, err
	}
	tokenHash := hashSessionToken(raw)
	expiresAt := time.Now().Add(ttl)

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return CreateInvitationResult{}, fmt.Errorf("begin create invitation: %w", err)
	}
	defer tx.Rollback(ctx)
	qtx := s.q.WithTx(tx)

	if err := qtx.RevokeExpiredActiveInvitationsByEmail(ctx, email); err != nil {
		return CreateInvitationResult{}, fmt.Errorf("revoke expired invitations: %w", err)
	}
	active, err := qtx.LockActiveInvitationsByEmail(ctx, email)
	if err != nil {
		return CreateInvitationResult{}, fmt.Errorf("lock active invitations: %w", err)
	}
	for _, inv := range active {
		if invitationUsable(inv, time.Now()) {
			return CreateInvitationResult{}, fmt.Errorf("%w: active invitation already exists for email", ErrInvalidInput)
		}
	}

	var noteVal pgtype.Text
	if note != "" {
		noteVal = pgtype.Text{String: note, Valid: true}
	}
	var createdByVal pgtype.UUID
	if createdBy.Valid {
		createdByVal = createdBy
	}
	row, err := qtx.CreateInvitation(ctx, store.CreateInvitationParams{
		Email:     email,
		TokenHash: tokenHash[:],
		PlanID:    planID,
		Note:      noteVal,
		ExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: true},
		CreatedBy: createdByVal,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return CreateInvitationResult{}, fmt.Errorf("%w: active invitation already exists for email", ErrInvalidInput)
		}
		return CreateInvitationResult{}, fmt.Errorf("create invitation: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return CreateInvitationResult{}, fmt.Errorf("commit create invitation: %w", err)
	}
	return CreateInvitationResult{Invitation: toInvitation(row, time.Now()), RawToken: raw}, nil
}

func (s *Service) RevokeInvitation(ctx context.Context, id pgtype.UUID) (Invitation, error) {
	row, err := s.q.RevokeInvitation(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Invitation{}, ErrInviteInvalid
		}
		return Invitation{}, fmt.Errorf("revoke invitation: %w", err)
	}
	return toInvitation(row, time.Now()), nil
}

func (s *Service) GetInvitationByID(ctx context.Context, id pgtype.UUID) (Invitation, error) {
	row, err := s.q.GetInvitationByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Invitation{}, ErrInviteInvalid
		}
		return Invitation{}, fmt.Errorf("get invitation: %w", err)
	}
	return toInvitation(row, time.Now()), nil
}

func (s *Service) ListInvitations(ctx context.Context, status string) ([]Invitation, error) {
	status = strings.TrimSpace(strings.ToLower(status))
	var statusArg pgtype.Text
	switch status {
	case "":
		// all
	case "pending", "used", "expired", "revoked":
		statusArg = pgtype.Text{String: status, Valid: true}
	default:
		return nil, fmt.Errorf("%w: invalid status filter", ErrInvalidInput)
	}
	rows, err := s.q.ListInvitations(ctx, statusArg)
	if err != nil {
		return nil, fmt.Errorf("list invitations: %w", err)
	}
	now := time.Now()
	out := make([]Invitation, 0, len(rows))
	for _, row := range rows {
		out = append(out, toInvitation(row, now))
	}
	return out, nil
}

func (s *Service) ValidateInvitationToken(ctx context.Context, rawToken string) (PublicInvitation, error) {
	rawToken = strings.TrimSpace(rawToken)
	if rawToken == "" {
		return PublicInvitation{}, ErrInviteInvalid
	}
	hash := hashSessionToken(rawToken)
	inv, err := s.q.GetInvitationByTokenHash(ctx, hash[:])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return PublicInvitation{}, ErrInviteInvalid
		}
		return PublicInvitation{}, fmt.Errorf("get invitation by token: %w", err)
	}
	if !invitationUsable(inv, time.Now()) {
		return PublicInvitation{}, ErrInviteInvalid
	}
	return PublicInvitation{Email: inv.Email, ExpiresAt: inv.ExpiresAt.Time.UTC()}, nil
}

func newInviteToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate invitation token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func photographerFromCreateRow(r store.CreatePhotographerRow) store.Photographer {
	return store.Photographer{ID: r.ID, Email: r.Email, PasswordHash: r.PasswordHash, BusinessName: r.BusinessName, Slug: r.Slug, Phone: r.Phone, PlanID: r.PlanID, StorageUsedBytes: r.StorageUsedBytes, Status: r.Status, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt}
}

func photographerFromCreateWithPlanRow(r store.CreatePhotographerWithPlanRow) store.Photographer {
	return store.Photographer{ID: r.ID, Email: r.Email, PasswordHash: r.PasswordHash, BusinessName: r.BusinessName, Slug: r.Slug, Phone: r.Phone, PlanID: r.PlanID, StorageUsedBytes: r.StorageUsedBytes, Status: r.Status, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt}
}

func photographerFromGetByEmailRow(r store.GetPhotographerByEmailRow) store.Photographer {
	return store.Photographer{ID: r.ID, Email: r.Email, PasswordHash: r.PasswordHash, BusinessName: r.BusinessName, Slug: r.Slug, Phone: r.Phone, PlanID: r.PlanID, StorageUsedBytes: r.StorageUsedBytes, Status: r.Status, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt}
}

func photographerFromSessionRow(r store.GetPhotographerBySessionTokenHashRow) store.Photographer {
	return store.Photographer{ID: r.ID, Email: r.Email, PasswordHash: r.PasswordHash, BusinessName: r.BusinessName, Slug: r.Slug, Phone: r.Phone, PlanID: r.PlanID, StorageUsedBytes: r.StorageUsedBytes, Status: r.Status, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt}
}
