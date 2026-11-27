package deliveries

import (
	"fmt"
	"net/mail"
	"strings"
	"time"
)

type previewMsg struct {
	subject string
	text    string
}

func buildPreviewMessage(studioName, title, contact string, expiresAt time.Time) previewMsg {
	studio := strings.TrimSpace(studioName)
	if studio == "" {
		studio = "Your photographer"
	}
	if strings.TrimSpace(title) == "" {
		title = "your photos"
	}
	expiry := expiresAt.UTC().Format("2 January 2006")
	subject := sanitizeHeader(fmt.Sprintf("Your photos from %s are ready", title))
	var b strings.Builder
	b.WriteString(fmt.Sprintf("%s shared these photos with you.\n\n", studio))
	b.WriteString("View your photos:\n")
	b.WriteString("https://photobooth.example/g/preview\n\n")
	b.WriteString("This link expires on ")
	b.WriteString(expiry)
	b.WriteString(".\n\n")
	b.WriteString("From: ")
	b.WriteString(studio)
	b.WriteString("\n")
	if strings.TrimSpace(contact) != "" {
		b.WriteString("Contact: ")
		b.WriteString(strings.TrimSpace(contact))
		b.WriteString("\n")
	}
	b.WriteString("\nIf you were not expecting this email, you can ignore it.\n")
	return previewMsg{subject: subject, text: b.String()}
}

func maskEmail(email string) string {
	email = strings.TrimSpace(email)
	at := strings.LastIndex(email, "@")
	if at <= 0 || at == len(email)-1 {
		if email == "" {
			return ""
		}
		return "***"
	}
	local := []rune(email[:at])
	domain := email[at+1:]
	return string(local[0]) + "***@" + domain
}

func validateRecipientSyntax(addr string) error {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return fmt.Errorf("missing")
	}
	p, err := mail.ParseAddress(addr)
	if err != nil || p == nil || strings.TrimSpace(p.Address) == "" {
		return fmt.Errorf("invalid")
	}
	if strings.ContainsAny(addr, "\r\n") {
		return fmt.Errorf("invalid")
	}
	return nil
}

func sanitizeHeader(v string) string {
	v = strings.ReplaceAll(v, "\r", " ")
	v = strings.ReplaceAll(v, "\n", " ")
	return strings.TrimSpace(v)
}

func notifyErrorCode(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "recipient_rejected"):
		return "recipient_rejected"
	case strings.Contains(msg, "send_timeout"), strings.Contains(msg, "deadline exceeded"):
		return "send_timeout"
	default:
		return "send_failed"
	}
}
