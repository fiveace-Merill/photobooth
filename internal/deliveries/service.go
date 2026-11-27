package deliveries

import (
	"archive/zip"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"path"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/fiveace-Merill/photobooth/internal/deliveries/store"
	"github.com/fiveace-Merill/photobooth/internal/notify"
	"github.com/fiveace-Merill/photobooth/internal/platform/database"
	photostorage "github.com/fiveace-Merill/photobooth/internal/storage"
)

var (
	ErrNotFound         = errors.New("delivery not found")
	ErrAlreadyDelivered = errCode(409, "already_delivered", "shoot already has an active delivery link")
	ErrNotDeliverable   = errCode(409, "shoot_not_deliverable", "shoot is not ready for delivery")
	ErrAlreadyUnlocked  = errCode(409, "already_unlocked", "originals are already unlocked")
	ErrNotMarkPaid      = errCode(409, "invalid_mark_paid", "shoot cannot be marked as paid")
	ErrLocked           = errCode(403, "originals_locked", "originals are locked")
)

type serviceError struct {
	status  int
	code    string
	message string
}

func (e *serviceError) Error() string { return e.message }
func errCode(status int, code, msg string) error {
	return &serviceError{status: status, code: code, message: msg}
}
func AsServiceError(err error) (status int, code, message string, ok bool) {
	var se *serviceError
	if errors.As(err, &se) {
		return se.status, se.code, se.message, true
	}
	return 0, "", "", false
}

type ObjectStore interface {
	PresignGet(ctx context.Context, key string, expires time.Duration, contentDisposition string) (string, error)
	Get(ctx context.Context, key string) (io.ReadCloser, error)
}

type NotificationResult struct {
	Channel   string  `json:"channel"`
	Status    string  `json:"status"`
	ErrorCode *string `json:"errorCode,omitempty"`
}

type NotificationView struct {
	Channel   string    `json:"channel"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"createdAt"`
	ErrorCode *string   `json:"errorCode,omitempty"`
	Recipient string    `json:"recipient"`
}

type DeliveryEmailSendInput struct {
	PhotographerID uuid.UUID
	ShootID        uuid.UUID
	ClientID       uuid.UUID
	Recipient      string
	StudioName     string
	StudioContact  string
	ReplyTo        string
	ShootTitle     string
	DeliveryURL    string
	ExpiresAt      time.Time
}

type ReservedEmail struct {
	ID uuid.UUID
}

type EmailNotifier interface {
	CheckEmailAvailability(ctx context.Context, photographerID, shootID uuid.UUID, recipient string) (available bool, reason string, err error)
	ReserveDeliveryEmail(ctx context.Context, tx pgx.Tx, in DeliveryEmailSendInput) (ReservedEmail, error)
	FinalizeDeliveryEmail(ctx context.Context, in DeliveryEmailSendInput, reservationID uuid.UUID) (NotificationResult, error)
	ListShootNotifications(ctx context.Context, photographerID, shootID uuid.UUID, limit int) ([]NotificationView, error)
	EmailsRemainingToday(ctx context.Context, photographerID uuid.UUID) (int, error)
}

type Service struct {
	pool      *pgxpool.Pool
	objects   ObjectStore
	notifier  EmailNotifier
	publicURL string
	now       func() time.Time
	logger    *slog.Logger
}

func NewService(pool *pgxpool.Pool, objects ObjectStore, publicURL string, notifier EmailNotifier) *Service {
	return &Service{pool: pool, objects: objects, notifier: notifier, publicURL: strings.TrimRight(publicURL, "/"), now: time.Now, logger: slog.Default()}
}

func (s *Service) WithNow(now func() time.Time) *Service {
	if now != nil {
		s.now = now
	}
	return s
}
func (s *Service) WithLogger(l *slog.Logger) *Service {
	if l != nil {
		s.logger = l
	}
	return s
}

type CreateLinkResult struct {
	URL           string               `json:"url"`
	ExpiresAt     time.Time            `json:"expiresAt"`
	Notifications []NotificationResult `json:"notifications,omitempty"`
}

type SendOptions struct {
	Email bool
}

