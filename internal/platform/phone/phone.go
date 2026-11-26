package phone

import (
	"fmt"
	"regexp"
	"strings"
)

var kenyanMobileRE = regexp.MustCompile(`^(?:\+?254|0)?([71]\d{8})$`)

// Normalize accepts Kenyan mobile numbers in common local/international forms
// and returns the canonical Daraja-friendly format: 2547XXXXXXXX or 2541XXXXXXXX.
//
// We normalize before storing because the DB unique index is on
// (photographer_id, phone); different spellings of the same number must become
// the same stored value.
func Normalize(s string) (string, error) {
	cleaned := strings.ReplaceAll(strings.ReplaceAll(strings.TrimSpace(s), " ", ""), "-", "")
	m := kenyanMobileRE.FindStringSubmatch(cleaned)
	if m == nil {
		return "", fmt.Errorf("invalid Kenyan mobile phone number")
	}
	return "254" + m[1], nil
}
