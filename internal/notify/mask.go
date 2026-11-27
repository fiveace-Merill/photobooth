package notify

import "strings"

func MaskEmail(email string) string {
	email = strings.TrimSpace(email)
	at := strings.LastIndex(email, "@")
	if at <= 0 || at == len(email)-1 {
		if len(email) <= 2 {
			return "***"
		}
		return email[:1] + "***"
	}
	local := email[:at]
	domain := email[at+1:]
	maskedLocal := local[:1] + "***"
	if len(local) > 1 {
		// keep first rune only, avoid leaking length details beyond minimal shape
		maskedLocal = string([]rune(local)[0]) + "***"
	}
	return maskedLocal + "@" + domain
}