type DeliveryState struct {
	LinkState    string          `json:"linkState"` // active, revoked, none
	ExpiresAt    *time.Time      `json:"expiresAt,omitempty"`
	LastViewedAt *time.Time      `json:"lastViewedAt,omitempty"`
	Unlocked     bool            `json:"unlocked"`
	Events       []DeliveryEvent `json:"events"`
}

type DeliveryEvent struct {
	ID        uuid.UUID       `json:"id"`
	ShootID   uuid.UUID       `json:"shootId"`
	PhotoID   *uuid.UUID      `json:"photoId,omitempty"`
	Kind      string          `json:"kind"`
	Payload   json.RawMessage `json:"payload"`
	CreatedAt time.Time       `json:"createdAt"`
}

type MarkPaidInput struct {
	Method string `json:"method"`
	Note   string `json:"note,omitempty"`
}

func (s *Service) Deliver(ctx context.Context, photographerID, shootID uuid.UUID, send SendOptions) (CreateLinkResult, error) {
	var res CreateLinkResult
	var emailTarget *DeliveryEmailSendInput
	var reserved *ReservedEmail
	err := database.WithTenant(ctx, s.pool, pgUUID(photographerID), func(tx pgx.Tx) error {
		q := store.New(tx)
		shoot, err := q.GetShootDeliveryInfo(ctx, store.GetShootDeliveryInfoParams{PhotographerID: pgUUID(photographerID), ID: pgUUID(shootID)})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		if _, err := q.GetActiveGalleryLink(ctx, store.GetActiveGalleryLinkParams{PhotographerID: pgUUID(photographerID), ShootID: pgUUID(shootID)}); err == nil {
			return ErrAlreadyDelivered
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		counts, err := q.CountShootPhotosByStatus(ctx, store.CountShootPhotosByStatusParams{PhotographerID: pgUUID(photographerID), ShootID: pgUUID(shootID)})
		if err != nil {
			return err
		}
		if counts.ReadyCount < 1 || counts.ProcessingCount > 0 {
			return ErrNotDeliverable
		}
		if send.Email {
			if err := s.prepareEmailTarget(ctx, tx, photographerID, shootID, shoot, &emailTarget, &reserved); err != nil {
				return err
			}
		}
		token, hash, err := newToken()
		if err != nil {
			return err
		}
		deliveredAt := s.now().UTC()
		expiresAt := deliveredAt.Add(time.Duration(shoot.RetentionDays) * 24 * time.Hour)
		unlock := shoot.PriceMinor == 0
		if _, err := q.MarkShootDelivered(ctx, store.MarkShootDeliveredParams{PhotographerID: pgUUID(photographerID), ID: pgUUID(shootID), Column3: unlock}); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		if _, err := q.InsertGalleryLink(ctx, store.InsertGalleryLinkParams{PhotographerID: pgUUID(photographerID), ShootID: pgUUID(shootID), TokenHash: hash, ExpiresAt: pgTime(expiresAt)}); err != nil {
			return err
		}
		payload, _ := json.Marshal(map[string]any{"expiresAt": expiresAt.Format(time.RFC3339), "priceMinor": shoot.PriceMinor, "unlocked": unlock})
		if err := q.CreateShootEvent(ctx, store.CreateShootEventParams{PhotographerID: pgUUID(photographerID), ShootID: pgUUID(shootID), PhotoID: pgtype.UUID{}, Kind: "shoot_delivered", Payload: payload}); err != nil {
			return err
		}
		if unlock {
			p2, _ := json.Marshal(map[string]any{"reason": "price_zero"})
			if err := q.CreateShootEvent(ctx, store.CreateShootEventParams{PhotographerID: pgUUID(photographerID), ShootID: pgUUID(shootID), PhotoID: pgtype.UUID{}, Kind: "originals_unlocked", Payload: p2}); err != nil {
				return err
			}
		}
		res = CreateLinkResult{URL: s.publicURL + "/g/" + token, ExpiresAt: expiresAt, Notifications: []NotificationResult{}}
		if emailTarget != nil {
			emailTarget.DeliveryURL = res.URL
			emailTarget.ExpiresAt = res.ExpiresAt
		}
		return nil
	})
	if err != nil {
		return CreateLinkResult{}, err
	}
	if emailTarget != nil && reserved != nil {
		n, sendErr := s.notifier.FinalizeDeliveryEmail(ctx, *emailTarget, reserved.ID)
		if sendErr != nil {
			s.logger.Warn("delivery email failed after link creation", "shoot_id", shootID, "code", notifyErrorCode(sendErr))
			code := notifyErrorCode(sendErr)
			n = NotificationResult{Channel: "email", Status: "failed", ErrorCode: &code}
		}
		res.Notifications = []NotificationResult{n}
	}
	return res, nil
}

func (s *Service) RegenerateLink(ctx context.Context, photographerID, shootID uuid.UUID, send SendOptions) (CreateLinkResult, error) {
	var res CreateLinkResult
	var emailTarget *DeliveryEmailSendInput
	var reserved *ReservedEmail
	err := database.WithTenant(ctx, s.pool, pgUUID(photographerID), func(tx pgx.Tx) error {
		q := store.New(tx)
		shoot, err := q.GetShootDeliveryInfo(ctx, store.GetShootDeliveryInfoParams{PhotographerID: pgUUID(photographerID), ID: pgUUID(shootID)})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		if _, err := q.GetLatestGalleryLink(ctx, store.GetLatestGalleryLinkParams{PhotographerID: pgUUID(photographerID), ShootID: pgUUID(shootID)}); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return errCode(409, "not_delivered", "shoot has not been delivered")
			}
			return err
		}
		if send.Email {
			if err := s.prepareEmailTarget(ctx, tx, photographerID, shootID, shoot, &emailTarget, &reserved); err != nil {
				return err
			}
		}
		if _, err := q.RevokeActiveGalleryLinks(ctx, store.RevokeActiveGalleryLinksParams{PhotographerID: pgUUID(photographerID), ShootID: pgUUID(shootID)}); err != nil {
			return err
		}
		token, hash, err := newToken()
		if err != nil {
			return err
		}
		base := shoot.DeliveredAt
		if !base.Valid {
			base = pgTime(s.now().UTC())
		}
		expiresAt := base.Time.UTC().Add(time.Duration(shoot.RetentionDays) * 24 * time.Hour)
		if _, err := q.InsertGalleryLink(ctx, store.InsertGalleryLinkParams{PhotographerID: pgUUID(photographerID), ShootID: pgUUID(shootID), TokenHash: hash, ExpiresAt: pgTime(expiresAt)}); err != nil {
			return err
		}
		payload, _ := json.Marshal(map[string]any{"expiresAt": expiresAt.Format(time.RFC3339)})
		if err := q.CreateShootEvent(ctx, store.CreateShootEventParams{PhotographerID: pgUUID(photographerID), ShootID: pgUUID(shootID), PhotoID: pgtype.UUID{}, Kind: "delivery_link_regenerated", Payload: payload}); err != nil {
			return err
		}
		res = CreateLinkResult{URL: s.publicURL + "/g/" + token, ExpiresAt: expiresAt, Notifications: []NotificationResult{}}
		if emailTarget != nil {
			emailTarget.DeliveryURL = res.URL
			emailTarget.ExpiresAt = res.ExpiresAt
		}
		return nil
	})
	if err != nil {
		return CreateLinkResult{}, err
	}
	if emailTarget != nil && reserved != nil {
		n, sendErr := s.notifier.FinalizeDeliveryEmail(ctx, *emailTarget, reserved.ID)
		if sendErr != nil {
			s.logger.Warn("delivery email failed after link creation", "shoot_id", shootID, "code", notifyErrorCode(sendErr))
			code := notifyErrorCode(sendErr)
			n = NotificationResult{Channel: "email", Status: "failed", ErrorCode: &code}
		}
		res.Notifications = []NotificationResult{n}
	}
	return res, nil
}

