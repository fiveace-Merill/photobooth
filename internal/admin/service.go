package admin

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/fiveace-Merill/photobooth/internal/admin/store"
	"github.com/fiveace-Merill/photobooth/internal/auth"
	"github.com/fiveace-Merill/photobooth/internal/notify"
	waitlistpkg "github.com/fiveace-Merill/photobooth/internal/waitlist"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUnauthorized       = errors.New("unauthorized")
	ErrForbidden          = errors.New("forbidden")
	ErrNotFound           = errors.New("not found")
	ErrInvalidInput       = errors.New("invalid input")
	ErrRateLimited        = errors.New("rate limited")
	ErrPending            = errors.New("pending 2fa")
	ErrNoTOTP             = errors.New("totp not configured")
)

type Service struct {
	pool        *pgxpool.Pool
	q           *store.Queries
	now         func() time.Time
	totpKey     []byte
	require2FA  bool
	sessionIdle time.Duration
	sessionMax  time.Duration
	dummyHash   string
}

func NewService(pool *pgxpool.Pool, q *store.Queries, totpKey []byte, require2FA bool, idleMin, maxHours int) *Service {
	dummy, _ := auth.HashPassword("dummy-password-for-timing-only")
	if idleMin <= 0 {
		idleMin = 60
	}
	if maxHours <= 0 {
		maxHours = 12
	}
	return &Service{pool: pool, q: q, now: time.Now, totpKey: append([]byte(nil), totpKey...), require2FA: require2FA, sessionIdle: time.Duration(idleMin) * time.Minute, sessionMax: time.Duration(maxHours) * time.Hour, dummyHash: dummy}
}

func (s *Service) WithNow(now func() time.Time) {
	if now != nil {
		s.now = now
	}
}

type LoginResult struct {
	Next     string // totp, totp_setup, ok
	RawToken string
	Expires  time.Time
	Admin    store.Admin
}

func (s *Service) Login(ctx context.Context, email, password, ip string) (LoginResult, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || password == "" {
		_, _ = auth.VerifyPassword(password, s.dummyHash)
		return LoginResult{}, ErrInvalidCredentials
	}
	a, err := s.q.GetAdminByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			_, _ = auth.VerifyPassword(password, s.dummyHash)
			return LoginResult{}, ErrInvalidCredentials
		}
		return LoginResult{}, err
	}
	if a.Status != "active" {
		_, _ = auth.VerifyPassword(password, s.dummyHash)
		return LoginResult{}, ErrInvalidCredentials
	}
	ok, err := auth.VerifyPassword(password, a.PasswordHash)
	if err != nil || !ok {
		return LoginResult{}, ErrInvalidCredentials
	}

	needs2FA := s.require2FA
	confirmed := a.TotpConfirmedAt.Valid && len(a.TotpSecretEnc) > 0
	pending := needs2FA
	next := "ok"
	if needs2FA {
		if confirmed {
			next = "totp"
		} else {
			next = "totp_setup"
		}
	}
	ttl := s.sessionMax
	if pending {
		ttl = 5 * time.Minute
	}
	raw, err := newToken(32)
	if err != nil {
		return LoginResult{}, err
	}
	hash := hashToken(raw)
	expires := s.now().Add(ttl)
	_, err = s.q.CreateAdminSession(ctx, store.CreateAdminSessionParams{TokenHash: hash, AdminID: a.ID, Pending: pending, ExpiresAt: pgtype.Timestamptz{Time: expires, Valid: true}})
	if err != nil {
		return LoginResult{}, err
	}
	_ = s.q.UpdateAdminLastLoginAt(ctx, a.ID)
	return LoginResult{Next: next, RawToken: raw, Expires: expires, Admin: a}, nil
}

