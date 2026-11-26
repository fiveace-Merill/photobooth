package config

import (
	"fmt"
	"os"
	"strconv"
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
}

// Load reads configuration from environment variables and validates required values.
// It returns an error instead of panicking so callers (main, tests) can decide
// how to handle missing configuration.
func Load() (Config, error) {
	cfg := Config{
		DatabaseURL:   os.Getenv("DATABASE_URL"),
		Port:          os.Getenv("PORT"),
		CookieSecure:  true,
		SignupEnabled: false,
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

	return cfg, nil
}
