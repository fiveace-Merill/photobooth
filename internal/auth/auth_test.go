package auth_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fiveace-Merill/photobooth/internal/auth"
	"github.com/fiveace-Merill/photobooth/internal/auth/store"
	"github.com/fiveace-Merill/photobooth/internal/platform/testdb"
)

func TestSignupLoginLogout(t *testing.T) {
	tdb := testdb.Open(t)
	svc := auth.NewService(store.New(tdb.Owner), true)
	h := auth.NewHandler(svc, false)

	mux := http.NewServeMux()
	h.Register(mux, auth.NewMiddleware(svc, false))

	// Signup happy path.
	signupBody := map[string]string{
		"email":         "photog@example.com",
		"password":      "super-secret-password",
		"business_name": "Studio One",
		"slug":          "studio-one",
	}
	res := doJSON(t, mux, http.MethodPost, "/api/v1/auth/signup", signupBody, nil)
	if res.Code != http.StatusCreated {
		t.Fatalf("signup status = %d, want %d; body=%s", res.Code, http.StatusCreated, res.Body.String())
	}
	var signupResp map[string]any
	if err := json.Unmarshal(res.Body.Bytes(), &signupResp); err != nil {
		t.Fatalf("unmarshal signup response: %v", err)
	}
	if signupResp["email"] != "photog@example.com" {
		t.Fatalf("signup email = %v", signupResp["email"])
	}
	cookies := res.Result().Cookies()
	sessionCookie := findCookie(cookies, auth.SessionCookieName)
	if sessionCookie == nil || sessionCookie.Value == "" {
		t.Fatal("expected session cookie on signup")
	}

	// Duplicate email should fail.
	res = doJSON(t, mux, http.MethodPost, "/api/v1/auth/signup", signupBody, nil)
	if res.Code != http.StatusConflict {
		t.Fatalf("duplicate signup status = %d, want %d; body=%s", res.Code, http.StatusConflict, res.Body.String())
	}

	// Login happy path.
	loginBody := map[string]string{
		"email":    "photog@example.com",
		"password": "super-secret-password",
	}
	res = doJSON(t, mux, http.MethodPost, "/api/v1/auth/login", loginBody, nil)
	if res.Code != http.StatusOK {
		t.Fatalf("login status = %d, want %d; body=%s", res.Code, http.StatusOK, res.Body.String())
	}
	loginCookie := findCookie(res.Result().Cookies(), auth.SessionCookieName)
	if loginCookie == nil || loginCookie.Value == "" {
		t.Fatal("expected session cookie on login")
	}

	// Wrong password.
	res = doJSON(t, mux, http.MethodPost, "/api/v1/auth/login", map[string]string{
		"email":    "photog@example.com",
		"password": "wrong-password",
	}, nil)
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("wrong password status = %d, want %d; body=%s", res.Code, http.StatusUnauthorized, res.Body.String())
	}

	// Logout clears session.
	res = doJSON(t, mux, http.MethodPost, "/api/v1/auth/logout", nil, []*http.Cookie{loginCookie})
	if res.Code != http.StatusNoContent {
		t.Fatalf("logout status = %d, want %d; body=%s", res.Code, http.StatusNoContent, res.Body.String())
	}
	cleared := findCookie(res.Result().Cookies(), auth.SessionCookieName)
	if cleared == nil {
		t.Fatal("expected cleared session cookie on logout")
	}
	if cleared.MaxAge >= 0 {
		t.Fatalf("expected cleared cookie MaxAge < 0, got %d", cleared.MaxAge)
	}

	// Session should no longer be valid in DB.
	var count int
	err := tdb.Owner.QueryRow(context.Background(), "SELECT COUNT(*) FROM sessions").Scan(&count)
	if err != nil {
		t.Fatalf("count sessions: %v", err)
	}
	// Signup created one session, login created another; logout deleted the one
	// presented in the request cookie. One session should remain.
	if count != 1 {
		t.Fatalf("sessions count = %d, want 1", count)
	}
}

