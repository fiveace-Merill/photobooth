package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds environment-driven settings for the API process.
// Keeping this as a plain struct makes dependencies explicit: handlers/services
// receive the values they need instead of reading os.Getenv everywhere.
type Config struct {
	// DatabaseURL is the connection string for the non-owner app role.
	// The API must use this role because it is subject to row-level security.
	DatabaseURL string

	// Port is the HTTP listen port, without a leading colon in config form.
	// main.go turns it into an address like ":8080".
	Port string

	// CookieSecure controls the Secure flag on session cookies.
	// It must be true in production (HTTPS), but local plain HTTP development
	// needs it false or the browser will refuse to store the cookie.
	CookieSecure bool

	// SignupEnabled gates public self-signup. The product is invite-only for now,
	// so this defaults to false. When false, signup returns a stable 403 code.
	SignupEnabled bool

	// StorageDriver selects object storage implementation: "r2" or "fake".
	// "fake" is intended for tests/local development without R2 credentials.
	StorageDriver string

	// R2 config is required when StorageDriver is "r2".
	R2AccountID       string
	R2AccessKeyID     string
	R2SecretAccessKey string
	R2Bucket          string

	// PublicAppURL is the public base URL of the web app (no trailing slash).
	PublicAppURL string

	// Photo processing/upload limits.
	MaxPhotoBytes      int64
	MaxPhotosPerShoot  int
	MaxImageMegapixels int
	RiverMaxWorkers    int

	// PhotoTestDelay slows process_photo before processing. It is for kill/recovery
	// testing only and is ignored when APP_ENV=production.
	PhotoTestDelay time.Duration

	// Email delivery (slice 4). Never log secrets or delivery URLs/tokens.
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

// Load reads configuration from environment variables and validates required values.
// It returns an error instead of panicking so callers (main, tests) can decide
// how to handle missing configuration.
func Load() (Config, error) {
	cfg := Config{
		DatabaseURL:        os.Getenv("DATABASE_URL"),
		Port:               os.Getenv("PORT"),
		CookieSecure:       true,
		SignupEnabled:      false,
		StorageDriver:      strings.ToLower(strings.TrimSpace(getenvDefault("STORAGE_DRIVER", "r2"))),
		R2AccountID:        strings.TrimSpace(os.Getenv("R2_ACCOUNT_ID")),
		R2AccessKeyID:      strings.TrimSpace(os.Getenv("R2_ACCESS_KEY_ID")),
		R2SecretAccessKey:  os.Getenv("R2_SECRET_ACCESS_KEY"),
		R2Bucket:           strings.TrimSpace(os.Getenv("R2_BUCKET")),
		PublicAppURL:       strings.TrimRight(strings.TrimSpace(getenvDefault("PUBLIC_APP_URL", "http://localhost:3000")), "/"),
		MaxPhotoBytes:      60 << 20,
		MaxPhotosPerShoot:  500,
		MaxImageMegapixels: 50,
		RiverMaxWorkers:    1,
	}

	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.Port == "" {
		cfg.Port = "8080"
	}

	if raw := os.Getenv("COOKIE_SECURE"); raw != "" {
		secure, err := strconv.ParseBool(raw)
		if err != nil {
			return Config{}, fmt.Errorf("COOKIE_SECURE must be a boolean: %w", err)
		}
		cfg.CookieSecure = secure
	}
	if raw := os.Getenv("SIGNUP_ENABLED"); raw != "" {
		enabled, err := strconv.ParseBool(raw)
		if err != nil {
			return Config{}, fmt.Errorf("SIGNUP_ENABLED must be a boolean: %w", err)
		}
		cfg.SignupEnabled = enabled
	}

	if raw := os.Getenv("MAX_PHOTO_BYTES"); raw != "" {
		v, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || v <= 0 {
			return Config{}, fmt.Errorf("MAX_PHOTO_BYTES must be a positive integer")
		}
		cfg.MaxPhotoBytes = v
	}
	if raw := os.Getenv("MAX_PHOTOS_PER_SHOOT"); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil || v <= 0 {
			return Config{}, fmt.Errorf("MAX_PHOTOS_PER_SHOOT must be a positive integer")
		}
		cfg.MaxPhotosPerShoot = v
	}
	if raw := os.Getenv("MAX_IMAGE_MEGAPIXELS"); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil || v <= 0 {
			return Config{}, fmt.Errorf("MAX_IMAGE_MEGAPIXELS must be a positive integer")
		}
		cfg.MaxImageMegapixels = v
	}
	if raw := os.Getenv("RIVER_MAX_WORKERS"); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil || v <= 0 {
			return Config{}, fmt.Errorf("RIVER_MAX_WORKERS must be a positive integer")
		}
		cfg.RiverMaxWorkers = v
	}
	if raw := os.Getenv("PHOTO_TEST_DELAY"); raw != "" {
		d, err := time.ParseDuration(raw)
		if err != nil || d < 0 {
			return Config{}, fmt.Errorf("PHOTO_TEST_DELAY must be a non-negative duration")
		}
		if strings.EqualFold(strings.TrimSpace(os.Getenv("APP_ENV")), "production") {
			d = 0
		}
		cfg.PhotoTestDelay = d
	}

	cfg.EmailProvider = strings.ToLower(strings.TrimSpace(getenvDefault("EMAIL_PROVIDER", "log")))
	cfg.SMTPHost = strings.TrimSpace(getenvDefault("SMTP_HOST", "mailpit"))
	cfg.SMTPPort = 1025
	if raw := os.Getenv("SMTP_PORT"); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil || v <= 0 || v > 65535 {
			return Config{}, fmt.Errorf("SMTP_PORT must be a valid port number")
		}
		cfg.SMTPPort = v
	}
	cfg.SMTPUser = os.Getenv("SMTP_USER")
	cfg.SMTPPassword = os.Getenv("SMTP_PASSWORD")
	cfg.SMTPTLS = strings.ToLower(strings.TrimSpace(getenvDefault("SMTP_TLS", "none")))
	cfg.MailFromAddress = strings.TrimSpace(getenvDefault("MAIL_FROM_ADDRESS", "no-reply@photobooth.local"))
	cfg.MailFromName = strings.TrimSpace(getenvDefault("MAIL_FROM_NAME", "Photobooth"))
	cfg.EmailDailyLimitPerPhotographer = 50
	if raw := os.Getenv("EMAIL_DAILY_LIMIT_PER_PHOTOGRAPHER"); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil || v <= 0 {
			return Config{}, fmt.Errorf("EMAIL_DAILY_LIMIT_PER_PHOTOGRAPHER must be a positive integer")
		}
		cfg.EmailDailyLimitPerPhotographer = v
	}
	switch cfg.EmailProvider {
	case "log", "smtp":
	default:
		return Config{}, fmt.Errorf("EMAIL_PROVIDER must be log or smtp")
	}
	switch cfg.SMTPTLS {
	case "starttls", "implicit", "none":
	default:
		return Config{}, fmt.Errorf("SMTP_TLS must be starttls, implicit, or none")
	}
	if cfg.MailFromAddress == "" {
		return Config{}, fmt.Errorf("MAIL_FROM_ADDRESS is required")
	}
	if strings.EqualFold(strings.TrimSpace(os.Getenv("APP_ENV")), "production") && cfg.EmailProvider == "log" {
		return Config{}, fmt.Errorf("EMAIL_PROVIDER=log is not allowed when APP_ENV=production")
	}

	switch cfg.StorageDriver {
	case "r2":
		missing := make([]string, 0, 4)
		if cfg.R2AccountID == "" {
			missing = append(missing, "R2_ACCOUNT_ID")
		}
		if cfg.R2AccessKeyID == "" {
			missing = append(missing, "R2_ACCESS_KEY_ID")
		}
		if cfg.R2SecretAccessKey == "" {
			missing = append(missing, "R2_SECRET_ACCESS_KEY")
		}
		if cfg.R2Bucket == "" {
			missing = append(missing, "R2_BUCKET")
		}
		if len(missing) > 0 {
			return Config{}, fmt.Errorf("storage driver r2 requires: %s", strings.Join(missing, ", "))
		}
	case "fake":
		// Useful for tests/local dev without R2 credentials.
	default:
		return Config{}, fmt.Errorf("STORAGE_DRIVER must be r2 or fake")
	}

	return cfg, nil
}

func getenvDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
