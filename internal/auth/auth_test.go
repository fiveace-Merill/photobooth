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
	svc := auth.NewService(store.New(tdb.Owner))
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
	svc := auth.NewService(store.New(tdb.Owner))
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
	if body != nil {
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
