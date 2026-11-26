package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"time"

	"github.com/fiveace-Merill/photobooth/internal/platform/config"
	photostorage "github.com/fiveace-Merill/photobooth/internal/storage"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if cfg.StorageDriver != "r2" {
		fmt.Fprintln(os.Stderr, "STORAGE_DRIVER must be r2")
		os.Exit(1)
	}
	ctx := context.Background()
	store, err := photostorage.NewR2(ctx, photostorage.R2Config{AccountID: cfg.R2AccountID, AccessKeyID: cfg.R2AccessKeyID, SecretAccessKey: cfg.R2SecretAccessKey, Bucket: cfg.R2Bucket})
	if err != nil {
		fatal(err)
	}
	key := fmt.Sprintf("r2check/%d.txt", time.Now().UnixNano())
	uid, err := store.InitiateMultipart(ctx, key, "text/plain")
	if err != nil {
		fatal(err)
	}
	_, _ = store.PresignUploadPart(ctx, key, uid, 1, 5*time.Minute)
	_ = store.AbortMultipart(ctx, key, uid)

	data := []byte("ok")
	if err := store.Put(ctx, key, "text/plain", bytes.NewReader(data), int64(len(data))); err != nil {
		fatal(err)
	}
	info, err := store.Head(ctx, key)
	if err != nil {
		fatal(err)
	}
	if info.Size != int64(len(data)) {
		fatal(fmt.Errorf("size mismatch: got %d", info.Size))
	}
	if _, err := store.PresignGet(ctx, key, time.Minute, ""); err != nil {
		fatal(err)
	}
	if err := store.Delete(ctx, key); err != nil {
		fatal(err)
	}
	fmt.Println("r2check ok")
}

func fatal(err error) { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