func (s *Service) RevokeLink(ctx context.Context, photographerID, shootID uuid.UUID) error {
	return database.WithTenant(ctx, s.pool, pgUUID(photographerID), func(tx pgx.Tx) error {
		q := store.New(tx)
		if _, err := q.GetShootDeliveryInfo(ctx, store.GetShootDeliveryInfoParams{PhotographerID: pgUUID(photographerID), ID: pgUUID(shootID)}); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		n, err := q.RevokeActiveGalleryLinks(ctx, store.RevokeActiveGalleryLinksParams{PhotographerID: pgUUID(photographerID), ShootID: pgUUID(shootID)})
		if err != nil {
			return err
		}
		if n == 0 {
			return errCode(409, "no_active_link", "no active delivery link")
		}
		payload, _ := json.Marshal(map[string]any{})
		return q.CreateShootEvent(ctx, store.CreateShootEventParams{PhotographerID: pgUUID(photographerID), ShootID: pgUUID(shootID), PhotoID: pgtype.UUID{}, Kind: "delivery_link_revoked", Payload: payload})
	})
}

func (s *Service) MarkPaid(ctx context.Context, photographerID, shootID uuid.UUID, in MarkPaidInput) error {
	switch in.Method {
	case "cash", "mpesa_direct", "other":
	default:
		return errCode(400, "invalid_input", "method must be cash, mpesa_direct, or other")
	}
	note := strings.TrimSpace(in.Note)
	if len(note) > 500 {
		return errCode(400, "invalid_input", "note too long")
	}
	return database.WithTenant(ctx, s.pool, pgUUID(photographerID), func(tx pgx.Tx) error {
		q := store.New(tx)
		shoot, err := q.GetShootDeliveryInfo(ctx, store.GetShootDeliveryInfoParams{PhotographerID: pgUUID(photographerID), ID: pgUUID(shootID)})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		if shoot.Status != "delivered" || shoot.PriceMinor <= 0 {
			return ErrNotMarkPaid
		}
		if shoot.OriginalsUnlockedAt.Valid {
			return ErrAlreadyUnlocked
		}
		if _, err := q.MarkShootPaid(ctx, store.MarkShootPaidParams{PhotographerID: pgUUID(photographerID), ID: pgUUID(shootID)}); err != nil {
			return err
		}
		payload, _ := json.Marshal(map[string]any{"method": in.Method, "note": note, "manual": true, "outsideCommissionSystem": true})
		return q.CreateShootEvent(ctx, store.CreateShootEventParams{PhotographerID: pgUUID(photographerID), ShootID: pgUUID(shootID), PhotoID: pgtype.UUID{}, Kind: "manual_mark_paid", Payload: payload})
	})
}