func TestMe(t *testing.T) {
	tdb := testdb.Open(t)
	svc := auth.NewService(store.New(tdb.Owner), true)
	h := auth.NewHandler(svc, false)

	mux := http.NewServeMux()
	mw := auth.NewMiddleware(svc, false)
	h.Register(mux, mw)
	handler := mw(mux)

	// Unauthenticated request should be rejected.
	res := doJSON(t, handler, http.MethodGet, "/api/v1/auth/me", nil, nil)
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated /me status = %d, want %d; body=%s", res.Code, http.StatusUnauthorized, res.Body.String())
	}

	// Signup to get a valid session cookie.
	res = doJSON(t, handler, http.MethodPost, "/api/v1/auth/signup", map[string]string{
		"email":         "me@example.com",
		"password":      "super-secret-password",
		"business_name": "Studio Me",
		"slug":          "studio-me",
	}, nil)
	if res.Code != http.StatusCreated {
		t.Fatalf("signup status = %d, want %d; body=%s", res.Code, http.StatusCreated, res.Body.String())
	}
	sessionCookie := findCookie(res.Result().Cookies(), auth.SessionCookieName)
	if sessionCookie == nil || sessionCookie.Value == "" {
		t.Fatal("expected session cookie on signup")
	}

	// Authenticated request should return current photographer.
	res = doJSON(t, handler, http.MethodGet, "/api/v1/auth/me", nil, []*http.Cookie{sessionCookie})
	if res.Code != http.StatusOK {
		t.Fatalf("authenticated /me status = %d, want %d; body=%s", res.Code, http.StatusOK, res.Body.String())
	}
	var me map[string]any
	if err := json.Unmarshal(res.Body.Bytes(), &me); err != nil {
		t.Fatalf("unmarshal /me response: %v", err)
	}
	if me["email"] != "me@example.com" {
		t.Fatalf("me email = %v, want me@example.com", me["email"])
	}
	if me["business_name"] != "Studio Me" {
		t.Fatalf("me business_name = %v, want Studio Me", me["business_name"])
	}
	if me["plan_id"] != "free" {
		t.Fatalf("me plan_id = %v, want free", me["plan_id"])
	}
}

func doJSON(t *testing.T, h http.Handler, method, path string, body any, cookies []*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode body: %v", err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	if body != nil || method == http.MethodPost || method == http.MethodPut || method == http.MethodPatch || method == http.MethodDelete {
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

func TestSignupClosed(t *testing.T) {
	tdb := testdb.Open(t)
	svc := auth.NewService(store.New(tdb.Owner), false)
	h := auth.NewHandler(svc, false)
	mux := http.NewServeMux()
	h.Register(mux, auth.NewMiddleware(svc, false))

	res := doJSON(t, mux, http.MethodPost, "/api/v1/auth/signup", map[string]string{
		"email":         "closed@example.com",
		"password":      "super-secret-password",
		"business_name": "Studio",
		"slug":          "studio",
	}, nil)
	if res.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%s", res.Code, http.StatusForbidden, res.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body["code"] != "signup_closed" {
		t.Fatalf("code = %v, want signup_closed", body["code"])
	}
}

func TestCookieSecureFlag(t *testing.T) {
	tdb := testdb.Open(t)
	svc := auth.NewService(store.New(tdb.Owner), true)
	h := auth.NewHandler(svc, true)
	mux := http.NewServeMux()
	h.Register(mux, auth.NewMiddleware(svc, true))

	res := doJSON(t, mux, http.MethodPost, "/api/v1/auth/signup", map[string]string{
		"email":         "secure@example.com",
		"password":      "super-secret-password",
		"business_name": "Studio Secure",
		"slug":          "studio-secure",
	}, nil)
	if res.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", res.Code, http.StatusCreated, res.Body.String())
	}
	c := findCookie(res.Result().Cookies(), auth.SessionCookieName)
	if c == nil || !c.Secure || !c.HttpOnly || c.SameSite != http.SameSiteLaxMode {
		t.Fatalf("unexpected cookie: %+v", c)
	}
	res = doJSON(t, mux, http.MethodPost, "/api/v1/auth/logout", nil, []*http.Cookie{c})
	if res.Code != http.StatusNoContent {
		t.Fatalf("logout status = %d, want %d; body=%s", res.Code, http.StatusNoContent, res.Body.String())
	}
	cleared := findCookie(res.Result().Cookies(), auth.SessionCookieName)
	if cleared == nil || !cleared.Secure {
		t.Fatalf("expected cleared secure cookie, got %+v", cleared)
	}
}

func TestCSRFMiddleware(t *testing.T) {
	tdb := testdb.Open(t)
	svc := auth.NewService(store.New(tdb.Owner), true)
	h := auth.NewHandler(svc, false)
	mux := http.NewServeMux()
	h.Register(mux, auth.NewMiddleware(svc, false))

	body := map[string]string{
		"email": "csrf@example.com", "password": "super-secret-password", "business_name": "Studio", "slug": "studio-csrf",
	}
	// Wrong content type.
	buf := bytes.NewBufferString(`{"email":"a@b.com"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", buf)
	req.Header.Set("Content-Type", "text/plain")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("content-type status = %d, want %d", rec.Code, http.StatusUnsupportedMediaType)
	}

	// Cross-site origin.
	res := doJSONWithOrigin(t, mux, http.MethodPost, "/api/v1/auth/signup", body, nil, "https://evil.example")
	if res.Code != http.StatusForbidden {
		t.Fatalf("origin status = %d, want %d; body=%s", res.Code, http.StatusForbidden, res.Body.String())
	}
}

func doJSONWithOrigin(t *testing.T, h http.Handler, method, path string, body any, cookies []*http.Cookie, origin string) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode body: %v", err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}
