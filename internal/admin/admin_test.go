package admin_test

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/pquerna/otp/totp"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
	"time"

	"github.com/fiveace-Merill/photobooth/internal/admin"
	adminstore "github.com/fiveace-Merill/photobooth/internal/admin/store"
	"github.com/fiveace-Merill/photobooth/internal/auth"
	authstore "github.com/fiveace-Merill/photobooth/internal/auth/store"
	"github.com/fiveace-Merill/photobooth/internal/platform/testdb"
	"github.com/fiveace-Merill/photobooth/internal/waitlist"
	waitliststore "github.com/fiveace-Merill/photobooth/internal/waitlist/store"
	"github.com/jackc/pgx/v5/pgtype"
)

type testEnv struct {
	DB       *testdb.DB
	Mux      *http.ServeMux
	AdminSvc *admin.Service
	Waitlist *waitlist.Service
	AuthSvc  *auth.Service
}

func setup(t *testing.T) (*testdb.DB, *http.ServeMux, *admin.Service) {
	t.Helper()
	env := setupEnv(t)
	return env.DB, env.Mux, env.AdminSvc
}

func setupEnv(t *testing.T) testEnv {
	t.Helper()
	tdb := testdb.Open(t)
	key := bytes.Repeat([]byte{1}, 32)
	adminSvc := admin.NewService(tdb.App, adminstore.New(tdb.App), key, true, 60, 12)
	wlSvc := waitlist.NewService(waitliststore.New(tdb.App), tdb.App)
	wlHandler := waitlist.NewHandler(wlSvc)
	h := admin.NewHandler(adminSvc, wlSvc, nil, "Photobooth", "no-reply@example.com", "http://localhost:3000", false, nil)
	mw := admin.NewMiddleware(adminSvc, false, nil, nil)
	mux := http.NewServeMux()
	h.Register(mux, mw)
	wlHandler.Register(mux)
	// also mount auth for cross-cookie isolation tests
	authSvc := auth.NewService(authstore.New(tdb.App), tdb.App, "open")
	authH := auth.NewHandler(authSvc, false, "open")
	authH.Register(mux, auth.NewMiddleware(authSvc, false))
	return testEnv{DB: tdb, Mux: mux, AdminSvc: adminSvc, Waitlist: wlSvc, AuthSvc: authSvc}
}

