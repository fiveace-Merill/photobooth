package notify

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net/mail"
	"strings"
	"time"

	gomail "github.com/wneessen/go-mail"
)

// ErrLogProviderInProduction is returned when the log email provider is used in production.
var ErrLogProviderInProduction = errors.New("email log provider is not allowed in production")

type Message struct {
	FromName string
	FromAddr string
	ReplyTo  string
	To       string
	Subject  string
	Text     string
	HTML     string
}

type EmailSender interface {
	Send(ctx context.Context, msg Message) (providerMessageID string, err error)
}

type SMTPConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	TLS      string // starttls, implicit, none
}

type smtpSender struct {
	cfg SMTPConfig
}

func NewSMTPSender(cfg SMTPConfig) EmailSender {
	return &smtpSender{cfg: cfg}
}

func (s *smtpSender) Send(ctx context.Context, msg Message) (string, error) {
	client, err := s.newClient()
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrSendFailed, err)
	}
	m := gomail.NewMsg()
	if err := m.FromFormat(msg.FromName, msg.FromAddr); err != nil {
		return "", fmt.Errorf("%w: invalid from", ErrSendFailed)
	}
	if err := m.To(msg.To); err != nil {
		return "", fmt.Errorf("%w: invalid recipient", ErrRecipientRejected)
	}
	if strings.TrimSpace(msg.ReplyTo) != "" {
		if err := m.ReplyTo(msg.ReplyTo); err != nil {
			return "", fmt.Errorf("%w: invalid reply-to", ErrSendFailed)
		}
	}
	m.Subject(msg.Subject)
	m.SetBodyString(gomail.TypeTextPlain, msg.Text)
	m.AddAlternativeString(gomail.TypeTextHTML, msg.HTML)
	m.SetMessageID()

	if err := client.DialAndSendWithContext(ctx, m); err != nil {
		return "", fmt.Errorf("%w: %v", mapSMTPError(ctx, err), err)
	}
	id := m.GetMessageID()
	return id, nil
}

func (s *smtpSender) newClient() (*gomail.Client, error) {
	opts := []gomail.Option{
		gomail.WithPort(s.cfg.Port),
		gomail.WithTimeout(8 * time.Second),
	}
	switch strings.ToLower(strings.TrimSpace(s.cfg.TLS)) {
	case "implicit":
		opts = append(opts, gomail.WithSSL(), gomail.WithTLSPolicy(gomail.TLSMandatory))
	case "starttls":
		opts = append(opts, gomail.WithTLSPolicy(gomail.TLSMandatory), gomail.WithTLSConfig(&tls.Config{ServerName: s.cfg.Host, MinVersion: tls.VersionTLS12}))
	case "none":
		opts = append(opts, gomail.WithTLSPolicy(gomail.NoTLS))
	default:
		return nil, fmt.Errorf("invalid smtp tls mode")
	}
	if strings.TrimSpace(s.cfg.User) != "" {
		opts = append(opts, gomail.WithSMTPAuth(gomail.SMTPAuthPlain), gomail.WithUsername(s.cfg.User), gomail.WithPassword(s.cfg.Password))
	}
	return gomail.NewClient(s.cfg.Host, opts...)
}

func mapSMTPError(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return ErrSendTimeout
	}
	var sendErr *gomail.SendError
	if errors.As(err, &sendErr) {
		if isPermanentSMTPError(sendErr) {
			return ErrRecipientRejected
		}
		return ErrSendFailed
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "timeout") {
		return ErrSendTimeout
	}
	if strings.Contains(msg, "550") || strings.Contains(msg, "551") || strings.Contains(msg, "552") || strings.Contains(msg, "553") || strings.Contains(msg, "554") || strings.Contains(msg, "5.1") || strings.Contains(msg, "5.7") || strings.Contains(msg, "permanent") {
		return ErrRecipientRejected
	}
	return ErrSendFailed
}

func isPermanentSMTPError(err *gomail.SendError) bool {
	if err == nil {
		return false
	}
	// go-mail SendError may wrap enhanced status codes in Reason/Error; check string defensively.
	s := strings.ToLower(err.Error())
	for _, code := range []string{" 5.1.", " 5.7.", "550 ", "551 ", "552 ", "553 ", "554 "} {
		if strings.Contains(" "+s+" ", code) {
			return true
		}
	}
	return strings.HasPrefix(s, "5")
}

type logSender struct {
	logger      *slog.Logger
	allowURLLog bool
}

func NewLogSender(logger *slog.Logger, appEnv string) (EmailSender, error) {
	if strings.EqualFold(strings.TrimSpace(appEnv), "production") {
		return nil, ErrLogProviderInProduction
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &logSender{logger: logger, allowURLLog: strings.EqualFold(strings.TrimSpace(appEnv), "development")}, nil
}

func (s *logSender) Send(ctx context.Context, msg Message) (string, error) {
	_ = ctx
	// Never log secrets. Log URL only in development for local Mailpit/testing convenience.
	attrs := []any{"to_masked", MaskEmail(msg.To), "subject", sanitizeHeader(msg.Subject)}
	if s.allowURLLog {
		if u := extractLink(msg.Text); u != "" {
			attrs = append(attrs, "delivery_url_dev_only", u)
		}
	}
	s.logger.Info("email sent via log provider", attrs...)
	return fmt.Sprintf("<log-%d@photobooth.local>", time.Now().UnixNano()), nil
}

func extractLink(text string) string {
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "http://") || strings.HasPrefix(line, "https://") {
			return line
		}
	}
	return ""
}

func ValidateRecipient(addr string) error {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return errors.New("missing recipient")
	}
	parsed, err := mail.ParseAddress(addr)
	if err != nil || parsed.Address == "" {
		return errors.New("invalid recipient")
	}
	// Reject obvious header/newline injection in stored addresses.
	if strings.ContainsAny(addr, "\r\n") {
		return errors.New("invalid recipient")
	}
	return nil
}
