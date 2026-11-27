package notify

import (
	"fmt"
	"html"
	"strings"
	"time"
)

type DeliveryEmailInput struct {
	StudioName    string
	StudioContact string
	ReplyTo       string
	Recipient     string
	Title         string
	DeliveryURL   string
	ExpiresAt     time.Time
}

type BuiltMessage struct {
	Subject string
	Text    string
	HTML    string
}

func BuildDeliveryEmail(in DeliveryEmailInput) BuiltMessage {
	studio := strings.TrimSpace(in.StudioName)
	if studio == "" {
		studio = "Your photographer"
	}
	title := strings.TrimSpace(in.Title)
	if title == "" {
		title = "your photos"
	}
	contact := strings.TrimSpace(in.StudioContact)
	expiry := in.ExpiresAt.UTC().Format("2 January 2006")
	url := strings.TrimSpace(in.DeliveryURL)

	subject := sanitizeHeader(fmt.Sprintf("Your photos from %s are ready", title))
	why := fmt.Sprintf("%s shared these photos with you.", studio)

	var text strings.Builder
	text.WriteString(why)
	text.WriteString("\n\n")
	text.WriteString("View your photos:\n")
	text.WriteString(url)
	text.WriteString("\n\n")
	text.WriteString("This link expires on ")
	text.WriteString(expiry)
	text.WriteString(".\n\n")
	text.WriteString("From: ")
	text.WriteString(studio)
	text.WriteString("\n")
	if contact != "" {
		text.WriteString("Contact: ")
		text.WriteString(contact)
		text.WriteString("\n")
	}
	text.WriteString("\nIf you were not expecting this email, you can ignore it.\n")

	htmlBody := buildHTML(studio, title, why, url, expiry, contact)
	return BuiltMessage{Subject: subject, Text: text.String(), HTML: htmlBody}
}

func buildHTML(studio, title, why, url, expiry, contact string) string {
	escStudio := html.EscapeString(studio)
	escTitle := html.EscapeString(title)
	escWhy := html.EscapeString(why)
	escURL := html.EscapeString(url)
	escExpiry := html.EscapeString(expiry)
	escContact := html.EscapeString(contact)
	contactLine := ""
	if contact != "" {
		contactLine = fmt.Sprintf(`<p style="margin:0 0 8px 0;color:#444;font-size:14px;line-height:1.5;"><strong>Contact:</strong> %s</p>`, escContact)
	}
	return fmt.Sprintf(`<!doctype html>
<html>
  <body style="margin:0;padding:24px;background:#f7f7f5;font-family:Arial,sans-serif;color:#1d1d1b;">
    <div style="max-width:560px;margin:0 auto;background:#ffffff;padding:24px;border:1px solid #e5e5e0;">
      <p style="margin:0 0 16px 0;font-size:14px;color:#555;line-height:1.5;">%s</p>
      <h1 style="margin:0 0 12px 0;font-size:24px;line-height:1.3;color:#1d1d1b;">Your photos from %s are ready</h1>
      <p style="margin:0 0 20px 0;font-size:16px;line-height:1.5;color:#333;">Your gallery is ready to view.</p>
      <p style="margin:0 0 24px 0;">
        <a href="%s" style="display:inline-block;background:#d4ff00;color:#1d1d1b;text-decoration:none;padding:12px 18px;font-weight:700;font-size:14px;">View photos</a>
      </p>
      <p style="margin:0 0 8px 0;color:#444;font-size:14px;line-height:1.5;"><strong>Link expires:</strong> %s</p>
      <p style="margin:0 0 8px 0;color:#444;font-size:14px;line-height:1.5;"><strong>From:</strong> %s</p>
      %s
      <p style="margin:24px 0 0 0;color:#666;font-size:12px;line-height:1.5;">If you were not expecting this email, you can ignore it.</p>
    </div>
  </body>
</html>`, escWhy, escTitle, escURL, escExpiry, escStudio, contactLine)
}

func sanitizeHeader(v string) string {
	v = strings.ReplaceAll(v, "\r", " ")
	v = strings.ReplaceAll(v, "\n", " ")
	v = strings.ReplaceAll(v, "\x00", "")
	return strings.TrimSpace(v)
}
