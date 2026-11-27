package clients_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fiveace-Merill/photobooth/internal/auth"
	authstore "github.com/fiveace-Merill/photobooth/internal/auth/store"
	"github.com/fiveace-Merill/photobooth/internal/clients"
	"github.com/fiveace-Merill/photobooth/internal/platform/database"
	"github.com/fiveace-Merill/photobooth/internal/platform/testdb"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type api struct {
	mux    *http.ServeMux
	cookie *http.Cookie
	pid    pgtype.UUID
}

func setup(t *testing.T) (*testdb.DB, api, api) {
	t.Helper()
	tdb := testdb.Open(t)
	a := newPhotographer(t, tdb, "a@example.com", "A Studio", "studio-a")
	b := newPhotographer(t, tdb, "b@example.com", "B Studio", "studio-b")
	return tdb, a, b
}

func newPhotographer(t *testing.T, tdb *testdb.DB, email, biz, slug string) api {
	t.Helper()
	svc := auth.NewService(authstore.New(tdb.App), tdb.App, "open")
	h := auth.NewHandler(svc, false, "open")
	mw := auth.NewMiddleware(svc, false)
	clientsSvc := clients.NewService(tdb.App)
	clientsH := clients.NewHandler(clientsSvc)
	mux := http.NewServeMux()
	h.Register(mux, mw)
	clientsH.Register(mux, auth.RequireAuth)
	handler := mw(mux)
	// signup via handler
	res := doJSON(t, handler, http.MethodPost, "/api/v1/auth/signup", map[string]string{
		"email": email, "password": "super-secret-password", "business_name": biz, "slug": slug,
	}, nil)
	if res.Code != 201 {
		t.Fatalf("signup: %d %s", res.Code, res.Body.String())
	}
	c := findCookie(res.Result().Cookies(), auth.SessionCookieName)
	var body map[string]any
	_ = json.Unmarshal(res.Body.Bytes(), &body)
	var pid pgtype.UUID
	_ = pid.Scan(body["id"].(string))
	return api{mux: wrap(mux, mw), cookie: c, pid: pid}
}

func wrap(mux *http.ServeMux, mw func(http.Handler) http.Handler) *http.ServeMux {
	// return mux wrapped? easier to keep handler
	out := http.NewServeMux()
	out.Handle("/", mw(mux))
	return out
}

func TestClientsCRUDAndIsolation(t *testing.T) {
	tdb, A, B := setup(t)
	_ = tdb

	// A creates client
	res := doJSON(t, A.mux, http.MethodPost, "/api/v1/clients", map[string]any{
		"name": "Jane Client", "phone": "0712 345 678", "email": "jane@example.com", "notes": "VIP",
	}, []*http.Cookie{A.cookie})
	if res.Code != 201 {
		t.Fatalf("create: %d %s", res.Code, res.Body.String())
	}
	var created map[string]any
	_ = json.Unmarshal(res.Body.Bytes(), &created)
	id := created["id"].(string)
	if created["phone"].(string) != "254712345678" {
		t.Fatalf("phone not normalized: %v", created["phone"])
	}

	// duplicate phone
	res = doJSON(t, A.mux, http.MethodPost, "/api/v1/clients", map[string]any{"name": "Other", "phone": "+254712345678"}, []*http.Cookie{A.cookie})
	if res.Code != 409 {
		t.Fatalf("dup: %d %s", res.Code, res.Body.String())
	}
	var dup map[string]any
	_ = json.Unmarshal(res.Body.Bytes(), &dup)
	if dup["code"] != "client_phone_exists" {
		t.Fatalf("code=%v", dup["code"])
	}
	if dup["existingClientId"] != id {
		t.Fatalf("existingClientId=%v", dup["existingClientId"])
	}

	// B cannot read/list/update/delete A's client
	res = doJSON(t, B.mux, http.MethodGet, "/api/v1/clients/"+id, nil, []*http.Cookie{B.cookie})
	if res.Code != 404 {
		t.Fatalf("B get: %d %s", res.Code, res.Body.String())
	}
	res = doJSON(t, B.mux, http.MethodPatch, "/api/v1/clients/"+id, map[string]any{"name": "Hacked"}, []*http.Cookie{B.cookie})
	if res.Code != 404 {
		t.Fatalf("B patch: %d %s", res.Code, res.Body.String())
	}
	res = doJSON(t, B.mux, http.MethodDelete, "/api/v1/clients/"+id, nil, []*http.Cookie{B.cookie})
	if res.Code != 404 {
		t.Fatalf("B delete: %d %s", res.Code, res.Body.String())
	}
	res = doJSON(t, B.mux, http.MethodGet, "/api/v1/clients", nil, []*http.Cookie{B.cookie})
	var list map[string]any
	_ = json.Unmarshal(res.Body.Bytes(), &list)
	if list["total"].(float64) != 0 {
		t.Fatalf("B list total=%v", list["total"])
	}

	// list with q
	res = doJSON(t, A.mux, http.MethodGet, "/api/v1/clients?q=jane", nil, []*http.Cookie{A.cookie})
	_ = json.Unmarshal(res.Body.Bytes(), &list)
	if list["total"].(float64) != 1 {
		t.Fatalf("q total=%v body=%s", list["total"], res.Body.String())
	}
}

func TestNoTenantContextReturnsZeroRows(t *testing.T) {
	tdb := testdb.Open(t)
	ctx := context.Background()
	var pid pgtype.UUID
	err := tdb.Owner.QueryRow(ctx, `INSERT INTO photographers (email,password_hash,business_name,slug) VALUES ('n@example.com','h','N','studio-n') RETURNING id`).Scan(&pid)
	if err != nil {
		t.Fatal(err)
	}
	err = database.WithTenant(ctx, tdb.App, pid, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO clients (photographer_id,name,phone) VALUES ($1,'C','254700000001')`, pid)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	err = database.WithTenant(ctx, tdb.App, pgtype.UUID{}, func(tx pgx.Tx) error {
		var n int
		if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM clients WHERE photographer_id=$1`, pid).Scan(&n); err != nil {
			return err
		}
		if n != 0 {
			t.Fatalf("got %d", n)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func doJSON(t *testing.T, h http.Handler, method, path string, body any, cookies []*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	if body != nil || method == http.MethodPost || method == http.MethodPatch || method == http.MethodDelete {
		req.Header.Set("Content-Type", "application/json")
	}
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}
func findCookie(cookies []*http.Cookie, name string) *http.Cookie {
	for _, c := range cookies {
		if c.Name == name {
			return c
		}
	}
	return nil
}