func asString(v any) string {
	s, _ := v.(string)
	return strings.TrimSpace(s)
}
func (s *Service) prepareEmailTarget(ctx context.Context, tx pgx.Tx, photographerID, shootID uuid.UUID, shoot store.GetShootDeliveryInfoRow, target **DeliveryEmailSendInput, reserved **ReservedEmail) error {
	if s.notifier == nil {
		return errCode(503, "email_unavailable", "Email delivery is not configured")
	}
	recipient := asString(shoot.ClientEmail)
	if recipient == "" {
		return errCode(422, "channel_unavailable", "Client has no email address")
	}
	if err := validateRecipientSyntax(recipient); err != nil {
		return errCode(422, "channel_unavailable", "Client has no valid email address")
	}
	contact := strings.TrimSpace(shoot.StudioPhone.String)
	if contact == "" {
		contact = asString(shoot.StudioEmail)
	}
	in := DeliveryEmailSendInput{
		PhotographerID: photographerID,
		ShootID:        shootID,
		ClientID:       uuid.UUID(shoot.ClientID.Bytes),
		Recipient:      recipient,
		StudioName:     shoot.StudioName,
		StudioContact:  contact,
		ReplyTo:        asString(shoot.StudioEmail),
		ShootTitle:     shoot.Title,
	}
	r, err := s.notifier.ReserveDeliveryEmail(ctx, tx, in)
	if err != nil {
		if le, ok := notify.AsLimitError(err); ok {
			return errCode(le.Status, le.Code, le.Message)
		}
		return err
	}
	*target = &in
	*reserved = &r
	return nil
}

func (s *Service) ListNotifications(ctx context.Context, photographerID, shootID uuid.UUID) ([]NotificationView, error) {
	if s.notifier == nil {
		return []NotificationView{}, nil
	}
	// Ensure shoot exists in tenant scope.
	err := database.WithTenant(ctx, s.pool, pgUUID(photographerID), func(tx pgx.Tx) error {
		_, err := store.New(tx).GetShootDeliveryInfo(ctx, store.GetShootDeliveryInfoParams{PhotographerID: pgUUID(photographerID), ID: pgUUID(shootID)})
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	})
	if err != nil {
		return nil, err
	}
	return s.notifier.ListShootNotifications(ctx, photographerID, shootID, 20)
}

