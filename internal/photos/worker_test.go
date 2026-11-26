package photos

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/riverqueue/river"

	"github.com/fiveace-Merill/photobooth/internal/photos/store"
	"github.com/fiveace-Merill/photobooth/internal/platform/database"
	"github.com/fiveace-Merill/photobooth/internal/platform/testdb"
	photostorage "github.com/fiveace-Merill/photobooth/internal/storage"
)

func TestProcessPhotoWorkerInvalidImageMarksFailed(t *testing.T) {
	tdb := testdb.Open(t)
	ctx := context.Background()
	p1, shoot1 := seedPhotographerShoot(t, ctx, tdb, "badimg@p.com", "badimg", 10<<20)
	fake := photostorage.NewFake()
	svc := NewService(Deps{Pool: tdb.App, Objects: fake, MaxPhotoBytes: 60 << 20, MaxPhotosPerShoot: 500, MaxImageMegapixels: 100})
	up, err := svc.CreateUpload(ctx, p1, shoot1, CreateUploadInput{Filename: "a.jpg", ContentType: "image/jpeg", SizeBytes: 4})
	if err != nil {
		t.Fatal(err)
	}
	// put invalid bytes as original object
	var origKey string
	err = database.WithTenant(ctx, tdb.App, pgtype.UUID{Bytes: p1, Valid: true}, func(tx pgx.Tx) error {
		p, err := store.New(tx).GetPhoto(ctx, store.GetPhotoParams{PhotographerID: pgtype.UUID{Bytes: p1, Valid: true}, ShootID: pgtype.UUID{Bytes: shoot1, Valid: true}, ID: pgtype.UUID{Bytes: up.PhotoID, Valid: true}})
		if err != nil {
			return err
		}
		origKey = p.OriginalKey
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := fake.Put(ctx, origKey, "image/jpeg", bytes.NewReader([]byte("not-image")), 9); err != nil {
		t.Fatal(err)
	}
	// simulate completion having marked photo processing
	err = database.WithTenant(ctx, tdb.App, pgtype.UUID{Bytes: p1, Valid: true}, func(tx pgx.Tx) error {
		_, err := store.New(tx).MarkPhotoProcessing(ctx, store.MarkPhotoProcessingParams{PhotographerID: pgtype.UUID{Bytes: p1, Valid: true}, ShootID: pgtype.UUID{Bytes: shoot1, Valid: true}, ID: pgtype.UUID{Bytes: up.PhotoID, Valid: true}})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}

	w := NewProcessPhotoWorker(tdb.App, fake, 100, nil)
	job := &river.Job[ProcessPhotoArgs]{Args: ProcessPhotoArgs{PhotographerID: p1, ShootID: shoot1, PhotoID: up.PhotoID}}
	err = w.Work(ctx, job)
	if err == nil {
		t.Fatal("expected error")
	}
	var cancelErr *river.JobCancelError
	if !errors.As(err, &cancelErr) {
		t.Fatalf("expected JobCancel, got %v", err)
	}

	var status string
	var perr *string
	err = database.WithTenant(ctx, tdb.App, pgtype.UUID{Bytes: p1, Valid: true}, func(tx pgx.Tx) error {
		p, err := store.New(tx).GetPhoto(ctx, store.GetPhotoParams{PhotographerID: pgtype.UUID{Bytes: p1, Valid: true}, ShootID: pgtype.UUID{Bytes: shoot1, Valid: true}, ID: pgtype.UUID{Bytes: up.PhotoID, Valid: true}})
		if err != nil {
			return err
		}
		status = p.Status
		if p.Error.Valid {
			s := p.Error.String
			perr = &s
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if status != "failed" || perr == nil || *perr != "invalid_image" {
		t.Fatalf("status=%s err=%v", status, perr)
	}
}

func TestProcessPhotoWorkerSuccess(t *testing.T) {
	tdb := testdb.Open(t)
	ctx := context.Background()
	p1, shoot1 := seedPhotographerShoot(t, ctx, tdb, "okimg@p.com", "okimg", 10<<20)
	fake := photostorage.NewFake()
	svc := NewService(Deps{Pool: tdb.App, Objects: fake, MaxPhotoBytes: 60 << 20, MaxPhotosPerShoot: 500, MaxImageMegapixels: 100})
	up, err := svc.CreateUpload(ctx, p1, shoot1, CreateUploadInput{Filename: "a.jpg", ContentType: "image/jpeg", SizeBytes: 4})
	if err != nil {
		t.Fatal(err)
	}
	var origKey, previewKey, thumbKey string
	err = database.WithTenant(ctx, tdb.App, pgtype.UUID{Bytes: p1, Valid: true}, func(tx pgx.Tx) error {
		p, err := store.New(tx).GetPhoto(ctx, store.GetPhotoParams{PhotographerID: pgtype.UUID{Bytes: p1, Valid: true}, ShootID: pgtype.UUID{Bytes: shoot1, Valid: true}, ID: pgtype.UUID{Bytes: up.PhotoID, Valid: true}})
		if err != nil {
			return err
		}
		origKey = p.OriginalKey
		previewKey = p.PreviewKey.String
		thumbKey = p.ThumbKey.String
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rect(0, 0, 20, 10))
	for y := 0; y < 10; y++ {
		for x := 0; x < 20; x++ {
			img.SetRGBA(x, y, color.RGBA{R: 200, G: 100, B: 50, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}
	if err := fake.Put(ctx, origKey, "image/jpeg", bytes.NewReader(buf.Bytes()), int64(buf.Len())); err != nil {
		t.Fatal(err)
	}
	err = database.WithTenant(ctx, tdb.App, pgtype.UUID{Bytes: p1, Valid: true}, func(tx pgx.Tx) error {
		_, err := store.New(tx).MarkPhotoProcessing(ctx, store.MarkPhotoProcessingParams{PhotographerID: pgtype.UUID{Bytes: p1, Valid: true}, ShootID: pgtype.UUID{Bytes: shoot1, Valid: true}, ID: pgtype.UUID{Bytes: up.PhotoID, Valid: true}})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}

	w := NewProcessPhotoWorker(tdb.App, fake, 100, nil)
	job := &river.Job[ProcessPhotoArgs]{Args: ProcessPhotoArgs{PhotographerID: p1, ShootID: shoot1, PhotoID: up.PhotoID}}
	if err := w.Work(ctx, job); err != nil {
		t.Fatalf("work: %v", err)
	}

	var status string
	err = database.WithTenant(ctx, tdb.App, pgtype.UUID{Bytes: p1, Valid: true}, func(tx pgx.Tx) error {
		p, err := store.New(tx).GetPhoto(ctx, store.GetPhotoParams{PhotographerID: pgtype.UUID{Bytes: p1, Valid: true}, ShootID: pgtype.UUID{Bytes: shoot1, Valid: true}, ID: pgtype.UUID{Bytes: up.PhotoID, Valid: true}})
		if err != nil {
			return err
		}
		status = p.Status
		if !p.Width.Valid || p.Width.Int32 != 20 {
			t.Fatalf("width=%v", p.Width)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if status != "ready" {
		t.Fatalf("status=%s", status)
	}
	if _, ok := fake.Object(previewKey); !ok {
		t.Fatal("missing preview")
	}
	if _, ok := fake.Object(thumbKey); !ok {
		t.Fatal("missing thumb")
	}
}

var _ = uuid.New
