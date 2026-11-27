package notify

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/fiveace-Merill/photobooth/internal/notify/store"
	"github.com/fiveace-Merill/photobooth/internal/platform/database"
)

type ReservedNotification struct {
	ID        uuid.UUID
	Recipient string
}

func (s *Service) ReserveDeliveryEmail(ctx context.Context, tx pgx.Tx, in SendDeliveryLinkInput) (ReservedNotification, error) {
	recipient := strings.TrimSpace(in.Recipient)
	if err := ValidateRecipient(recipient); err != nil {
		return ReservedNotification{}, &LimitError{Status: 422, Code: "channel_unavailable", Message: "Client has no valid email address"}
	}
	now := s.now().UTC()
	if err := lockEmailLimits(ctx, tx, in.PhotographerID, in.ShootID, now); err != nil {
		return ReservedNotification{}, err
	}
	q := store.New(tx)
	shootCount, err := q.CountShootEmailAttemptsSince(ctx, store.CountShootEmailAttemptsSinceParams{PhotographerID: pgUUID(in.PhotographerID), ShootID: pgUUID(in.ShootID), CreatedAt: pgTime(now.Add(-1 * time.Hour))})
	if err != nil {
		return ReservedNotification{}, err
	}
	if int(shootCount) >= 5 {
		return ReservedNotification{}, &LimitError{Status: 429, Code: "too_many_sends", Message: "Too many delivery emails were sent for this shoot recently"}
	}
	dayStart := nairobiDayStart(now)
	dailyCount, err := q.CountPhotographerEmailAttemptsSince(ctx, store.CountPhotographerEmailAttemptsSinceParams{PhotographerID: pgUUID(in.PhotographerID), CreatedAt: pgTime(dayStart)})
	if err != nil {
		return ReservedNotification{}, err
	}
	if int(dailyCount) >= s.dailyLimit {
		return ReservedNotification{}, &LimitError{Status: 429, Code: "email_daily_limit_reached", Message: "Daily email limit reached"}
	}
	row, err := q.InsertNotificationQueued(ctx, store.InsertNotificationQueuedParams{
		PhotographerID: pgUUID(in.PhotographerID),
		ShootID:        pgUUID(in.ShootID),
		ClientID:       pgUUID(in.ClientID),
		Recipient:      recipient,
	})
	if err != nil {
		return ReservedNotification{}, err
	}
	return ReservedNotification{ID: uuid.UUID(row.ID.Bytes), Recipient: recipient}, nil
}

func (s *Service) FinalizeDeliveryEmail(ctx context.Context, in SendDeliveryLinkInput, notifID uuid.UUID) (NotificationResult, error) {
	msg := BuildDeliveryEmail(DeliveryEmailInput{
		StudioName:    in.StudioName,
		StudioContact: in.StudioContact,
		ReplyTo:       in.ReplyTo,
		Recipient:     in.Recipient,
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
		To:       sanitizeHeader(in.Recipient),
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
		code := "send_failed"
		status = "failed"
		errCode = &code
	}
	if sendErr != nil {
		s.logger.Warn("delivery email send failed", "code", ErrorCode(sendErr), "shoot_id", in.ShootID, "notification_id", notifID)
	}
	return NotificationResult{Channel: "email", Status: status, ErrorCode: errCode}, nil
}
