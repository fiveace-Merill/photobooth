package notify

import (
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
)

type FactoryConfig struct {
	AppEnv                         string
	EmailProvider                  string
	SMTPHost                       string
	SMTPPort                       int
	SMTPUser                       string
	SMTPPassword                   string
	SMTPTLS                        string
	MailFromAddress                string
	MailFromName                   string
	EmailDailyLimitPerPhotographer int
}

func NewSenderFromConfig(cfg FactoryConfig, logger *slog.Logger) (EmailSender, error) {
	switch cfg.EmailProvider {
	case "log":
		return NewLogSender(logger, cfg.AppEnv)
	case "smtp":
		return NewSMTPSender(SMTPConfig{Host: cfg.SMTPHost, Port: cfg.SMTPPort, User: cfg.SMTPUser, Password: cfg.SMTPPassword, TLS: cfg.SMTPTLS}), nil
	default:
		return nil, fmt.Errorf("unsupported email provider: %s", cfg.EmailProvider)
	}
}

func NewServiceFromConfig(pool *pgxpool.Pool, cfg FactoryConfig, logger *slog.Logger) (*Service, error) {
	sender, err := NewSenderFromConfig(cfg, logger)
	if err != nil {
		return nil, err
	}
	return NewService(pool, sender, Config{FromName: cfg.MailFromName, FromAddr: cfg.MailFromAddress, DailyLimit: cfg.EmailDailyLimitPerPhotographer}, logger), nil
}
