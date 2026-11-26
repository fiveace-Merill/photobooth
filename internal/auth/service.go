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
)

// Service contains auth business logic. It depends on sqlc-generated queries,
// not on HTTP types, so it can be tested and reused without a web server.
type Service struct {
	q             *store.Queries
	signupEnabled bool
}

// NewService constructs an auth Service.
func NewService(q *store.Queries, signupEnabled bool) *Service {
	return &Service{q: q, signupEnabled: signupEnabled}
}

// SignupInput is the data required to create a photographer account.
type SignupInput struct {
	Email        string
	Password     string
	BusinessName string
	Slug         string
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
	if !s.signupEnabled {
		return store.Photographer{}, SessionResult{}, ErrSignupClosed
	}

	in.Email = normalizeEmail(in.Email)
	in.BusinessName = strings.TrimSpace(in.BusinessName)
	in.Slug = strings.TrimSpace(in.Slug)

	if err := validateSignup(in); err != nil {
		return store.Photographer{}, SessionResult{}, err
	}

	hash, err := HashPassword(in.Password)
	if err != nil {
		return store.Photographer{}, SessionResult{}, fmt.Errorf("hash password: %w", err)
	}

	photog, err := s.q.CreatePhotographer(ctx, store.CreatePhotographerParams{
		Email:        in.Email,
		PasswordHash: hash,
		BusinessName: in.BusinessName,
		Slug:         in.Slug,
	})
	if err != nil {
		if isUniqueViolation(err) {
			if strings.Contains(err.Error(), "photographers_slug_key") {
				return store.Photographer{}, SessionResult{}, ErrSlugTaken
			}
			return store.Photographer{}, SessionResult{}, ErrEmailTaken
		}
		return store.Photographer{}, SessionResult{}, fmt.Errorf("create photographer: %w", err)
	}

	sess, err := s.createSession(ctx, photog.ID)
	if err != nil {
		return store.Photographer{}, SessionResult{}, err
	}
	return photog, sess, nil
}

// Login verifies credentials and creates a new session.
// It returns ErrInvalidCredentials for unknown email or wrong password to avoid
// leaking which part failed.
func (s *Service) Login(ctx context.Context, email, password string) (store.Photographer, SessionResult, error) {
	email = normalizeEmail(email)
	if email == "" || password == "" {
		return store.Photographer{}, SessionResult{}, ErrInvalidCredentials
	}

	photog, err := s.q.GetPhotographerByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return store.Photographer{}, SessionResult{}, ErrInvalidCredentials
		}
		return store.Photographer{}, SessionResult{}, fmt.Errorf("get photographer: %w", err)
	}
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
	p, err := s.q.GetPhotographerBySessionTokenHash(ctx, hash[:])
	if err != nil {
		return store.Photographer{}, err
	}
	return p, nil
}

func (s *Service) createSession(ctx context.Context, photographerID pgtype.UUID) (SessionResult, error) {
	raw, err := newSessionToken()
	if err != nil {
		return SessionResult{}, err
	}
	hash := hashSessionToken(raw)
	expiresAt := time.Now().Add(sessionDuration)

	_, err = s.q.CreateSession(ctx, store.CreateSessionParams{
		TokenHash:      hash[:],
		PhotographerID: photographerID,
		ExpiresAt:      pgtype.Timestamptz{Time: expiresAt, Valid: true},
	})
	if err != nil {
		return SessionResult{}, fmt.Errorf("create session: %w", err)
	}
	return SessionResult{RawToken: raw, ExpiresAt: expiresAt}, nil
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
