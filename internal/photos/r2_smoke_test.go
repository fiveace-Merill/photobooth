package photos_test

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/fiveace-Merill/photobooth/internal/platform/config"
	photostorage "github.com/fiveace-Merill/photobooth/internal/storage"
)

func TestR2Smoke(t *testing.T) {
	if os.Getenv("R2_SMOKE") != "1" {
		t.Skip("set R2_SMOKE=1 to run real R2 smoke test")
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	store, err := photostorage.NewR2(context.Background(), photostorage.R2Config{
		AccountID: cfg.R2AccountID, AccessKeyID: cfg.R2AccessKeyID, SecretAccessKey: cfg.R2SecretAccessKey, Bucket: cfg.R2Bucket,
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	key := "smoke/test.txt"
	if err := store.Put(ctx, key, "text/plain", bytes.NewReader([]byte("ok")), 2); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Head(ctx, key); err != nil {
		t.Fatal(err)
	}
	if _, err := store.PresignGet(ctx, key, 60, ""); err != nil {
		t.Fatal(err)
	}
	if err := store.Delete(ctx, key); err != nil {
		t.Fatal(err)
	}
}
