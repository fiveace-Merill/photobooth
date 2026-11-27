package waitlist_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/fiveace-Merill/photobooth/internal/platform/ratelimit"
	"github.com/fiveace-Merill/photobooth/internal/platform/testdb"
	"github.com/fiveace-Merill/photobooth/internal/waitlist"
	"github.com/fiveace-Merill/photobooth/internal/waitlist/store"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestWaitlistSubmitDuplicateHoneypotIdentical(t *testing.T) {
	tdb := testdb.Open(t)
	svc := waitlist.NewService(store.New(tdb.App), tdb.App)
	h := waitlist.NewHandler(svc)
	mux := http.NewServeMux()
	h.Register(mux)

	body := map[string]any{"name": "Jane", "contact": "jane@example.com", "photographyType": "Wedding", "shootsPerMonth": "1-5", "consent": true, "website": ""}
	res1 := doJSON(mux, http.MethodPost, "/api/v1/waitlist", body)
	res2 := doJSON(mux, http.MethodPost, "/api/v1/waitlist", body)
	honeypot := body
	honeypot["website"] = "http://spam"
	res3 := doJSON(mux, http.MethodPost, "/api/v1/waitlist", honeypot)
	for i, r := range []*httptest.ResponseRecorder{res1, res2, res3} {
		if r.Code != 202 {
			t.Fatalf("case %d code=%d body=%s", i, r.Code, r.Body.String())
		}
		if !bytes.Equal(r.Body.Bytes(), res1.Body.Bytes()) {
			t.Fatalf("case %d body differs: %s vs %s", i, r.Body.String(), res1.Body.String())
		}
	}
	var count int
	if err := tdb.Owner.QueryRow(context.Background(), `SELECT COUNT(*) FROM waitlist_entries`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("count=%d", count)
	}
}

func TestWaitlistConsentRequiredAndPhoneNormalization(t *testing.T) {
	tdb := testdb.Open(t)
	svc := waitlist.NewService(store.New(tdb.App), tdb.App)
	h := waitlist.NewHandler(svc)
	mux := http.NewServeMux()
	h.Register(mux)

	res := doJSON(mux, http.MethodPost, "/api/v1/waitlist", map[string]any{"name": "Jane", "contact": "0712345678", "photographyType": "Wedding", "shootsPerMonth": "1-5", "consent": false, "website": ""})
	if res.Code != 400 {
		t.Fatalf("code=%d body=%s", res.Code, res.Body.String())
	}

	res2 := doJSON(mux, http.MethodPost, "/api/v1/waitlist", map[string]any{"name": "Jane", "contact": "0712 345 678", "photographyType": "Wedding", "shootsPerMonth": "1-5", "consent": true, "website": ""})
	if res2.Code != 202 {
		t.Fatalf("code=%d body=%s", res2.Code, res2.Body.String())
	}
	var phone string
	if err := tdb.Owner.QueryRow(context.Background(), `SELECT contact_phone FROM waitlist_entries LIMIT 1`).Scan(&phone); err != nil {
		t.Fatal(err)
	}
	if phone != "254712345678" {
		t.Fatalf("phone=%q", phone)
	}
}

func TestWaitlistRateLimit(t *testing.T) {
	tdb := testdb.Open(t)
	svc := waitlist.NewService(store.New(tdb.App), tdb.App)
	h := waitlist.NewHandler(svc)
	h.SetLimiter(ratelimit.New(2, time.Minute))
	mux := http.NewServeMux()
	h.Register(mux)
	for i := 0; i < 2; i++ {
		r := httptest.NewRequest(http.MethodPost, "/api/v1/waitlist", strings.NewReader(`{"name":"A","contact":"a`+string(rune('a'+i))+`@example.com","photographyType":"W","shootsPerMonth":"1","consent":true,"website":""}`))
		r.Header.Set("Content-Type", "application/json")
		r.RemoteAddr = "1.2.3.4:1234"
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, r)
		if rec.Code != 202 {
			t.Fatalf("i=%d code=%d", i, rec.Code)
		}
	}
	r := httptest.NewRequest(http.MethodPost, "/api/v1/waitlist", strings.NewReader(`{"name":"A","contact":"c@example.com","photographyType":"W","shootsPerMonth":"1","consent":true,"website":""}`))
	r.Header.Set("Content-Type", "application/json")
	r.RemoteAddr = "1.2.3.4:1234"
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, r)
	if rec.Code != 429 {
		t.Fatalf("code=%d", rec.Code)
	}
}

func TestWaitlistDeleteRemovesRow(t *testing.T) {
	tdb := testdb.Open(t)
	svc := waitlist.NewService(store.New(tdb.App), tdb.App)
	_, err := svc.Submit(context.Background(), waitlist.SubmitInput{Name: "A", Contact: "a@example.com", PhotographyType: "W", ShootsPerMonth: "1", Consent: true})
	if err != nil {
		t.Fatal(err)
	}
	var id string
	if err := tdb.Owner.QueryRow(context.Background(), `SELECT id::text FROM waitlist_entries LIMIT 1`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	var uid pgtype.UUID
	if err := uid.Scan(id); err != nil {
		t.Fatal(err)
	}
	if err := svc.Delete(context.Background(), uid, nil); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := tdb.Owner.QueryRow(context.Background(), `SELECT COUNT(*) FROM waitlist_entries`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("count=%d", count)
	}
}

func doJSON(mux *http.ServeMux, method, path string, body any) *httptest.ResponseRecorder {
	b, _ := json.Marshal(body)
	r := httptest.NewRequest(method, path, bytes.NewReader(b))
	r.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, r)
	return rec
}