type MessagePreview struct {
	Subject              string              `json:"subject"`
	BodyText             string              `json:"bodyText"`
	RecipientMasked      string              `json:"recipientMasked"`
	EmailsRemainingToday int                 `json:"emailsRemainingToday"`
	Availability         MessagePreviewAvail `json:"availability"`
}

type MessagePreviewAvail struct {
	Email MessageChannelAvailability `json:"email"`
}

type MessageChannelAvailability struct {
	Available bool   `json:"available"`
	Reason    string `json:"reason,omitempty"`
}

func (s *Service) GetMessagePreview(ctx context.Context, photographerID, shootID uuid.UUID) (MessagePreview, error) {
	var out MessagePreview
	err := database.WithTenant(ctx, s.pool, pgUUID(photographerID), func(tx pgx.Tx) error {
		q := store.New(tx)
		shoot, err := q.GetShootDeliveryInfo(ctx, store.GetShootDeliveryInfoParams{PhotographerID: pgUUID(photographerID), ID: pgUUID(shootID)})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		recipient := asString(shoot.ClientEmail)
		out.RecipientMasked = maskEmail(recipient)
		available := false
		reason := "channel_unavailable"
		if s.notifier != nil && recipient != "" && validateRecipientSyntax(recipient) == nil {
			ok, r, err := s.notifier.CheckEmailAvailability(ctx, photographerID, shootID, recipient)
			if err != nil {
				if le, ok := notify.AsLimitError(err); ok {
					reason = le.Code
				} else {
					return err
				}
			} else {
				available = ok
				reason = r
			}
		}
		remaining := 0
		if s.notifier != nil {
			remaining, err = s.notifier.EmailsRemainingToday(ctx, photographerID)
			if err != nil {
				return err
			}
		}
		contact := strings.TrimSpace(shoot.StudioPhone.String)
		if contact == "" {
			contact = asString(shoot.StudioEmail)
		}
		expiry := s.now().UTC().Add(time.Duration(shoot.RetentionDays) * 24 * time.Hour)
		msg := buildPreviewMessage(shoot.StudioName, shoot.Title, contact, expiry)
		out.Subject = msg.subject
		out.BodyText = msg.text
		out.EmailsRemainingToday = remaining
		out.Availability = MessagePreviewAvail{Email: MessageChannelAvailability{Available: available, Reason: reason}}
		return nil
	})
	return out, err
}

func (s *Service) GetDeliveryState(ctx context.Context, photographerID, shootID uuid.UUID) (DeliveryState, error) {
	var out DeliveryState
	err := database.WithTenant(ctx, s.pool, pgUUID(photographerID), func(tx pgx.Tx) error {
		q := store.New(tx)
		shoot, err := q.GetShootDeliveryInfo(ctx, store.GetShootDeliveryInfoParams{PhotographerID: pgUUID(photographerID), ID: pgUUID(shootID)})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		out.Unlocked = shoot.OriginalsUnlockedAt.Valid
		out.LinkState = "none"
		if link, err := q.GetLatestGalleryLink(ctx, store.GetLatestGalleryLinkParams{PhotographerID: pgUUID(photographerID), ShootID: pgUUID(shootID)}); err == nil {
			if link.RevokedAt.Valid {
				out.LinkState = "revoked"
			} else {
				out.LinkState = "active"
			}
			if link.ExpiresAt.Valid {
				t := link.ExpiresAt.Time.UTC()
				out.ExpiresAt = &t
			}
			if link.LastViewedAt.Valid {
				t := link.LastViewedAt.Time.UTC()
				out.LastViewedAt = &t
			}
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		events, err := q.ListShootEvents(ctx, store.ListShootEventsParams{PhotographerID: pgUUID(photographerID), ShootID: pgUUID(shootID), Limit: 20})
		if err != nil {
			return err
		}
		out.Events = make([]DeliveryEvent, 0, len(events))
		for _, e := range events {
			ev := DeliveryEvent{ID: uuid.UUID(e.ID.Bytes), ShootID: uuid.UUID(e.ShootID.Bytes), Kind: e.Kind, Payload: json.RawMessage(e.Payload), CreatedAt: e.CreatedAt.Time.UTC()}
			if e.PhotoID.Valid {
				id := uuid.UUID(e.PhotoID.Bytes)
				ev.PhotoID = &id
			}
			out.Events = append(out.Events, ev)
		}
		return nil
	})
	return out, err
}

type publicResolved struct {
	photographerID uuid.UUID
	shootID        uuid.UUID
	tokenHash      []byte
}

func (s *Service) resolveToken(ctx context.Context, rawToken string) (publicResolved, error) {
	hash := sha256.Sum256([]byte(rawToken))
	var pID, sID pgtype.UUID
	err := s.pool.QueryRow(ctx, `SELECT photographer_id, shoot_id FROM resolve_gallery_link($1)`, hash[:]).Scan(&pID, &sID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return publicResolved{}, ErrNotFound
		}
		return publicResolved{}, err
	}
	return publicResolved{photographerID: uuid.UUID(pID.Bytes), shootID: uuid.UUID(sID.Bytes), tokenHash: hash[:]}, nil
}

