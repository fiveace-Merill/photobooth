package shoots_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fiveace-Merill/photobooth/internal/auth"
	authstore "github.com/fiveace-Merill/photobooth/internal/auth/store"
	"github.com/fiveace-Merill/photobooth/internal/clients"
	"github.com/fiveace-Merill/photobooth/internal/platform/database"
	"github.com/fiveace-Merill/photobooth/internal/platform/testdb"
	"github.com/fiveace-Merill/photobooth/internal/shoots"
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
	authSvc := auth.NewService(authstore.New(tdb.App), true)
	authH := auth.NewHandler(authSvc, false)
	mw := auth.NewMiddleware(authSvc, false)
	clientsSvc := clients.NewService(tdb.App)
	clientsH := clients.NewHandler(clientsSvc)
	shootsSvc, err := shoots.NewService(tdb.App, clientsSvc)
	if err != nil {
		t.Fatal(err)
	}
	// fixed "today" in Nairobi to make overdue deterministic
	shootsSvc = shootsSvc.WithNow(func() time.Time { return time.Date(2026, 11, 26, 9, 0, 0, 0, time.FixedZone("EAT", 3*3600)) })
	shootsH := shoots.NewHandler(shootsSvc)

	mux := http.NewServeMux()
	authH.Register(mux, mw)
	clientsH.Register(mux, auth.RequireAuth)
	shootsH.Register(mux, auth.RequireAuth)
	handler := mw(mux)

	res := doJSON(t, handler, http.MethodPost, "/api/v1/auth/signup", map[string]string{
		"email": email, "password": "super-secret-password", "business_name": biz, "slug": slug,
	}, nil)
	if res.Code != 201 {
		t.Fatalf("signup: %d %s", res.Code, res.Body.String())
	}
	cookie := findCookie(res.Result().Cookies(), auth.SessionCookieName)
	var body map[string]any
	_ = json.Unmarshal(res.Body.Bytes(), &body)
	var pid pgtype.UUID
	_ = pid.Scan(body["id"].(string))

	out := http.NewServeMux()
	out.Handle("/", mw(mux))
	return api{mux: out, cookie: cookie, pid: pid}
}

func createClient(t *testing.T, a api, name, phone string) string {
	t.Helper()
	res := doJSON(t, a.mux, http.MethodPost, "/api/v1/clients", map[string]any{"name": name, "phone": phone}, []*http.Cookie{a.cookie})
	if res.Code != 201 {
		t.Fatalf("create client: %d %s", res.Code, res.Body.String())
	}
	var body map[string]any
	_ = json.Unmarshal(res.Body.Bytes(), &body)
	return body["id"].(string)
}

func createShoot(t *testing.T, a api, payload map[string]any) map[string]any {
	t.Helper()
	res := doJSON(t, a.mux, http.MethodPost, "/api/v1/shoots", payload, []*http.Cookie{a.cookie})
	if res.Code != 201 {
		t.Fatalf("create shoot: %d %s", res.Code, res.Body.String())
	}
	var body map[string]any
	_ = json.Unmarshal(res.Body.Bytes(), &body)
	return body
}

