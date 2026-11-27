package notify

import (
	"strings"
	"testing"
	"time"
)

func TestBuildDeliveryEmailContainsLinkOnce(t *testing.T) {
	msg := BuildDeliveryEmail(DeliveryEmailInput{
		StudioName:    "Studio",
		StudioContact: "+254700000000",
		Recipient:     "client@example.com",
		Title:         "Wedding",
		DeliveryURL:   "https://app.test/g/token123",
		ExpiresAt:     time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC),
	})
	if got := strings.Count(msg.Text, "https://app.test/g/token123"); got != 1 {
		t.Fatalf("text link count=%d\n%s", got, msg.Text)
	}
	if got := strings.Count(msg.HTML, "https://app.test/g/token123"); got != 1 {
		// single button href
		t.Fatalf("html link count=%d\n%s", got, msg.HTML)
	}
}

func TestBuildDeliveryEmailEscapesHostileValues(t *testing.T) {
	msg := BuildDeliveryEmail(DeliveryEmailInput{
		StudioName:    "<script>alert(1)</script>",
		StudioContact: "0712\r\nBcc:evil@example.com",
		Recipient:     "client@example.com",
		Title:         "Title\r\nInjected: x <b>",
		DeliveryURL:   "https://app.test/g/token123",
		ExpiresAt:     time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC),
	})
	if strings.Contains(msg.HTML, "<script>") || strings.Contains(msg.HTML, "Injected: x <b>") {
		t.Fatalf("html not escaped: %s", msg.HTML)
	}
	if !strings.Contains(msg.HTML, "&lt;script&gt;alert(1)&lt;/script&gt;") {
		t.Fatalf("expected escaped studio name: %s", msg.HTML)
	}
	if strings.Contains(msg.Subject, "\r") || strings.Contains(msg.Subject, "\n") {
		t.Fatalf("subject has header injection: %q", msg.Subject)
	}
}