type PublicPhoto struct {
	ID          uuid.UUID `json:"id"`
	Width       int32     `json:"width"`
	Height      int32     `json:"height"`
	ThumbURL    string    `json:"thumbUrl"`
	PreviewURL  string    `json:"previewUrl"`
	OriginalURL string    `json:"originalUrl,omitempty"`
}

type PublicDelivery struct {
	Status             string         `json:"status"`
	StudioName         string         `json:"studioName"`
	StudioPhone        *string        `json:"studioPhone"`
	Title              string         `json:"title"`
	ShotOn             string         `json:"shotOn"`
	PriceMinor         int64          `json:"priceMinor"`
	Currency           string         `json:"currency"`
	ExpiresAt          string         `json:"expiresAt"`
	Features           PublicFeatures `json:"features"`
	Photos             []PublicPhoto  `json:"photos"`
	DownloadAllURL     string         `json:"downloadAllUrl,omitempty"`
	TotalOriginalBytes int64          `json:"totalOriginalBytes,omitempty"`
}

type PublicFeatures struct {
	Payments string `json:"payments"`
	Reviews  bool   `json:"reviews"`
}

func (s *Service) GetPublicDelivery(ctx context.Context, rawToken string) (PublicDelivery, error) {
	resolved, err := s.resolveToken(ctx, rawToken)
	if err != nil {
		return PublicDelivery{}, err
	}
	var out PublicDelivery
	err = database.WithTenant(ctx, s.pool, pgUUID(resolved.photographerID), func(tx pgx.Tx) error {
		q := store.New(tx)
		shoot, err := q.GetDeliveryShootForPublic(ctx, store.GetDeliveryShootForPublicParams{PhotographerID: pgUUID(resolved.photographerID), ID: pgUUID(resolved.shootID)})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		// Confirm link still active for this exact token hash and shoot and get expiry.
		var expiresAt time.Time
		err = tx.QueryRow(ctx, `SELECT expires_at FROM gallery_links WHERE photographer_id=$1 AND shoot_id=$2 AND token_hash=$3 AND revoked_at IS NULL AND expires_at > now()`, pgUUID(resolved.photographerID), pgUUID(resolved.shootID), resolved.tokenHash).Scan(&expiresAt)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		_, _ = q.TouchGalleryLinkViewed(ctx, store.TouchGalleryLinkViewedParams{PhotographerID: pgUUID(resolved.photographerID), ShootID: pgUUID(resolved.shootID), TokenHash: resolved.tokenHash})

		rows, err := q.ListReadyPhotosForDelivery(ctx, store.ListReadyPhotosForDeliveryParams{PhotographerID: pgUUID(resolved.photographerID), ShootID: pgUUID(resolved.shootID)})
		if err != nil {
			return err
		}
		unlocked := shoot.OriginalsUnlockedAt.Valid
		out.Status = "preview"
		if unlocked {
			out.Status = "unlocked"
		}
		out.StudioName = shoot.StudioName
		if shoot.StudioPhone.Valid {
			v := shoot.StudioPhone.String
			out.StudioPhone = &v
		}
		out.Title = shoot.Title
		if shoot.ShootDate.Valid {
			out.ShotOn = shoot.ShootDate.Time.Format("2006-01-02")
		} else {
			out.ShotOn = ""
		}
		out.PriceMinor = shoot.PriceMinor
		out.Currency = shoot.Currency
		out.ExpiresAt = expiresAt.UTC().Format(time.RFC3339)
		out.Features = PublicFeatures{Payments: "manual", Reviews: false}
		out.Photos = make([]PublicPhoto, 0, len(rows))
		var totalBytes int64
		for _, p := range rows {
			thumb, err := s.objects.PresignGet(ctx, p.ThumbKey.String, photostorage.GetTTL, "")
			if err != nil {
				return err
			}
			preview, err := s.objects.PresignGet(ctx, p.PreviewKey.String, photostorage.GetTTL, "")
			if err != nil {
				return err
			}
			photo := PublicPhoto{ID: uuid.UUID(p.ID.Bytes), Width: p.Width.Int32, Height: p.Height.Int32, ThumbURL: thumb, PreviewURL: preview}
			if unlocked {
				photo.OriginalURL = fmt.Sprintf("/api/v1/deliveries/%s/photos/%s/original", url.PathEscape(rawToken), uuid.UUID(p.ID.Bytes).String())
				totalBytes += p.SizeBytes
			}
			out.Photos = append(out.Photos, photo)
		}
		if unlocked {
			out.DownloadAllURL = fmt.Sprintf("/api/v1/deliveries/%s/download", url.PathEscape(rawToken))
			out.TotalOriginalBytes = totalBytes
		}
		return nil
	})
	if err != nil {
		return PublicDelivery{}, err
	}
	return out, nil
}