func TestShootsCRUDFiltersSummaryAndIsolation(t *testing.T) {
	tdb, A, B := setup(t)

	clientA := createClient(t, A, "Client A", "0700000111")
	clientB := createClient(t, B, "Client B", "0700000222")

	// B cannot create shoot for A's client
	res := doJSON(t, B.mux, http.MethodPost, "/api/v1/shoots", map[string]any{
		"clientId": clientA, "title": "Bad", "priceMinor": 100, "currency": "KES",
	}, []*http.Cookie{B.cookie})
	if res.Code != 404 {
		t.Fatalf("cross-tenant create: %d %s", res.Code, res.Body.String())
	}

	// A creates shoots
	s1 := createShoot(t, A, map[string]any{"clientId": clientA, "title": "Wedding", "shootDate": "2026-11-20", "dueDate": "2026-11-25", "priceMinor": 300000, "currency": "KES"})
	s2 := createShoot(t, A, map[string]any{"clientId": clientA, "title": "Portrait", "dueDate": "2026-11-26", "priceMinor": 0, "currency": "KES"})
	s3 := createShoot(t, A, map[string]any{"clientId": clientA, "title": "Future", "dueDate": "2026-11-30", "priceMinor": 100, "currency": "KES"})
	_ = createShoot(t, B, map[string]any{"clientId": clientB, "title": "B Shoot", "dueDate": "2026-11-20", "priceMinor": 100, "currency": "KES"})

	if s1["overdue"] != true || s1["daysOverdue"].(float64) != 1 {
		t.Fatalf("s1 overdue=%v days=%v", s1["overdue"], s1["daysOverdue"])
	}
	if s2["overdue"] != false {
		t.Fatalf("due today should not be overdue: %v", s2["overdue"])
	}

	// status rules
	res = doJSON(t, A.mux, http.MethodPost, "/api/v1/shoots/"+s3["id"].(string)+"/status", map[string]any{"status": "delivered"}, []*http.Cookie{A.cookie})
	if res.Code != 422 {
		t.Fatalf("delivered transition: %d %s", res.Code, res.Body.String())
	}
	res = doJSON(t, A.mux, http.MethodPost, "/api/v1/shoots/"+s3["id"].(string)+"/status", map[string]any{"status": "editing"}, []*http.Cookie{A.cookie})
	if res.Code != 200 {
		t.Fatalf("status editing: %d %s", res.Code, res.Body.String())
	}

	// list filters/sort/paging
	res = doJSON(t, A.mux, http.MethodGet, "/api/v1/shoots?status=shot,editing&overdue=true&sort=due&limit=25&offset=0", nil, []*http.Cookie{A.cookie})
	if res.Code != 200 {
		t.Fatalf("list: %d %s", res.Code, res.Body.String())
	}
	var list map[string]any
	_ = json.Unmarshal(res.Body.Bytes(), &list)
	if list["total"].(float64) != 1 {
		t.Fatalf("overdue total=%v body=%s", list["total"], res.Body.String())
	}
	items := list["items"].([]any)
	first := items[0].(map[string]any)
	if first["clientName"] != "Client A" {
		t.Fatalf("clientName=%v", first["clientName"])
	}

	// B cannot read/update/delete/list A shoot
	shootAID := s1["id"].(string)
	res = doJSON(t, B.mux, http.MethodGet, "/api/v1/shoots/"+shootAID, nil, []*http.Cookie{B.cookie})
	if res.Code != 404 {
		t.Fatalf("B get: %d", res.Code)
	}
	res = doJSON(t, B.mux, http.MethodPatch, "/api/v1/shoots/"+shootAID, map[string]any{"title": "Hacked"}, []*http.Cookie{B.cookie})
	if res.Code != 404 {
		t.Fatalf("B patch: %d", res.Code)
	}
	res = doJSON(t, B.mux, http.MethodDelete, "/api/v1/shoots/"+shootAID, nil, []*http.Cookie{B.cookie})
	if res.Code != 404 {
		t.Fatalf("B delete: %d", res.Code)
	}
	res = doJSON(t, B.mux, http.MethodGet, "/api/v1/shoots", nil, []*http.Cookie{B.cookie})
	_ = json.Unmarshal(res.Body.Bytes(), &list)
	if list["total"].(float64) != 1 {
		t.Fatalf("B total should be own only: %v", list["total"])
	}

	// summary excludes other tenant
	res = doJSON(t, A.mux, http.MethodGet, "/api/v1/shoots/summary", nil, []*http.Cookie{A.cookie})
	var sum map[string]any
	_ = json.Unmarshal(res.Body.Bytes(), &sum)
	counts := sum["counts"].(map[string]any)
	if counts["shot"].(float64) != 2 || counts["editing"].(float64) != 1 {
		t.Fatalf("counts=%v", counts)
	}
	if sum["overdue"].(float64) != 1 {
		t.Fatalf("summary overdue=%v", sum["overdue"])
	}

	// delete delivered shoot rejected
	// set delivered directly in DB as system code
	_ = tdb
	var shootUUID pgtype.UUID
	_ = shootUUID.Scan(s2["id"].(string))
	err := database.WithTenant(context.Background(), tdb.App, A.pid, func(tx pgx.Tx) error {
		_, err := tx.Exec(context.Background(), `UPDATE shoots SET status='delivered', updated_at=now() WHERE photographer_id=$1 AND id=$2`, A.pid, shootUUID)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	res = doJSON(t, A.mux, http.MethodDelete, "/api/v1/shoots/"+s2["id"].(string), nil, []*http.Cookie{A.cookie})
	if res.Code != 409 {
		t.Fatalf("delete delivered: %d %s", res.Code, res.Body.String())
	}

	// delete allowed status
	res = doJSON(t, A.mux, http.MethodDelete, "/api/v1/shoots/"+s3["id"].(string), nil, []*http.Cookie{A.cookie})
	if res.Code != 204 {
		t.Fatalf("delete editing: %d %s", res.Code, res.Body.String())
	}
}

func TestShootsNoTenantContextZeroRows(t *testing.T) {
	tdb := testdb.Open(t)
	ctx := context.Background()
	var pid, cid pgtype.UUID
	if err := tdb.Owner.QueryRow(ctx, `INSERT INTO photographers (email,password_hash,business_name,slug) VALUES ('s@example.com','h','S','studio-s') RETURNING id`).Scan(&pid); err != nil {
		t.Fatal(err)
	}
	if err := database.WithTenant(ctx, tdb.App, pid, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `INSERT INTO clients (photographer_id,name,phone) VALUES ($1,'C','254700000099') RETURNING id`, pid).Scan(&cid)
	}); err != nil {
		t.Fatal(err)
	}
	if err := database.WithTenant(ctx, tdb.App, pid, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO shoots (photographer_id,client_id,title,status,price_minor,currency) VALUES ($1,$2,'T','shot',0,'KES')`, pid, cid)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	err := database.WithTenant(ctx, tdb.App, pgtype.UUID{}, func(tx pgx.Tx) error {
		var n int
		if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM shoots WHERE photographer_id=$1`, pid).Scan(&n); err != nil {
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
