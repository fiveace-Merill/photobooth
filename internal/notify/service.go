package notify

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/fiveace-Merill/photobooth/internal/notify/store"
	"github.com/fiveace-Merill/photobooth/internal/platform/database"
)

var NairobiLocation *time.Location

func init() {
	loc, err := time.LoadLocation("Africa/Nairobi")
	if err != nil {
		NairobiLocation = time.FixedZone("Africa/Nairobi", 3*60*60)
		return
	}
	NairobiLocation = loc
}

type Service struct {
	pool        *pgxpool.Pool
	sender      EmailSender
	fromName    string
	fromAddr    string
	dailyLimit  int
	sendTimeout time.Duration
	now         func() time.Time
	logger      *slog.Logger
}

type Config struct {
	FromName   string
	FromAddr   string
	DailyLimit int
}

func NewService(pool *pgxpool.Pool, sender EmailSender, cfg Config, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	if cfg.DailyLimit <= 0 {
		cfg.DailyLimit = 50
	}
	return &Service{
		pool:        pool,
		sender:      sender,
		fromName:    strings.TrimSpace(cfg.FromName),
		fromAddr:    strings.TrimSpace(cfg.FromAddr),
		dailyLimit:  cfg.DailyLimit,
		sendTimeout: 8 * time.Second,
		now:         time.Now,
		logger:      logger,
	}
}

func (s *Service) WithNow(now func() time.Time) *Service {
	if now != nil {
		s.now = now
	}
	return s
}

func (s *Service) WithSendTimeout(d time.Duration) *Service {
	if d > 0 {
		s.sendTimeout = d
	}
	return s
}

type SendDeliveryLinkInput struct {
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

type NotificationResult struct {
	Channel   string  `json:"channel"`
	Status    string  `json:"status"`
	ErrorCode *string `json:"errorCode,omitempty"`
}

type DeliveryEmailSendInput = SendDeliveryLinkInput

// SendDeliveryEmail satisfies delivery email notification needs without exposing notify store types.
func (s *Service) SendDeliveryEmail(ctx context.Context, in DeliveryEmailSendInput) (NotificationResult, error) {
	return s.SendDeliveryLink(ctx, in)
}

type NotificationView struct {
	Channel   string    `json:"channel"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"createdAt"`
	ErrorCode *string   `json:"errorCode,omitempty"`
	Recipient string    `json:"recipient"`
}

func (s *Service) CheckEmailAvailability(ctx context.Context, photographerID, shootID uuid.UUID, recipient string) (available bool, reason string, err error) {
	recipient = strings.TrimSpace(recipient)
	if err := ValidateRecipient(recipient); err != nil {
		return false, "channel_unavailable", nil
	}
	now := s.now().UTC()
	err = database.WithTenant(ctx, s.pool, pgUUID(photographerID), func(tx pgx.Tx) error {
		if err := lockEmailLimits(ctx, tx, photographerID, shootID, now); err != nil {
			return err
		}
		q := store.New(tx)
		shootCount, err := q.CountShootEmailAttemptsSince(ctx, store.CountShootEmailAttemptsSinceParams{PhotographerID: pgUUID(photographerID), ShootID: pgUUID(shootID), CreatedAt: pgTime(now.Add(-1 * time.Hour))})
		if err != nil {
			return err
		}
		if int(shootCount) >= 5 {
			available, reason = false, "too_many_sends"
			return nil
		}
		dayStart := nairobiDayStart(now)
		dailyCount, err := q.CountPhotographerEmailAttemptsSince(ctx, store.CountPhotographerEmailAttemptsSinceParams{PhotographerID: pgUUID(photographerID), CreatedAt: pgTime(dayStart)})
		if err != nil {
			return err
		}
		if int(dailyCount) >= s.dailyLimit {
			available, reason = false, "email_daily_limit_reached"
			return nil
		}
		available, reason = true, ""
		return nil
	})
	return available, reason, err
}

func (s *Service) SendDeliveryLink(ctx context.Context, in SendDeliveryLinkInput) (NotificationResult, error) {
	recipient := strings.TrimSpace(in.Recipient)
	if err := ValidateRecipient(recipient); err != nil {
		return NotificationResult{}, &LimitError{Status: 422, Code: "channel_unavailable", Message: "Client has no valid email address"}
	}
	now := s.now().UTC()
	var notifID uuid.UUID
	err := database.WithTenant(ctx, s.pool, pgUUID(in.PhotographerID), func(tx pgx.Tx) error {
		if err := lockEmailLimits(ctx, tx, in.PhotographerID, in.ShootID, now); err != nil {
			return err
		}
		q := store.New(tx)
		shootCount, err := q.CountShootEmailAttemptsSince(ctx, store.CountShootEmailAttemptsSinceParams{PhotographerID: pgUUID(in.PhotographerID), ShootID: pgUUID(in.ShootID), CreatedAt: pgTime(now.Add(-1 * time.Hour))})
		if err != nil {
			return err
		}
		if int(shootCount) >= 5 {
			return &LimitError{Status: 429, Code: "too_many_sends", Message: "Too many delivery emails were sent for this shoot recently"}
		}
		dayStart := nairobiDayStart(now)
		dailyCount, err := q.CountPhotographerEmailAttemptsSince(ctx, store.CountPhotographerEmailAttemptsSinceParams{PhotographerID: pgUUID(in.PhotographerID), CreatedAt: pgTime(dayStart)})
		if err != nil {
			return err
		}
		if int(dailyCount) >= s.dailyLimit {
			return &LimitError{Status: 429, Code: "email_daily_limit_reached", Message: "Daily email limit reached"}
		}
		row, err := q.InsertNotificationQueued(ctx, store.InsertNotificationQueuedParams{
			PhotographerID: pgUUID(in.PhotographerID),
			ShootID:        pgUUID(in.ShootID),
			ClientID:       pgUUID(in.ClientID),
			Recipient:      recipient,
		})
		if err != nil {
			return err
		}
		notifID = uuid.UUID(row.ID.Bytes)
		return nil
	})
	if err != nil {
		return NotificationResult{}, err
	}

	msg := BuildDeliveryEmail(DeliveryEmailInput{
		StudioName:    in.StudioName,
		StudioContact: in.StudioContact,
		ReplyTo:       in.ReplyTo,
		Recipient:     recipient,
		Title:         in.ShootTitle,
		DeliveryURL:   in.DeliveryURL,
		ExpiresAt:     in.ExpiresAt,
	})
	sendCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.sendTimeout)
	defer cancel()
	providerID, sendErr := s.sender.Send(sendCtx, Message{
		FromName: sanitizeHeader(fmt.Sprintf("%s via %s", strings.TrimSpace(in.StudioName), s.fromName)),
		FromAddr: s.fromAddr,
		ReplyTo:  sanitizeHeader(in.ReplyTo),
		To:       sanitizeHeader(recipient),
		Subject:  msg.Subject,
		Text:     msg.Text,
		HTML:     msg.HTML,
	})

	status := "sent"
	var errCode *string
	updateErr := database.WithTenant(ctx, s.pool, pgUUID(in.PhotographerID), func(tx pgx.Tx) error {
		q := store.New(tx)
		if sendErr != nil {
			code := ErrorCode(sendErr)
			status = "failed"
			errCode = &code
			return q.MarkNotificationFailed(ctx, store.MarkNotificationFailedParams{ID: pgUUID(notifID), ErrorCode: pgtype.Text{String: code, Valid: true}})
		}
		return q.MarkNotificationSent(ctx, store.MarkNotificationSentParams{ID: pgUUID(notifID), ProviderMessageID: pgtype.Text{String: providerID, Valid: providerID != ""}})
	})
	if updateErr != nil {
		s.logger.Error("update notification status failed", "err", updateErr, "notification_id", notifID, "shoot_id", in.ShootID)
		// The email may have been accepted; do not expose URL/token. Report failure conservatively.
		code := "send_failed"
		if sendErr == nil {
			// Provider accepted but DB update failed; keep conservative status.
			status = "failed"
			errCode = &code
		}
	}
	if sendErr != nil {
		s.logger.Warn("delivery email send failed", "code", ErrorCode(sendErr), "shoot_id", in.ShootID, "notification_id", notifID)
	}
	return NotificationResult{Channel: "email", Status: status, ErrorCode: errCode}, nil
}

