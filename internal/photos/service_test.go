package photos

import (
	"bytes"
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/fiveace-Merill/photobooth/internal/photos/store"
	"github.com/fiveace-Merill/photobooth/internal/platform/database"
	"github.com/fiveace-Merill/photobooth/internal/platform/testdb"
	photostorage "github.com/fiveace-Merill/photobooth/internal/storage"
)

func TestQuotaReserveRace(t *testing.T) {
	tdb := testdb.Open(t)
	ctx := context.Background()
	p1, shoot1 := seedPhotographerShoot(t, ctx, tdb, "race@p.com", "slug-race", 1024)
	fake := photostorage.NewFake()
	svc := NewService(Deps{Pool: tdb.App, Objects: fake, MaxPhotoBytes: 60 << 20, MaxPhotosPerShoot: 500, MaxImageMegapixels: 100})

	type res struct{ err error }
	ch := make(chan res, 2)
	for i := 0; i < 2; i++ {
		go func() {
			_, err := svc.CreateUpload(ctx, p1, shoot1, CreateUploadInput{Filename: "a.jpg", ContentType: "image/jpeg", SizeBytes: 1024})
			ch <- res{err: err}
		}()
	}
	r1, r2 := <-ch, <-ch
	okCount := 0
	quotaErr := false
	for _, r := range []res{r1, r2} {
		if r.err == nil {
			okCount++
			continue
		}
		if status, code, _, ok := AsServiceError(r.err); ok && status == 409 && code == "quota_exceeded" {
			quotaErr = true
		} else {
			t.Fatalf("unexpected err: %v", r.err)
		}
	}
	if okCount != 1 || !quotaErr {
		t.Fatalf("ok=%d quotaErr=%v", okCount, quotaErr)
	}
}

func TestCompleteSizeMismatchReleasesQuota(t *testing.T) {
	tdb := testdb.Open(t)
	ctx := context.Background()
	p1, shoot1 := seedPhotographerShoot(t, ctx, tdb, "size@p.com", "slug-size", 10<<20)
	fake := photostorage.NewFake()
	svc := NewService(Deps{Pool: tdb.App, Objects: fake, MaxPhotoBytes: 60 << 20, MaxPhotosPerShoot: 500, MaxImageMegapixels: 100})
	up, err := svc.CreateUpload(ctx, p1, shoot1, CreateUploadInput{Filename: "a.jpg", ContentType: "image/jpeg", SizeBytes: 4})
	if err != nil {
		t.Fatalf("create upload: %v", err)
	}

	// Find upload key/id, then place wrong-sized object in fake and complete.
	var origKey, uploadID string
	err = database.WithTenant(ctx, tdb.App, pgtype.UUID{Bytes: p1, Valid: true}, func(tx pgx.Tx) error {
		q := store.New(tx)
		p, err := q.GetPhoto(ctx, store.GetPhotoParams{PhotographerID: pgtype.UUID{Bytes: p1, Valid: true}, ShootID: pgtype.UUID{Bytes: shoot1, Valid: true}, ID: pgtype.UUID{Bytes: up.PhotoID, Valid: true}})
		if err != nil {
			return err
		}
		origKey = p.OriginalKey
		uploadID = p.UploadID.String
		return nil
	})
	if err != nil {
		t.Fatalf("get photo: %v", err)
	}
	if err := fake.AddUploadPart(uploadID, 1, []byte("toolong")); err != nil {
		t.Fatalf("add part: %v", err)
	}
	// CompleteMultipart in fake assembles parts into object; our service completes then checks HEAD size.
	_, err = svc.CompleteUpload(ctx, p1, shoot1, up.PhotoID, []CompletePart{{PartNumber: 1, ETag: "etag-1"}})
	if status, code, _, ok := AsServiceError(err); !ok || status != 422 || code != "size_mismatch" {
		t.Fatalf("expected size_mismatch, got %v", err)
	}
	// Quota should be released.
	var used int64
	if err := tdb.Owner.QueryRow(ctx, `SELECT storage_used_bytes FROM photographers WHERE id=$1`, p1).Scan(&used); err != nil {
		t.Fatalf("quota: %v", err)
	}
	if used != 0 {
		t.Fatalf("used=%d want 0", used)
	}
	if _, ok := fake.Object(origKey); ok {
		t.Fatalf("object should be deleted")
	}
}

func TestDeleteReleasesQuotaOnce(t *testing.T) {
	tdb := testdb.Open(t)
	ctx := context.Background()
	p1, shoot1 := seedPhotographerShoot(t, ctx, tdb, "del@p.com", "slug-del", 10<<20)
	fake := photostorage.NewFake()
	svc := NewService(Deps{Pool: tdb.App, Objects: fake, MaxPhotoBytes: 60 << 20, MaxPhotosPerShoot: 500, MaxImageMegapixels: 100})
	up, err := svc.CreateUpload(ctx, p1, shoot1, CreateUploadInput{Filename: "a.jpg", ContentType: "image/jpeg", SizeBytes: 100})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := svc.Delete(ctx, p1, shoot1, up.PhotoID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	var used int64
	if err := tdb.Owner.QueryRow(ctx, `SELECT storage_used_bytes FROM photographers WHERE id=$1`, p1).Scan(&used); err != nil {
		t.Fatal(err)
	}
	if used != 0 {
		t.Fatalf("used=%d", used)
	}
	err = svc.Delete(ctx, p1, shoot1, up.PhotoID)
	if status, code, _, ok := AsServiceError(err); !ok || status != 404 || code != "not_found" {
		t.Fatalf("second delete err=%v", err)
	}
	if err := tdb.Owner.QueryRow(ctx, `SELECT storage_used_bytes FROM photographers WHERE id=$1`, p1).Scan(&used); err != nil {
		t.Fatal(err)
	}
	if used != 0 {
		t.Fatalf("used after second delete=%d", used)
	}
}

func seedPhotographerShoot(t *testing.T, ctx context.Context, tdb *testdb.DB, email, slug string, quota int64) (uuid.UUID, uuid.UUID) {
	t.Helper()
	var pID, shootID, clientID uuid.UUID
	planID := "test-" + slug
	if _, err := tdb.Owner.Exec(ctx, `INSERT INTO plans (id, name, monthly_fee_minor, commission_bps, storage_quota_bytes, retention_days) VALUES ($1,$2,0,0,$3,30)`, planID, "Test "+slug, quota); err != nil {
		t.Fatalf("insert plan: %v", err)
	}
	if err := tdb.Owner.QueryRow(ctx, `INSERT INTO photographers (email, password_hash, business_name, slug, plan_id) VALUES ($1,'x','B',$2,$3) RETURNING id`, email, slug, planID).Scan(&pID); err != nil {
		t.Fatalf("insert photographer: %v", err)
	}
	if err := tdb.Owner.QueryRow(ctx, `INSERT INTO clients (photographer_id,name,phone) VALUES ($1,'C','254700000001') RETURNING id`, pID).Scan(&clientID); err != nil {
		t.Fatalf("insert client: %v", err)
	}
	if err := tdb.Owner.QueryRow(ctx, `INSERT INTO shoots (photographer_id,client_id,title,price_minor,currency,status) VALUES ($1,$2,'S',0,'KES','shot') RETURNING id`, pID, clientID).Scan(&shootID); err != nil {
		t.Fatalf("insert shoot: %v", err)
	}
	return pID, shootID
}

var _ = bytes.NewReader