type OriginalRedirect struct {
	URL      string
	Filename string
}

func (s *Service) GetOriginalRedirect(ctx context.Context, rawToken string, photoID uuid.UUID) (OriginalRedirect, error) {
	resolved, err := s.resolveToken(ctx, rawToken)
	if err != nil {
		return OriginalRedirect{}, err
	}
	var out OriginalRedirect
	err = database.WithTenant(ctx, s.pool, pgUUID(resolved.photographerID), func(tx pgx.Tx) error {
		q := store.New(tx)
		shoot, err := q.GetDeliveryShootForPublic(ctx, store.GetDeliveryShootForPublicParams{PhotographerID: pgUUID(resolved.photographerID), ID: pgUUID(resolved.shootID)})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		if !shoot.OriginalsUnlockedAt.Valid {
			return ErrLocked
		}
		p, err := q.GetReadyPhotoForDelivery(ctx, store.GetReadyPhotoForDeliveryParams{PhotographerID: pgUUID(resolved.photographerID), ShootID: pgUUID(resolved.shootID), ID: pgUUID(photoID)})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		cd := contentDisposition(p.Filename)
		u, err := s.objects.PresignGet(ctx, p.OriginalKey, 15*time.Minute, cd)
		if err != nil {
			return err
		}
		out = OriginalRedirect{URL: u, Filename: p.Filename}
		return nil
	})
	return out, err
}

type ZipPhoto struct {
	Filename    string
	OriginalKey string
}

type ZipInfo struct {
	ShootTitle string
	Photos     []ZipPhoto
}

func (s *Service) GetZipInfo(ctx context.Context, rawToken string) (ZipInfo, uuid.UUID, uuid.UUID, error) {
	resolved, err := s.resolveToken(ctx, rawToken)
	if err != nil {
		return ZipInfo{}, uuid.UUID{}, uuid.UUID{}, err
	}
	var out ZipInfo
	err = database.WithTenant(ctx, s.pool, pgUUID(resolved.photographerID), func(tx pgx.Tx) error {
		q := store.New(tx)
		shoot, err := q.GetDeliveryShootForPublic(ctx, store.GetDeliveryShootForPublicParams{PhotographerID: pgUUID(resolved.photographerID), ID: pgUUID(resolved.shootID)})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		if !shoot.OriginalsUnlockedAt.Valid {
			return ErrLocked
		}
		rows, err := q.ListReadyPhotosForDelivery(ctx, store.ListReadyPhotosForDeliveryParams{PhotographerID: pgUUID(resolved.photographerID), ShootID: pgUUID(resolved.shootID)})
		if err != nil {
			return err
		}
		out.ShootTitle = shoot.Title
		out.Photos = make([]ZipPhoto, 0, len(rows))
		for _, p := range rows {
			out.Photos = append(out.Photos, ZipPhoto{Filename: p.Filename, OriginalKey: p.OriginalKey})
		}
		return nil
	})
	return out, resolved.photographerID, resolved.shootID, err
}

