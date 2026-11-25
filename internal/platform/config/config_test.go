package config

import "testing"

// TestLoad verifies env parsing/validation. These are unit tests: they do not
// touch the database. t.Setenv registers cleanup automatically, so each subtest
// gets an isolated environment view.
func TestLoad(t *testing.T) {
	t.Run("requires DATABASE_URL", func(t *testing.T) {
		t.Setenv("DATABASE_URL", "")
		t.Setenv("PORT", "")
		t.Setenv("COOKIE_SECURE", "")

		_, err := Load()
		if err == nil {
			t.Fatal("expected error when DATABASE_URL is missing")
		}
	})

	t.Run("defaults", func(t *testing.T) {
		t.Setenv("DATABASE_URL", "postgres://app:pw@localhost:5432/photobooth?sslmode=disable")
		t.Setenv("PORT", "")
		t.Setenv("COOKIE_SECURE", "")

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if cfg.Port != "8080" {
			t.Fatalf("Port = %q, want 8080", cfg.Port)
		}
		if !cfg.CookieSecure {
			t.Fatal("CookieSecure default = false, want true")
		}
	})

	t.Run("parses explicit values", func(t *testing.T) {
		t.Setenv("DATABASE_URL", "postgres://app:pw@localhost:5432/photobooth?sslmode=disable")
		t.Setenv("PORT", "9090")
		t.Setenv("COOKIE_SECURE", "false")

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if cfg.Port != "9090" {
			t.Fatalf("Port = %q, want 9090", cfg.Port)
		}
		if cfg.CookieSecure {
			t.Fatal("CookieSecure = true, want false")
		}
	})

	t.Run("rejects invalid COOKIE_SECURE", func(t *testing.T) {
		t.Setenv("DATABASE_URL", "postgres://app:pw@localhost:5432/photobooth?sslmode=disable")
		t.Setenv("COOKIE_SECURE", "maybe")

		_, err := Load()
		if err == nil {
			t.Fatal("expected error for invalid COOKIE_SECURE")
		}
	})
}