func (s *Service) Authenticate(ctx context.Context, rawToken string) (store.Admin, []byte, bool, error) {
	if strings.TrimSpace(rawToken) == "" {
		return store.Admin{}, nil, false, ErrUnauthorized
	}
	hash := hashToken(rawToken)
	row, err := s.q.GetAdminSessionByHash(ctx, hash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return store.Admin{}, nil, false, ErrUnauthorized
		}
		return store.Admin{}, nil, false, err
	}
	now := s.now()
	if !row.ExpiresAt.Valid || !row.ExpiresAt.Time.After(now) {
		_ = s.q.DeleteAdminSession(ctx, hash)
		return store.Admin{}, nil, false, ErrUnauthorized
	}
	if !row.CreatedAt.Valid || row.CreatedAt.Time.Add(s.sessionMax).Before(now) {
		_ = s.q.DeleteAdminSession(ctx, hash)
		return store.Admin{}, nil, false, ErrUnauthorized
	}
	if !row.LastSeenAt.Valid || row.LastSeenAt.Time.Add(s.sessionIdle).Before(now) {
		_ = s.q.DeleteAdminSession(ctx, hash)
		return store.Admin{}, nil, false, ErrUnauthorized
	}
	if row.Status != "active" {
		_ = s.q.DeleteAdminSession(ctx, hash)
		return store.Admin{}, nil, false, ErrUnauthorized
	}
	_ = s.q.TouchAdminSession(ctx, hash)
	a := store.Admin{ID: row.ID, Email: row.Email, PasswordHash: row.PasswordHash, TotpSecretEnc: row.TotpSecretEnc, TotpConfirmedAt: row.TotpConfirmedAt, Status: row.Status, LastLoginAt: row.LastLoginAt, CreatedAt: row.AdminCreatedAt}
	return a, hash, row.Pending, nil
}

func (s *Service) Logout(ctx context.Context, rawToken string) error {
	if strings.TrimSpace(rawToken) == "" {
		return nil
	}
	hash := hashToken(rawToken)
	return s.q.DeleteAdminSession(ctx, hash)
}

func (s *Service) PromoteSession(ctx context.Context, hash []byte) error {
	expires := s.now().Add(s.sessionMax)
	return s.q.PromoteAdminSession(ctx, store.PromoteAdminSessionParams{TokenHash: hash, ExpiresAt: pgtype.Timestamptz{Time: expires, Valid: true}})
}

func (s *Service) StartTOTPSetup(ctx context.Context, admin store.Admin) (secret, otpauth string, err error) {
	if admin.TotpConfirmedAt.Valid {
		return "", "", fmt.Errorf("%w: totp already confirmed", ErrInvalidInput)
	}
	secret, otpauth, err = generateTOTPSecret(admin.Email)
	if err != nil {
		return "", "", err
	}
	enc, err := encryptSecret(s.totpKey, []byte(secret))
	if err != nil {
		return "", "", err
	}
	if err := s.q.SetAdminTOTPSecretEnc(ctx, store.SetAdminTOTPSecretEncParams{ID: admin.ID, TotpSecretEnc: enc}); err != nil {
		return "", "", err
	}
	return secret, otpauth, nil
}