func (s *Service) WriteZip(ctx context.Context, w io.Writer, info ZipInfo) error {
	zw := zip.NewWriter(w)
	used := map[string]int{}
	for _, p := range info.Photos {
		name := safeZipName(p.Filename, used)
		fh := &zip.FileHeader{Name: name, Method: zip.Store}
		fh.Modified = s.now().UTC()
		fw, err := zw.CreateHeader(fh)
		if err != nil {
			return err
		}
		rc, err := s.objects.Get(ctx, p.OriginalKey)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(fw, rc)
		closeErr := rc.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return zw.Close()
}

func (s *Service) LogDownloadEvent(ctx context.Context, photographerID, shootID uuid.UUID) {
	_ = database.WithTenant(ctx, s.pool, pgUUID(photographerID), func(tx pgx.Tx) error {
		q := store.New(tx)
		payload, _ := json.Marshal(map[string]any{"at": s.now().UTC().Format(time.RFC3339)})
		return q.CreateShootEvent(ctx, store.CreateShootEventParams{PhotographerID: pgUUID(photographerID), ShootID: pgUUID(shootID), PhotoID: pgtype.UUID{}, Kind: "delivery_download_all", Payload: payload})
	})
}

func newToken() (raw string, hash []byte, err error) {
	var b [32]byte
	if _, err = rand.Read(b[:]); err != nil {
		return "", nil, err
	}
	raw = base64.RawURLEncoding.EncodeToString(b[:])
	sum := sha256.Sum256([]byte(raw))
	return raw, sum[:], nil
}

func pgUUID(id uuid.UUID) pgtype.UUID       { return pgtype.UUID{Bytes: id, Valid: true} }
func pgTime(t time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: t.UTC(), Valid: true} }

func contentDisposition(filename string) string {
	fallback := asciiFallback(filename)
	// RFC 5987 filename*
	escaped := url.PathEscape(filename)
	return fmt.Sprintf("attachment; filename=\"%s\"; filename*=UTF-8''%s", fallback, escaped)
}

func asciiFallback(name string) string {
	base := name
	if base == "" {
		base = "download"
	}
	var b strings.Builder
	for _, r := range base {
		if r >= 32 && r < 127 && r != '"' && r != '\\' && r != ';' {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	out := strings.TrimSpace(b.String())
	if out == "" {
		return "download"
	}
	return out
}

var safeZipRe = regexp.MustCompile(`[^A-Za-z0-9._ -]+`)

func safeZipName(filename string, used map[string]int) string {
	name := strings.TrimSpace(filename)
	name = strings.ReplaceAll(name, "\\", "/")
	name = path.Base(name)
	if name == "." || name == "/" || name == "" {
		name = "photo"
	}
	name = safeZipRe.ReplaceAllString(name, "_")
	if !utf8.ValidString(name) {
		name = "photo"
	}
	ext := path.Ext(name)
	base := strings.TrimSuffix(name, ext)
	if base == "" {
		base = "photo"
	}
	key := strings.ToLower(name)
	if n, ok := used[key]; ok {
		n++
		used[key] = n
		name = fmt.Sprintf("%s (%d)%s", base, n, ext)
	} else {
		used[key] = 0
	}
	return name
}

func SanitizeZipBase(title string) string {
	name := safeZipRe.ReplaceAllString(strings.TrimSpace(title), "_")
	name = strings.Trim(name, " ._-")
	if name == "" {
		name = "photos"
	}
	if len(name) > 80 {
		name = strings.TrimSpace(name[:80])
	}
	return name
}