func (s *Service) ListShootNotifications(ctx context.Context, photographerID, shootID uuid.UUID, limit int) ([]NotificationView, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	var out []NotificationView
	err := database.WithTenant(ctx, s.pool, pgUUID(photographerID), func(tx pgx.Tx) error {
		rows, err := store.New(tx).ListShootNotifications(ctx, store.ListShootNotificationsParams{PhotographerID: pgUUID(photographerID), ShootID: pgUUID(shootID), Limit: int32(limit)})
		if err != nil {
			return err
		}
		out = make([]NotificationView, 0, len(rows))
		for _, r := range rows {
			var code *string
			if r.ErrorCode.Valid {
				v := r.ErrorCode.String
				code = &v
			}
			out = append(out, NotificationView{Channel: r.Channel, Status: r.Status, CreatedAt: r.CreatedAt.Time.UTC(), ErrorCode: code, Recipient: MaskEmail(r.Recipient)})
		}
		return nil
	})
	return out, err
}

func (s *Service) EmailsRemainingToday(ctx context.Context, photographerID uuid.UUID) (int, error) {
	now := s.now().UTC()
	dayStart := nairobiDayStart(now)
	var remaining int
	err := database.WithTenant(ctx, s.pool, pgUUID(photographerID), func(tx pgx.Tx) error {
		// Lock photographer/day bucket for a stable view.
		if err := lockDailyOnly(ctx, tx, photographerID, now); err != nil {
			return err
		}
		count, err := store.New(tx).CountPhotographerEmailAttemptsSince(ctx, store.CountPhotographerEmailAttemptsSinceParams{PhotographerID: pgUUID(photographerID), CreatedAt: pgTime(dayStart)})
		if err != nil {
			return err
		}
		remaining = s.dailyLimit - int(count)
		if remaining < 0 {
			remaining = 0
		}
		return nil
	})
	return remaining, err
}

func lockEmailLimits(ctx context.Context, tx pgx.Tx, photographerID, shootID uuid.UUID, now time.Time) error {
	if err := lockDailyOnly(ctx, tx, photographerID, now); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('email_shoot_hour'), hashtext($1))`, photographerID.String()+":"+shootID.String())
	return err
}

func lockDailyOnly(ctx context.Context, tx pgx.Tx, photographerID uuid.UUID, now time.Time) error {
	day := nairobiDayStart(now).In(NairobiLocation).Format("2006-01-02")
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('email_daily'), hashtext($1))`, photographerID.String()+":"+day)
	return err
}

func nairobiDayStart(now time.Time) time.Time {
	n := now.In(NairobiLocation)
	y, m, d := n.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, NairobiLocation).UTC()
}

type LimitError struct {
	Status  int
	Code    string
	Message string
}

func (e *LimitError) Error() string { return e.Message }

func AsLimitError(err error) (*LimitError, bool) {
	var le *LimitError
	if errors.As(err, &le) {
		return le, true
	}
	return nil, false
}

func pgUUID(id uuid.UUID) pgtype.UUID       { return pgtype.UUID{Bytes: id, Valid: true} }
func pgTime(t time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: t.UTC(), Valid: true} }
