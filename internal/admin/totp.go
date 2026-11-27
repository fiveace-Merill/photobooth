package admin

import (
	"crypto/rand"
	"encoding/base32"
	"fmt"
	"strings"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

func generateTOTPSecret(accountName string) (secret string, otpauthURL string, err error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "Photobooth",
		AccountName: accountName,
		Period:      30,
		Digits:      otp.DigitsSix,
		Algorithm:   otp.AlgorithmSHA1,
	})
	if err != nil {
		return "", "", fmt.Errorf("generate totp: %w", err)
	}
	return key.Secret(), key.URL(), nil
}

func validateTOTPCode(secret, code string) bool {
	code = strings.TrimSpace(code)
	return totp.Validate(code, secret)
}

func generateRecoveryCodes(n int) ([]string, error) {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	out := make([]string, 0, n)
	for i := 0; i < n; i++ {
		b := make([]byte, 10)
		if _, err := rand.Read(b); err != nil {
			return nil, err
		}
		var sb strings.Builder
		for j, by := range b {
			if j == 5 {
				sb.WriteByte('-')
			}
			sb.WriteByte(alphabet[int(by)%len(alphabet)])
		}
		out = append(out, sb.String())
	}
	return out, nil
}

func normalizeRecoveryCode(code string) string {
	code = strings.ToUpper(strings.TrimSpace(code))
	code = strings.ReplaceAll(code, "-", "")
	code = strings.ReplaceAll(code, " ", "")
	if len(code) == 10 {
		return code[:5] + "-" + code[5:]
	}
	return strings.ToUpper(strings.TrimSpace(code))
}

// ensure base32 import used? we store secret as returned by otp lib (base32 string)
var _ = base32.StdEncoding
var _ = time.Now
