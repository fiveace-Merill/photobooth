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
		t.Setenv("STORAGE_DRIVER", "fake")

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
		if cfg.MaxPhotoBytes != 60<<20 {
			t.Fatalf("MaxPhotoBytes = %d", cfg.MaxPhotoBytes)
		}
		if cfg.EmailProvider != "log" {
			t.Fatalf("EmailProvider = %q", cfg.EmailProvider)
		}
		if cfg.SMTPHost != "mailpit" || cfg.SMTPPort != 1025 || cfg.SMTPTLS != "none" {
			t.Fatalf("unexpected smtp defaults: %+v", cfg)
		}
		if cfg.MailFromAddress != "no-reply@photobooth.local" || cfg.MailFromName != "Photobooth" {
			t.Fatalf("unexpected mail from defaults: %+v", cfg)
		}
		if cfg.EmailDailyLimitPerPhotographer != 50 {
			t.Fatalf("EmailDailyLimitPerPhotographer = %d", cfg.EmailDailyLimitPerPhotographer)
		}
	})

	t.Run("parses explicit values", func(t *testing.T) {
		t.Setenv("DATABASE_URL", "postgres://app:pw@localhost:5432/photobooth?sslmode=disable")
		t.Setenv("PORT", "9090")
		t.Setenv("COOKIE_SECURE", "false")
		t.Setenv("STORAGE_DRIVER", "fake")
		t.Setenv("MAX_PHOTO_BYTES", "1024")
		t.Setenv("MAX_PHOTOS_PER_SHOOT", "10")
		t.Setenv("MAX_IMAGE_MEGAPIXELS", "50")
		t.Setenv("RIVER_MAX_WORKERS", "3")

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
		if cfg.MaxPhotoBytes != 1024 || cfg.MaxPhotosPerShoot != 10 || cfg.MaxImageMegapixels != 50 || cfg.RiverMaxWorkers != 3 {
			t.Fatalf("unexpected limits: %+v", cfg)
		}
		if cfg.PhotoTestDelay != 0 {
			t.Fatalf("PhotoTestDelay = %v, want 0", cfg.PhotoTestDelay)
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

func TestLoadPhotoTestDelayIgnoredInProduction(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://app:pw@localhost:5432/photobooth?sslmode=disable")
	t.Setenv("STORAGE_DRIVER", "fake")
	t.Setenv("APP_ENV", "production")
	t.Setenv("PHOTO_TEST_DELAY", "90s")
	t.Setenv("EMAIL_PROVIDER", "smtp")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.PhotoTestDelay != 0 {
		t.Fatalf("delay=%v want 0 in production", cfg.PhotoTestDelay)
	}
}

func TestLoadRejectsLogEmailProviderInProduction(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://app:pw@localhost:5432/photobooth?sslmode=disable")
	t.Setenv("STORAGE_DRIVER", "fake")
	t.Setenv("APP_ENV", "production")
	t.Setenv("EMAIL_PROVIDER", "log")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
}
