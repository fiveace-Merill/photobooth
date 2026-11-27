package admin_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/fiveace-Merill/photobooth/internal/platform/testdb"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestAdminResponsesContainNoTenantContent(t *testing.T) {
	tdb, mux, _ := setup(t)
	_, err := tdb.Owner.Exec(context.Background(), `INSERT INTO photographers (id,email,password_hash,business_name,slug,plan_id,status) VALUES ('11111111-1111-1111-1111-111111111111','p@example.com','hash','Studio','studio','free','active')`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = tdb.Owner.Exec(context.Background(), `INSERT INTO clients (id,photographer_id,name,phone,email) VALUES ('22222222-2222-2222-2222-222222222222','11111111-1111-1111-1111-111111111111','Secret Client','254712345678','client@example.com')`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = tdb.Owner.Exec(context.Background(), `INSERT INTO shoots (id,photographer_id,client_id,title,status) VALUES ('33333333-3333-3333-3333-333333333333','11111111-1111-1111-1111-111111111111','22222222-2222-2222-2222-222222222222','Secret Shoot','shot')`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = tdb.Owner.Exec(context.Background(), `INSERT INTO photos (id,photographer_id,shoot_id,filename,content_type,size_bytes,original_key,status) VALUES ('44444444-4444-4444-4444-444444444444','11111111-1111-1111-1111-111111111111','33333333-3333-3333-3333-333333333333','secret.jpg','image/jpeg',1,'orig/secret.jpg','ready')`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = tdb.Owner.Exec(context.Background(), `INSERT INTO gallery_links (id,photographer_id,shoot_id,token_hash,expires_at) VALUES ('55555555-5555-5555-5555-555555555555','11111111-1111-1111-1111-111111111111','33333333-3333-3333-3333-333333333333',decode('aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa','hex'), now()+interval '1 day')`)
	if err != nil {
		t.Fatal(err)
	}

	createAdmin(t, tdb, "admin@example.com", "super-secret-password")
	cookie, _ := confirmTOTP(t, tdb, mux, "admin@example.com", "super-secret-password")

	for _, path := range []string{"/api/v1/admin/overview", "/api/v1/admin/photographers", "/api/v1/admin/photographers/11111111-1111-1111-1111-111111111111"} {
		res := doJSON(mux, "GET", path, nil, cookie, "http://example.com")
		if res.Code != 200 {
			t.Fatalf("%s code=%d body=%s", path, res.Code, res.Body.String())
		}
		body := res.Body.Bytes()
		for _, forbidden := range []string{"Secret Client", "client@example.com", "254712345678", "Secret Shoot", "secret.jpg", "orig/secret.jpg", "gallery_links", "token_hash", "aaaaaaaaaaaaaaaa"} {
			if bytes.Contains(body, []byte(forbidden)) {
				t.Fatalf("%s leaked %q in %s", path, forbidden, string(body))
			}
		}
	}
}

func TestAdminAggregateFunctionsCountsOnly(t *testing.T) {
	tdb := testdb.Open(t)
	var cols int
	err := tdb.App.QueryRow(context.Background(), `SELECT COUNT(*) FROM admin_platform_stats()`).Scan(&cols)
	if err != nil {
		t.Fatal(err)
	}
	if cols != 1 {
		t.Fatalf("expected 1 row got %d", cols)
	}
	var total, active, suspended, shoots, photos, delivered30, storage int64
	err = tdb.App.QueryRow(context.Background(), `SELECT photographers_total,photographers_active,photographers_suspended,shoots_total,photos_total,shoots_delivered_last_30_days,storage_used_bytes_total FROM admin_platform_stats()`).Scan(&total, &active, &suspended, &shoots, &photos, &delivered30, &storage)
	if err != nil {
		t.Fatal(err)
	}
	var pid pgtype.UUID
	_ = pid.Scan("00000000-0000-0000-0000-000000000001")
	rows, err := tdb.App.Query(context.Background(), `SELECT photographer_id, shoots, photos, delivered FROM admin_photographer_usage($1::uuid[])`, []pgtype.UUID{pid})
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var id pgtype.UUID
		var s1, s2, s3 int64
		if err := rows.Scan(&id, &s1, &s2, &s3); err != nil {
			t.Fatal(err)
		}
	}
}