func (s *Service) ConfirmTOTPSetup(ctx context.Context, admin store.Admin, code string, audit AuditEntry) ([]string, error) {
	if len(admin.TotpSecretEnc) == 0 {
		return nil, ErrNoTOTP
	}
	secretBytes, err := decryptSecret(s.totpKey, admin.TotpSecretEnc)
	if err != nil {
		return nil, err
	}
	if !validateTOTPCode(string(secretBytes), code) {
		return nil, ErrInvalidCredentials
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	qtx := s.q.WithTx(tx)
	if err := qtx.ConfirmAdminTOTP(ctx, admin.ID); err != nil {
		return nil, err
	}
	if err := qtx.DeleteRecoveryCodesByAdmin(ctx, admin.ID); err != nil {
		return nil, err
	}
	codes, err := generateRecoveryCodes(10)
	if err != nil {
		return nil, err
	}
	for _, c := range codes {
		h := hashToken(normalizeRecoveryCode(c))
		if err := qtx.InsertRecoveryCode(ctx, store.InsertRecoveryCodeParams{AdminID: admin.ID, CodeHash: h}); err != nil {
			return nil, err
		}
	}
	if audit.Action != "" {
		if err := audit.Write(ctx, qtx); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return codes, nil
}

func (s *Service) VerifyTOTPOrRecovery(ctx context.Context, admin store.Admin, code, recovery string) error {
	if strings.TrimSpace(recovery) != "" {
		h := hashToken(normalizeRecoveryCode(recovery))
		n, err := s.q.ConsumeRecoveryCode(ctx, store.ConsumeRecoveryCodeParams{AdminID: admin.ID, CodeHash: h})
		if err != nil {
			return err
		}
		if n == 0 {
			return ErrInvalidCredentials
		}
		return nil
	}
	if len(admin.TotpSecretEnc) == 0 || !admin.TotpConfirmedAt.Valid {
		return ErrNoTOTP
	}
	secretBytes, err := decryptSecret(s.totpKey, admin.TotpSecretEnc)
	if err != nil {
		return err
	}
	if !validateTOTPCode(string(secretBytes), code) {
		return ErrInvalidCredentials
	}
	return nil
}

func (s *Service) ChangePassword(ctx context.Context, admin store.Admin, current, newPassword string, currentHash []byte, ip string) error {
	ok, err := auth.VerifyPassword(current, admin.PasswordHash)
	if err != nil || !ok {
		return ErrInvalidCredentials
	}
	if len(newPassword) < 12 {
		return fmt.Errorf("%w: password must be at least 12 characters", ErrInvalidInput)
	}
	hash, err := auth.HashPassword(newPassword)
	if err != nil {
		return err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	qtx := s.q.WithTx(tx)
	if err := qtx.UpdateAdminPasswordHash(ctx, store.UpdateAdminPasswordHashParams{ID: admin.ID, PasswordHash: hash}); err != nil {
		return err
	}
	if len(currentHash) > 0 {
		if err := qtx.DeleteOtherAdminSessions(ctx, store.DeleteOtherAdminSessionsParams{AdminID: admin.ID, TokenHash: currentHash}); err != nil {
			return err
		}
	} else {
		if err := qtx.DeleteAllAdminSessions(ctx, admin.ID); err != nil {
			return err
		}
	}
	if err := writeAudit(ctx, qtx, admin.ID, "admin", "admin.password.changed", "admin", uuidString(admin.ID), map[string]any{}, ip); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func randomBytes(n int) ([]byte, error) { b := make([]byte, n); _, err := rand.Read(b); return b, err }

func (s *Service) WriteAudit(ctx context.Context, adminID pgtype.UUID, actor, action, targetType, targetID string, meta map[string]any, ip string) error {
	return writeAudit(ctx, s.q, adminID, actor, action, targetType, targetID, meta, ip)
}

func (s *Service) Overview(ctx context.Context) (store.AdminPlatformStatsRow, int64, int64, error) {
	stats, err := s.q.AdminPlatformStats(ctx)
	if err != nil {
		return store.AdminPlatformStatsRow{}, 0, 0, err
	}
	// waitlist new and pending invitations counts are fetched by caller via services? easier here via pool queries; but keep simple.
	var pendingInv int64
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*)::bigint FROM invitations WHERE used_at IS NULL AND revoked_at IS NULL AND expires_at > now()`).Scan(&pendingInv); err != nil {
		return store.AdminPlatformStatsRow{}, 0, 0, err
	}
	var waitlistNew int64
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*)::bigint FROM waitlist_entries WHERE status='new'`).Scan(&waitlistNew); err != nil {
		return store.AdminPlatformStatsRow{}, 0, 0, err
	}
	return stats, waitlistNew, pendingInv, nil
}

func (s *Service) AuditList(ctx context.Context, action string, limit, offset int32) ([]store.AdminAuditLog, int64, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}
	total, err := s.q.CountAuditLog(ctx, action)
	if err != nil {
		return nil, 0, err
	}
	rows, err := s.q.ListAuditLog(ctx, store.ListAuditLogParams{Column1: action, Limit: limit, Offset: offset})
	if err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

type Invitation struct {
	ID        pgtype.UUID
	Email     string
	PlanID    string
	Note      string
	ExpiresAt time.Time
	CreatedAt time.Time
	CreatedBy pgtype.UUID
	UsedAt    *time.Time
	RevokedAt *time.Time
	Status    string
}

func (s *Service) invitationStatus(expiresAt time.Time, usedAt, revokedAt pgtype.Timestamptz) string {
	if revokedAt.Valid {
		return "revoked"
	}
	if usedAt.Valid {
		return "used"
	}
	if !expiresAt.After(s.now()) {
		return "expired"
	}
	return "pending"
}

func (s *Service) toInvitation(r store.Invitation) Invitation {
	var used, revoked *time.Time
	if r.UsedAt.Valid {
		t := r.UsedAt.Time.UTC()
		used = &t
	}
	if r.RevokedAt.Valid {
		t := r.RevokedAt.Time.UTC()
		revoked = &t
	}
	note := ""
	if r.Note.Valid {
		note = r.Note.String
	}
	return Invitation{ID: r.ID, Email: r.Email, PlanID: r.PlanID, Note: note, ExpiresAt: r.ExpiresAt.Time.UTC(), CreatedAt: r.CreatedAt.Time.UTC(), CreatedBy: r.CreatedBy, UsedAt: used, RevokedAt: revoked, Status: s.invitationStatus(r.ExpiresAt.Time, r.UsedAt, r.RevokedAt)}
}

func (s *Service) CreateInvitation(ctx context.Context, email, planID string, ttl time.Duration, note string, createdBy pgtype.UUID, audit AuditEntry) (string, Invitation, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	planID = strings.TrimSpace(planID)
	note = strings.TrimSpace(note)
	if email == "" || !strings.Contains(email, "@") || planID == "" || ttl <= 0 {
		return "", Invitation{}, fmt.Errorf("%w: invalid invitation", ErrInvalidInput)
	}
	raw, err := newToken(32)
	if err != nil {
		return "", Invitation{}, err
	}
	hash := hashToken(raw)
	expiresAt := s.now().Add(ttl)
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", Invitation{}, err
	}
	defer tx.Rollback(ctx)
	qtx := s.q.WithTx(tx)
	if err := qtx.RevokeExpiredActiveInvitationsByEmail(ctx, email); err != nil {
		return "", Invitation{}, err
	}
	active, err := qtx.LockActiveInvitationsByEmail(ctx, email)
	if err != nil {
		return "", Invitation{}, err
	}
	for _, inv := range active {
		if inv.RevokedAt.Valid || inv.UsedAt.Valid || !inv.ExpiresAt.Time.After(s.now()) {
			continue
		}
		return "", Invitation{}, fmt.Errorf("%w: active invitation exists", ErrInvalidInput)
	}
	var noteVal pgtype.Text
	if note != "" {
		noteVal = pgtype.Text{String: note, Valid: true}
	}
	r, err := qtx.CreateInvitation(ctx, store.CreateInvitationParams{Email: email, TokenHash: hash, PlanID: planID, Note: noteVal, ExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: true}, CreatedBy: createdBy})
	if err != nil {
		return "", Invitation{}, err
	}
	if audit.Action != "" {
		audit.TargetID = uuidString(r.ID)
		if audit.Meta == nil {
			audit.Meta = map[string]any{}
		}
		if err := audit.Write(ctx, qtx); err != nil {
			return "", Invitation{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return "", Invitation{}, err
	}
	return raw, s.toInvitation(r), nil
}

func (s *Service) ListInvitations(ctx context.Context, status string) ([]Invitation, error) {
	rows, err := s.q.ListInvitations(ctx, pgtype.Text{String: strings.TrimSpace(status), Valid: strings.TrimSpace(status) != ""})
	if err != nil {
		return nil, err
	}
	out := make([]Invitation, 0, len(rows))
	for _, r := range rows {
		out = append(out, s.toInvitation(r))
	}
	return out, nil
}

func (s *Service) RevokeInvitation(ctx context.Context, id pgtype.UUID, audit AuditEntry) (Invitation, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Invitation{}, err
	}
	defer tx.Rollback(ctx)
	qtx := s.q.WithTx(tx)
	r, err := qtx.RevokeInvitation(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Invitation{}, ErrNotFound
		}
		return Invitation{}, err
	}
	if audit.Action != "" {
		audit.TargetID = uuidString(r.ID)
		if audit.Meta == nil {
			audit.Meta = map[string]any{}
		}
		if err := audit.Write(ctx, qtx); err != nil {
			return Invitation{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return Invitation{}, err
	}
	return s.toInvitation(r), nil
}

func (s *Service) ResendInvitation(ctx context.Context, oldID pgtype.UUID, ttl time.Duration, createdBy pgtype.UUID, audit AuditEntry) (string, Invitation, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", Invitation{}, err
	}
	defer tx.Rollback(ctx)
	qtx := s.q.WithTx(tx)
	old, err := qtx.GetInvitationByID(ctx, oldID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", Invitation{}, ErrNotFound
		}
		return "", Invitation{}, err
	}
	_, _ = qtx.RevokeInvitation(ctx, oldID)
	email := strings.ToLower(strings.TrimSpace(old.Email))
	note := ""
	if old.Note.Valid {
		note = old.Note.String
	}
	raw, err := newToken(32)
	if err != nil {
		return "", Invitation{}, err
	}
	hash := hashToken(raw)
	expiresAt := s.now().Add(ttl)
	if err := qtx.RevokeExpiredActiveInvitationsByEmail(ctx, email); err != nil {
		return "", Invitation{}, err
	}
	_, err = qtx.LockActiveInvitationsByEmail(ctx, email)
	if err != nil {
		return "", Invitation{}, err
	}
	var noteVal pgtype.Text
	if note != "" {
		noteVal = pgtype.Text{String: note, Valid: true}
	}
	r, err := qtx.CreateInvitation(ctx, store.CreateInvitationParams{Email: email, TokenHash: hash, PlanID: old.PlanID, Note: noteVal, ExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: true}, CreatedBy: createdBy})
	if err != nil {
		return "", Invitation{}, err
	}
	if audit.Action != "" {
		audit.TargetID = uuidString(r.ID)
		if audit.Meta == nil {
			audit.Meta = map[string]any{}
		}
		if err := audit.Write(ctx, qtx); err != nil {
			return "", Invitation{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return "", Invitation{}, err
	}
	return raw, s.toInvitation(r), nil
}

type PhotographerView struct {
	ID                string     `json:"id"`
	Email             string     `json:"email"`
	BusinessName      string     `json:"businessName"`
	Slug              string     `json:"slug"`
	PlanID            string     `json:"planId"`
	Status            string     `json:"status"`
	StorageUsedBytes  int64      `json:"storageUsedBytes"`
	StorageQuotaBytes int64      `json:"storageQuotaBytes"`
	CreatedAt         time.Time  `json:"createdAt"`
	LastLoginAt       *time.Time `json:"lastLoginAt,omitempty"`
	SuspendedAt       *time.Time `json:"suspendedAt,omitempty"`
	SuspendedReason   string     `json:"suspendedReason,omitempty"`
	Shoots            int64      `json:"shoots"`
	Photos            int64      `json:"photos"`
	Delivered         int64      `json:"delivered"`
}

func (s *Service) ListPhotographers(ctx context.Context, q, status string, limit, offset int32) ([]PhotographerView, int64, error) {
	if limit <= 0 {
		limit = 25
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	total, err := s.q.CountPhotographers(ctx, store.CountPhotographersParams{Column1: strings.TrimSpace(q), Column2: strings.TrimSpace(status)})
	if err != nil {
		return nil, 0, err
	}
	rows, err := s.q.ListPhotographers(ctx, store.ListPhotographersParams{Column1: strings.TrimSpace(q), Column2: strings.TrimSpace(status), Limit: limit, Offset: offset})
	if err != nil {
		return nil, 0, err
	}
	ids := make([]pgtype.UUID, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.ID)
	}
	type usageRow struct{ Shoots, Photos, Delivered int64 }
	usage := map[string]usageRow{}
	if len(ids) > 0 {
		urows, err := s.q.AdminPhotographerUsage(ctx, ids)
		if err != nil {
			return nil, 0, err
		}
		for _, u := range urows {
			idv, _ := u.PhotographerID.(pgtype.UUID)
			shoots, _ := u.Shoots.(int64)
			photos, _ := u.Photos.(int64)
			delivered, _ := u.Delivered.(int64)
			usage[uuidString(idv)] = usageRow{Shoots: shoots, Photos: photos, Delivered: delivered}
		}
	}
	out := make([]PhotographerView, 0, len(rows))
	for _, r := range rows {
		u := usage[uuidString(r.ID)]
		var last, susp *time.Time
		if r.LastLoginAt.Valid {
			t := r.LastLoginAt.Time.UTC()
			last = &t
		}
		if r.SuspendedAt.Valid {
			t := r.SuspendedAt.Time.UTC()
			susp = &t
		}
		reason := ""
		if r.SuspendedReason.Valid {
			reason = r.SuspendedReason.String
		}
		out = append(out, PhotographerView{ID: uuidString(r.ID), Email: r.Email, BusinessName: r.BusinessName, Slug: r.Slug, PlanID: r.PlanID, Status: r.Status, StorageUsedBytes: r.StorageUsedBytes, StorageQuotaBytes: r.StorageQuotaBytes, CreatedAt: r.CreatedAt.Time.UTC(), LastLoginAt: last, SuspendedAt: susp, SuspendedReason: reason, Shoots: u.Shoots, Photos: u.Photos, Delivered: u.Delivered})
	}
	return out, total, nil
}

func (s *Service) GetPhotographer(ctx context.Context, id pgtype.UUID) (PhotographerView, error) {
	r, err := s.q.GetPhotographerForAdmin(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return PhotographerView{}, ErrNotFound
		}
		return PhotographerView{}, err
	}
	urows, err := s.q.AdminPhotographerUsage(ctx, []pgtype.UUID{id})
	if err != nil {
		return PhotographerView{}, err
	}
	type usageRow struct{ Shoots, Photos, Delivered int64 }
	var u usageRow
	if len(urows) > 0 {
		shoots, _ := urows[0].Shoots.(int64)
		photos, _ := urows[0].Photos.(int64)
		delivered, _ := urows[0].Delivered.(int64)
		u = usageRow{Shoots: shoots, Photos: photos, Delivered: delivered}
	}
	var last, susp *time.Time
	if r.LastLoginAt.Valid {
		t := r.LastLoginAt.Time.UTC()
		last = &t
	}
	if r.SuspendedAt.Valid {
		t := r.SuspendedAt.Time.UTC()
		susp = &t
	}
	reason := ""
	if r.SuspendedReason.Valid {
		reason = r.SuspendedReason.String
	}
	return PhotographerView{ID: uuidString(r.ID), Email: r.Email, BusinessName: r.BusinessName, Slug: r.Slug, PlanID: r.PlanID, Status: r.Status, StorageUsedBytes: r.StorageUsedBytes, StorageQuotaBytes: r.StorageQuotaBytes, CreatedAt: r.CreatedAt.Time.UTC(), LastLoginAt: last, SuspendedAt: susp, SuspendedReason: reason, Shoots: u.Shoots, Photos: u.Photos, Delivered: u.Delivered}, nil
}

