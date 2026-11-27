package deliveries

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/fiveace-Merill/photobooth/internal/notify"
)

type NotifyAdapter struct {
	svc *notify.Service
}

func NewNotifyAdapter(svc *notify.Service) *NotifyAdapter {
	if svc == nil {
		return nil
	}
	return &NotifyAdapter{svc: svc}
}

func (a *NotifyAdapter) CheckEmailAvailability(ctx context.Context, photographerID, shootID uuid.UUID, recipient string) (bool, string, error) {
	return a.svc.CheckEmailAvailability(ctx, photographerID, shootID, recipient)
}

func (a *NotifyAdapter) ReserveDeliveryEmail(ctx context.Context, tx pgx.Tx, in DeliveryEmailSendInput) (ReservedEmail, error) {
	res, err := a.svc.ReserveDeliveryEmail(ctx, tx, notify.DeliveryEmailSendInput{
		PhotographerID: in.PhotographerID,
		ShootID:        in.ShootID,
		ClientID:       in.ClientID,
		Recipient:      in.Recipient,
		StudioName:     in.StudioName,
		StudioContact:  in.StudioContact,
		ReplyTo:        in.ReplyTo,
		ShootTitle:     in.ShootTitle,
		DeliveryURL:    in.DeliveryURL,
		ExpiresAt:      in.ExpiresAt,
	})
	if err != nil {
		return ReservedEmail{}, err
	}
	return ReservedEmail{ID: res.ID}, nil
}

func (a *NotifyAdapter) FinalizeDeliveryEmail(ctx context.Context, in DeliveryEmailSendInput, reservationID uuid.UUID) (NotificationResult, error) {
	res, err := a.svc.FinalizeDeliveryEmail(ctx, notify.DeliveryEmailSendInput{
		PhotographerID: in.PhotographerID,
		ShootID:        in.ShootID,
		ClientID:       in.ClientID,
		Recipient:      in.Recipient,
		StudioName:     in.StudioName,
		StudioContact:  in.StudioContact,
		ReplyTo:        in.ReplyTo,
		ShootTitle:     in.ShootTitle,
		DeliveryURL:    in.DeliveryURL,
		ExpiresAt:      in.ExpiresAt,
	}, reservationID)
	if err != nil {
		return NotificationResult{}, err
	}
	return NotificationResult{Channel: res.Channel, Status: res.Status, ErrorCode: res.ErrorCode}, nil
}

func (a *NotifyAdapter) ListShootNotifications(ctx context.Context, photographerID, shootID uuid.UUID, limit int) ([]NotificationView, error) {
	rows, err := a.svc.ListShootNotifications(ctx, photographerID, shootID, limit)
	if err != nil {
		return nil, err
	}
	out := make([]NotificationView, 0, len(rows))
	for _, r := range rows {
		out = append(out, NotificationView{Channel: r.Channel, Status: r.Status, CreatedAt: r.CreatedAt, ErrorCode: r.ErrorCode, Recipient: r.Recipient})
	}
	return out, nil
}

func (a *NotifyAdapter) EmailsRemainingToday(ctx context.Context, photographerID uuid.UUID) (int, error) {
	return a.svc.EmailsRemainingToday(ctx, photographerID)
}