func createAdmin(t *testing.T, tdb *testdb.DB, email, password string) pgtype.UUID {
	t.Helper()
	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	var id pgtype.UUID
	err = tdb.Owner.QueryRow(context.Background(), `INSERT INTO admins (email,password_hash,status) VALUES ($1,$2,'active') RETURNING id`, email, hash).Scan(&id)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func doJSON(mux *http.ServeMux, method, path string, body any, cookie *http.Cookie, origin string) *httptest.ResponseRecorder {
	var r *http.Request
	if body != nil {
		b, _ := json.Marshal(body)
		r = httptest.NewRequest(method, path, bytes.NewReader(b))
		r.Header.Set("Content-Type", "application/json")
	} else {
		r = httptest.NewRequest(method, path, nil)
		if method != http.MethodGet {
			r.Header.Set("Content-Type", "application/json")
		}
	}
	if origin != "" {
		r.Header.Set("Origin", origin)
	} else {
		r.Header.Set("Origin", "http://example.com")
	}
	if cookie != nil {
		r.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, r)
	return rec
}

func loginPending(t *testing.T, mux *http.ServeMux, email, password string) *http.Cookie {
	t.Helper()
	res := doJSON(mux, http.MethodPost, "/api/v1/admin/auth/login", map[string]string{"email": email, "password": password}, nil, "http://example.com")
	if res.Code != 200 {
		t.Fatalf("login code=%d body=%s", res.Code, res.Body.String())
	}
	for _, c := range res.Result().Cookies() {
		if c.Name == admin.SessionCookieName {
			return c
		}
	}
	t.Fatal("no cookie")
	return nil
}

func TestAdminPhotographerCookieIsolation(t *testing.T) {
	tdb, mux, _ := setup(t)
	// create photographer session via auth signup
	res := doJSON(mux, http.MethodPost, "/api/v1/auth/signup", map[string]string{"email": "p@example.com", "password": "super-secret-password", "business_name": "Studio", "slug": "studio"}, nil, "http://example.com")
	if res.Code != 201 {
		t.Fatalf("signup=%d %s", res.Code, res.Body.String())
	}
	var pCookie *http.Cookie
	for _, c := range res.Result().Cookies() {
		if c.Name == "photobooth_session" {
			pCookie = c
		}
	}
	if pCookie == nil {
		t.Fatal("no photographer cookie")
	}
	// photographer cookie cannot call admin route
	r := doJSON(mux, http.MethodGet, "/api/v1/admin/overview", nil, pCookie, "http://example.com")
	if r.Code != 401 {
		t.Fatalf("expected 401 got %d %s", r.Code, r.Body.String())
	}

	// admin cookie cannot call photographer route
	createAdmin(t, tdb, "admin@example.com", "super-secret-password")
	adminCookie := loginPending(t, mux, "admin@example.com", "super-secret-password")
	// pending, but even full admin cookie shouldn't work on photographer route
	r2 := doJSON(mux, http.MethodGet, "/api/v1/auth/me", nil, adminCookie, "http://example.com")
	if r2.Code != 401 {
		t.Fatalf("expected 401 got %d %s", r2.Code, r2.Body.String())
	}
}

func confirmTOTP(t *testing.T, tdb *testdb.DB, mux *http.ServeMux, email, password string) (*http.Cookie, []string) {
	t.Helper()
	adminID := getAdminID(t, tdb, email)
	_ = adminID
	cookie := loginPending(t, mux, email, password)
	// start setup
	res := doJSON(mux, http.MethodPost, "/api/v1/admin/auth/totp/setup/start", map[string]any{}, cookie, "http://example.com")
	if res.Code != 200 {
		t.Fatalf("setup start=%d %s", res.Code, res.Body.String())
	}
	var body map[string]any
	_ = json.Unmarshal(res.Body.Bytes(), &body)
	secret, _ := body["secret"].(string)
	if secret == "" {
		t.Fatal("no secret")
	}
	code, err := totpCodeNow(secret)
	if err != nil {
		t.Fatal(err)
	}
	res2 := doJSON(mux, http.MethodPost, "/api/v1/admin/auth/totp/setup/confirm", map[string]any{"code": code}, cookie, "http://example.com")
	if res2.Code != 200 {
		t.Fatalf("confirm=%d %s", res2.Code, res2.Body.String())
	}
	var cbody map[string]any
	_ = json.Unmarshal(res2.Body.Bytes(), &cbody)
	rawCodes, _ := cbody["recoveryCodes"].([]any)
	codes := make([]string, 0, len(rawCodes))
	for _, c := range rawCodes {
		if s, ok := c.(string); ok {
			codes = append(codes, s)
		}
	}
	// cookie updated by server, reuse same cookie value still in jar? response sets cookie
	for _, c := range res2.Result().Cookies() {
		if c.Name == admin.SessionCookieName {
			cookie = c
		}
	}
	return cookie, codes
}

func getAdminID(t *testing.T, tdb *testdb.DB, email string) pgtype.UUID {
	t.Helper()
	var id pgtype.UUID
	if err := tdb.Owner.QueryRow(context.Background(), `SELECT id FROM admins WHERE email=$1`, email).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func totpCodeNow(secret string) (string, error) {
	return totp.GenerateCode(secret, time.Now())
}

func TestAdminPendingSessionCannotReachAdminRoutes(t *testing.T) {
	tdb, mux, _ := setup(t)
	createAdmin(t, tdb, "admin@example.com", "super-secret-password")
	cookie := loginPending(t, mux, "admin@example.com", "super-secret-password")
	res := doJSON(mux, http.MethodGet, "/api/v1/admin/overview", nil, cookie, "http://example.com")
	if res.Code != 401 {
		t.Fatalf("code=%d body=%s", res.Code, res.Body.String())
	}
	_ = tdb
}

func TestAdminOriginRejected(t *testing.T) {
	tdb, mux, _ := setup(t)
	createAdmin(t, tdb, "admin@example.com", "super-secret-password")
	res := doJSON(mux, http.MethodPost, "/api/v1/admin/auth/login", map[string]string{"email": "admin@example.com", "password": "super-secret-password"}, nil, "http://evil.example")
	if res.Code != 403 {
		t.Fatalf("code=%d body=%s", res.Code, res.Body.String())
	}
}

func TestAdminLoginUnknownEmailSameResponse(t *testing.T) {
	tdb, mux, _ := setup(t)
	createAdmin(t, tdb, "admin@example.com", "super-secret-password")
	bad := doJSON(mux, http.MethodPost, "/api/v1/admin/auth/login", map[string]string{"email": "admin@example.com", "password": "wrong-password"}, nil, "http://example.com")
	unknown := doJSON(mux, http.MethodPost, "/api/v1/admin/auth/login", map[string]string{"email": "nobody@example.com", "password": "wrong-password"}, nil, "http://example.com")
	if bad.Code != 401 || unknown.Code != 401 {
		t.Fatalf("codes=%d %d", bad.Code, unknown.Code)
	}
	if !bytes.Equal(bad.Body.Bytes(), unknown.Body.Bytes()) {
		t.Fatalf("bodies differ:\n%s\n%s", bad.Body.String(), unknown.Body.String())
	}
}

func TestAdminTOTPVerifyAndRecoverySingleUse(t *testing.T) {
	tdb, mux, _ := setup(t)
	createAdmin(t, tdb, "admin@example.com", "super-secret-password")
	cookie, codes := confirmTOTP(t, tdb, mux, "admin@example.com", "super-secret-password")
	// cookie now full session
	res := doJSON(mux, http.MethodGet, "/api/v1/admin/overview", nil, cookie, "http://example.com")
	if res.Code != 200 {
		t.Fatalf("overview=%d %s", res.Code, res.Body.String())
	}

	// login again to get pending session for recovery test
	cookie2 := loginPending(t, mux, "admin@example.com", "super-secret-password")
	if len(codes) == 0 {
		t.Fatal("no recovery codes")
	}
	res2 := doJSON(mux, http.MethodPost, "/api/v1/admin/auth/totp/verify", map[string]any{"recoveryCode": codes[0]}, cookie2, "http://example.com")
	if res2.Code != 200 {
		t.Fatalf("recovery=%d %s", res2.Code, res2.Body.String())
	}
	// single use
	cookie3 := loginPending(t, mux, "admin@example.com", "super-secret-password")
	res3 := doJSON(mux, http.MethodPost, "/api/v1/admin/auth/totp/verify", map[string]any{"recoveryCode": codes[0]}, cookie3, "http://example.com")
	if res3.Code != 401 {
		t.Fatalf("reuse=%d %s", res3.Code, res3.Body.String())
	}
}

func TestAdminAuditAppendOnlyAndStateChangeWritesRow(t *testing.T) {
	tdb, mux, _ := setup(t)
	createAdmin(t, tdb, "admin@example.com", "super-secret-password")
	cookie, _ := confirmTOTP(t, tdb, mux, "admin@example.com", "super-secret-password")
	// create waitlist entry
	res := doJSONRawMuxPublic(mux, http.MethodPost, "/api/v1/waitlist", map[string]any{"name": "Jane", "contact": "jane@example.com", "photographyType": "Wedding", "shootsPerMonth": "1-5", "consent": true, "website": ""})
	if res.Code != 202 {
		t.Fatalf("waitlist=%d %s", res.Code, res.Body.String())
	}
	var wid string
	if err := tdb.Owner.QueryRow(context.Background(), `SELECT id::text FROM waitlist_entries LIMIT 1`).Scan(&wid); err != nil {
		t.Fatal(err)
	}
	// decline => state-changing action
	res2 := doJSON(mux, http.MethodPost, "/api/v1/admin/waitlist/"+wid+"/decline", map[string]any{}, cookie, "http://example.com")
	if res2.Code != 200 {
		t.Fatalf("decline=%d %s", res2.Code, res2.Body.String())
	}
	var count int
	if err := tdb.Owner.QueryRow(context.Background(), `SELECT COUNT(*) FROM admin_audit_log WHERE action='waitlist.declined'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("audit count=%d", count)
	}
	// append-only
	_, err := tdb.Owner.Exec(context.Background(), `DELETE FROM admin_audit_log WHERE action='waitlist.declined'`)
	if err == nil {
		t.Fatal("expected error deleting audit log")
	}
}

func doJSONRawMuxPublic(mux *http.ServeMux, method, path string, body any) *httptest.ResponseRecorder {
	b, _ := json.Marshal(body)
	r := httptest.NewRequest(method, path, bytes.NewReader(b))
	r.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, r)
	return rec
}

func TestAdminPhotographerSuspendUnsuspendPlan(t *testing.T) {
	tdb, mux, _ := setup(t)
	createAdmin(t, tdb, "admin@example.com", "super-secret-password")
	cookie, _ := confirmTOTP(t, tdb, mux, "admin@example.com", "super-secret-password")
	// create photographer via auth
	res := doJSON(mux, http.MethodPost, "/api/v1/auth/signup", map[string]string{"email": "p@example.com", "password": "super-secret-password", "business_name": "Studio", "slug": "studio"}, nil, "http://example.com")
	if res.Code != 201 {
		t.Fatalf("signup=%d %s", res.Code, res.Body.String())
	}
	var pCookie *http.Cookie
	for _, c := range res.Result().Cookies() {
		if c.Name == "photobooth_session" {
			pCookie = c
		}
	}
	var pid string
	if err := tdb.Owner.QueryRow(context.Background(), `SELECT id::text FROM photographers WHERE email='p@example.com'`).Scan(&pid); err != nil {
		t.Fatal(err)
	}

	// suspend
	res2 := doJSON(mux, http.MethodPost, "/api/v1/admin/photographers/"+pid+"/suspend", map[string]any{"reason": "abuse"}, cookie, "http://example.com")
	if res2.Code != 200 {
		t.Fatalf("suspend=%d %s", res2.Code, res2.Body.String())
	}
	// session should no longer work
	me := doJSON(mux, http.MethodGet, "/api/v1/auth/me", nil, pCookie, "http://example.com")
	if me.Code != 401 {
		t.Fatalf("me after suspend=%d %s", me.Code, me.Body.String())
	}
	// login blocked (generic error)
	login := doJSON(mux, http.MethodPost, "/api/v1/auth/login", map[string]string{"email": "p@example.com", "password": "super-secret-password"}, nil, "http://example.com")
	if login.Code != 401 {
		t.Fatalf("login=%d %s", login.Code, login.Body.String())
	}
	var code string
	_ = json.Unmarshal(login.Body.Bytes(), &struct {
		Code *string `json:"code"`
	}{Code: &code})
	if code != "invalid_credentials" {
		t.Fatalf("code=%q body=%s", code, login.Body.String())
	}

	// unsuspend
	res3 := doJSON(mux, http.MethodPost, "/api/v1/admin/photographers/"+pid+"/unsuspend", map[string]any{}, cookie, "http://example.com")
	if res3.Code != 200 {
		t.Fatalf("unsuspend=%d %s", res3.Code, res3.Body.String())
	}
	login2 := doJSON(mux, http.MethodPost, "/api/v1/auth/login", map[string]string{"email": "p@example.com", "password": "super-secret-password"}, nil, "http://example.com")
	if login2.Code != 200 {
		t.Fatalf("login2=%d %s", login2.Code, login2.Body.String())
	}

	// plan change
	res4 := doJSON(mux, http.MethodPatch, "/api/v1/admin/photographers/"+pid+"/plan", map[string]any{"planId": "pro"}, cookie, "http://example.com")
	if res4.Code != 200 {
		t.Fatalf("plan=%d %s", res4.Code, res4.Body.String())
	}
	var plan string
	if err := tdb.Owner.QueryRow(context.Background(), `SELECT plan_id FROM photographers WHERE email='p@example.com'`).Scan(&plan); err != nil {
		t.Fatal(err)
	}
	if plan != "pro" {
		t.Fatalf("plan=%q", plan)
	}
}

func TestAdminLoginRateLimit(t *testing.T) {
	tdb, mux, svc := setup(t)
	_ = svc
	createAdmin(t, tdb, "admin@example.com", "super-secret-password")
	for i := 0; i < 5; i++ {
		res := doJSON(mux, http.MethodPost, "/api/v1/admin/auth/login", map[string]string{"email": "admin@example.com", "password": "wrong"}, nil, "http://example.com")
		if res.Code != 401 {
			t.Fatalf("i=%d code=%d body=%s", i, res.Code, res.Body.String())
		}
	}
	res := doJSON(mux, http.MethodPost, "/api/v1/admin/auth/login", map[string]string{"email": "admin@example.com", "password": "wrong"}, nil, "http://example.com")
	if res.Code != 429 {
		t.Fatalf("code=%d body=%s", res.Code, res.Body.String())
	}
}

func TestAdminAllowlistRejectsWith404(t *testing.T) {
	tdb := testdb.Open(t)
	key := bytes.Repeat([]byte{1}, 32)
	adminSvc := admin.NewService(tdb.App, adminstore.New(tdb.App), key, true, 60, 12)
	wlSvc := waitlist.NewService(waitliststore.New(tdb.App), tdb.App)
	h := admin.NewHandler(adminSvc, wlSvc, nil, "Photobooth", "no-reply@example.com", "http://localhost:3000", false, nil)
	mw := admin.NewMiddleware(adminSvc, false, mustParsePrefixes("10.0.0.0/8"), nil)
	mux := http.NewServeMux()
	h.Register(mux, mw)
	createAdmin(t, tdb, "admin@example.com", "super-secret-password")
	r := httptest.NewRequest(http.MethodPost, "/api/v1/admin/auth/login", bytes.NewReader([]byte(`{"email":"admin@example.com","password":"super-secret-password"}`)))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Origin", "http://example.com")
	r.RemoteAddr = "1.2.3.4:1234"
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, r)
	if rec.Code != 404 {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
}

func mustParsePrefixes(v string) []netip.Prefix {
	p, err := netip.ParsePrefix(v)
	if err != nil {
		panic(err)
	}
	return []netip.Prefix{p}
}