func (s *Service) SuspendPhotographer(ctx context.Context, id pgtype.UUID, reason string, adminID pgtype.UUID, ip string) error {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return fmt.Errorf("%w: reason required", ErrInvalidInput)
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	qtx := s.q.WithTx(tx)
	n, err := qtx.SuspendPhotographer(ctx, store.SuspendPhotographerParams{ID: id, SuspendedReason: pgtype.Text{String: reason, Valid: true}})
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	if err := qtx.DeletePhotographerSessions(ctx, id); err != nil {
		return err
	}
	if err := writeAudit(ctx, qtx, adminID, "admin", "photographer.suspended", "photographer", uuidString(id), map[string]any{"reason": reason}, ip); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Service) UnsuspendPhotographer(ctx context.Context, id pgtype.UUID, adminID pgtype.UUID, ip string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	qtx := s.q.WithTx(tx)
	n, err := qtx.UnsuspendPhotographer(ctx, id)
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	if err := writeAudit(ctx, qtx, adminID, "admin", "photographer.unsuspended", "photographer", uuidString(id), map[string]any{}, ip); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Service) ChangePhotographerPlan(ctx context.Context, id pgtype.UUID, planID string, adminID pgtype.UUID, ip string) error {
	planID = strings.TrimSpace(planID)
	if planID == "" {
		return fmt.Errorf("%w: planId required", ErrInvalidInput)
	}
	if _, err := s.q.GetPlanByID(ctx, planID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("%w: unknown plan", ErrInvalidInput)
		}
		return err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	qtx := s.q.WithTx(tx)
	n, err := qtx.UpdatePhotographerPlan(ctx, store.UpdatePhotographerPlanParams{ID: id, PlanID: planID})
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	if err := writeAudit(ctx, qtx, adminID, "admin", "photographer.plan_changed", "photographer", uuidString(id), map[string]any{"planId": planID}, ip); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Service) InviteWaitlist(ctx context.Context, wl interface {
	GetByID(context.Context, pgtype.UUID) (waitlistpkg.Entry, error)
	MarkInvitedInTx(context.Context, pgx.Tx, pgtype.UUID, pgtype.UUID, string) (waitlistpkg.Entry, error)
}, waitlistID pgtype.UUID, email, planID string, ttl time.Duration, send bool, adminID pgtype.UUID, ip string, sender notify.EmailSender, fromName, fromAddr, publicAppURL string, inviteEmailLimiter interface{ Allow(string) bool }) (string, Invitation, string, error) {
	entry, err := wl.GetByID(ctx, waitlistID)
	if err != nil {
		return "", Invitation{}, "", err
	}
	if email == "" {
		email = entry.ContactEmail
	}
	email = strings.ToLower(strings.TrimSpace(email))
	planID = strings.TrimSpace(planID)
	if email == "" || !strings.Contains(email, "@") {
		return "", Invitation{}, "", fmt.Errorf("%w: email required for waitlist invite", ErrInvalidInput)
	}
	if planID == "" || ttl <= 0 {
		return "", Invitation{}, "", fmt.Errorf("%w: invalid invitation", ErrInvalidInput)
	}

	emailStatus := "not_sent"
	if send {
		if sender == nil {
			emailStatus = "no_sender"
		} else if inviteEmailLimiter != nil && !inviteEmailLimiter.Allow("admin:"+uuidString(adminID)) {
			emailStatus = "rate_limited"
		}
	}

	raw, err := newToken(32)
	if err != nil {
		return "", Invitation{}, "", err
	}
	hash := hashToken(raw)
	expiresAt := s.now().Add(ttl)

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", Invitation{}, "", err
	}
	defer tx.Rollback(ctx)
	qtx := s.q.WithTx(tx)
	if err := qtx.RevokeExpiredActiveInvitationsByEmail(ctx, email); err != nil {
		return "", Invitation{}, "", err
	}
	active, err := qtx.LockActiveInvitationsByEmail(ctx, email)
	if err != nil {
		return "", Invitation{}, "", err
	}
	for _, inv := range active {
		if inv.RevokedAt.Valid || inv.UsedAt.Valid || !inv.ExpiresAt.Time.After(s.now()) {
			continue
		}
		return "", Invitation{}, "", fmt.Errorf("%w: active invitation exists", ErrInvalidInput)
	}
	noteVal := pgtype.Text{String: "waitlist:" + uuidString(waitlistID), Valid: true}
	invRow, err := qtx.CreateInvitation(ctx, store.CreateInvitationParams{Email: email, TokenHash: hash, PlanID: planID, Note: noteVal, ExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: true}, CreatedBy: adminID})
	if err != nil {
		return "", Invitation{}, "", err
	}
	if _, err := wl.MarkInvitedInTx(ctx, tx, waitlistID, invRow.ID, email); err != nil {
		return "", Invitation{}, "", err
	}
	if err := (AuditEntry{AdminID: adminID, Actor: "admin", Action: "waitlist.invited", TargetType: "waitlist_entry", TargetID: uuidString(waitlistID), Meta: map[string]any{"email": maskEmail(email), "planId": planID, "send": send, "emailStatus": emailStatus, "invitationId": uuidString(invRow.ID)}, IP: ip}).Write(ctx, qtx); err != nil {
		return "", Invitation{}, "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", Invitation{}, "", err
	}
	inv := s.toInvitation(invRow)
	url := strings.TrimRight(publicAppURL, "/") + "/signup?invite=" + raw
	if send && emailStatus == "not_sent" && sender != nil {
		subject := "Your Photobooth invitation"
		text := "You've been invited to Photobooth.\n\nAccept your invitation: " + url + "\n\nThis link expires on " + inv.ExpiresAt.UTC().Format("2 January 2006") + ".\n\nYou're receiving this because you requested early access to Photobooth."
		html := "<p>You've been invited to Photobooth.</p><p><a href=\"" + url + "\">Accept your invitation</a></p><p>This link expires on " + inv.ExpiresAt.UTC().Format("2 January 2006") + ".</p><p>You're receiving this because you requested early access to Photobooth.</p>"
		if _, err := sender.Send(ctx, notify.Message{FromName: fromName, FromAddr: fromAddr, To: email, Subject: subject, Text: text, HTML: html}); err != nil {
			emailStatus = "failed"
		} else {
			emailStatus = "sent"
		}
	}
	return raw, inv, emailStatus, nil
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
		pos++
		out[pos] = hex[by&0x0f]
		pos++
	}
	return string(out)
}
