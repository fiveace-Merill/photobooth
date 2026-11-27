package auth_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fiveace-Merill/photobooth/internal/platform/ratelimit"

	"github.com/fiveace-Merill/photobooth/internal/auth"
	"github.com/fiveace-Merill/photobooth/internal/auth/store"
	"github.com/fiveace-Merill/photobooth/internal/platform/testdb"
	"github.com/jackc/pgx/v5/pgtype"
)

type bufHandler struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (h *bufHandler) Enabled(context.Context, slog.Level) bool { return true }
func (h *bufHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.b.WriteString(r.Message)
	r.Attrs(func(a slog.Attr) bool {
		h.b.WriteString(" ")
		h.b.WriteString(a.Key)
		h.b.WriteString("=")
		h.b.WriteString(a.Value.String())
		return true
	})
	h.b.WriteByte('\n')
	return nil
}
func (h *bufHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *bufHandler) WithGroup(string) slog.Handler      { return h }
func (h *bufHandler) String() string                     { h.mu.Lock(); defer h.mu.Unlock(); return h.b.String() }

func TestInvitationValidateSignupFlow(t *testing.T) {
	tdb := testdb.Open(t)
	svc := auth.NewService(store.New(tdb.App), tdb.App, "invite")
	h := auth.NewHandler(svc, false, "invite")
	mux := http.NewServeMux()
	h.Register(mux, auth.NewMiddleware(svc, false))

	create, err := svc.CreateInvitation(context.Background(), "Invitee@Example.com", "pro", 24*time.Hour, "note", pgtype.UUID{})
	if err != nil {
		t.Fatalf("CreateInvitation: %v", err)
	}
	if create.RawToken == "" {
		t.Fatal("expected raw token")
	}

	// raw token must not be stored
	var tokenCount int
	if err := tdb.Owner.QueryRow(context.Background(), `SELECT COUNT(*) FROM invitations WHERE encode(token_hash,'hex') LIKE $1 OR token_hash::text LIKE $2`, "%"+create.RawToken+"%", "%"+create.RawToken+"%").Scan(&tokenCount); err != nil {
		t.Fatalf("query token leak: %v", err)
	}
	if tokenCount != 0 {
		t.Fatal("raw token stored in invitations")
	}

	// validate endpoint
	res := doJSON(t, mux, http.MethodGet, "/api/v1/auth/invitations/"+create.RawToken, nil, nil)
	if res.Code != http.StatusOK {
		t.Fatalf("validate status=%d body=%s", res.Code, res.Body.String())
	}
	if cc := res.Header().Get("Cache-Control"); cc != "no-store" {
		t.Fatalf("cache-control=%q", cc)
	}
	if rp := res.Header().Get("Referrer-Policy"); rp != "no-referrer" {
		t.Fatalf("referrer-policy=%q", rp)
	}
	var invResp map[string]any
	if err := json.Unmarshal(res.Body.Bytes(), &invResp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if invResp["email"] != "invitee@example.com" {
		t.Fatalf("email=%v", invResp["email"])
	}

	// signup with invitation
	signupRes := doJSON(t, mux, http.MethodPost, "/api/v1/auth/signup", map[string]string{
		"email":         "Invitee@Example.com",
		"password":      "super-secret-password",
		"business_name": "Invited Studio",
		"slug":          "invited-studio",
		"inviteToken":   create.RawToken,
	}, nil)
	if signupRes.Code != http.StatusCreated {
		t.Fatalf("signup status=%d body=%s", signupRes.Code, signupRes.Body.String())
	}
	var sbody map[string]any
	if err := json.Unmarshal(signupRes.Body.Bytes(), &sbody); err != nil {
		t.Fatalf("unmarshal signup: %v", err)
	}
	if sbody["plan_id"] != "pro" {
		t.Fatalf("plan_id=%v want pro", sbody["plan_id"])
	}

	// used token cannot be reused
	res2 := doJSON(t, mux, http.MethodGet, "/api/v1/auth/invitations/"+create.RawToken, nil, nil)
	signup2 := doJSON(t, mux, http.MethodPost, "/api/v1/auth/signup", map[string]string{
		"email":         "Invitee@Example.com",
		"password":      "super-secret-password",
		"business_name": "Invited Studio 2",
		"slug":          "invited-studio-2",
		"inviteToken":   create.RawToken,
	}, nil)
	if res2.Code != http.StatusNotFound || signup2.Code != http.StatusNotFound {
		t.Fatalf("reuse statuses get=%d signup=%d bodies=%s %s", res2.Code, signup2.Code, res2.Body.String(), signup2.Body.String())
	}
	if !bytes.Equal(res2.Body.Bytes(), signup2.Body.Bytes()) {
		t.Fatalf("expected identical bodies for invalid used cases:\nget=%s\nsignup=%s", res2.Body.Bytes(), signup2.Body.Bytes())
	}
}

func TestInvitationInvalidCasesUniform(t *testing.T) {
	tdb := testdb.Open(t)
	svc := auth.NewService(store.New(tdb.App), tdb.App, "invite")
	h := auth.NewHandler(svc, false, "invite")
	mux := http.NewServeMux()
	h.Register(mux, auth.NewMiddleware(svc, false))

	mkSignup := func(token string) *httptest.ResponseRecorder {
		return doJSON(t, mux, http.MethodPost, "/api/v1/auth/signup", map[string]string{
			"email": "u@example.com", "password": "super-secret-password", "business_name": "Studio", "slug": "studio-u", "inviteToken": token,
		}, nil)
	}
	mkGet := func(token string) *httptest.ResponseRecorder {
		return doJSON(t, mux, http.MethodGet, "/api/v1/auth/invitations/"+token, nil, nil)
	}

	// mismatch: invitation email differs
	mismatch, err := svc.CreateInvitation(context.Background(), "real@example.com", "free", 24*time.Hour, "", pgtype.UUID{})
	if err != nil {
		t.Fatal(err)
	}
	mismatchSignup := doJSON(t, mux, http.MethodPost, "/api/v1/auth/signup", map[string]string{
		"email": "other@example.com", "password": "super-secret-password", "business_name": "Studio", "slug": "studio-m", "inviteToken": mismatch.RawToken,
	}, nil)

	expired, err := svc.CreateInvitation(context.Background(), "expired@example.com", "free", time.Minute, "", pgtype.UUID{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tdb.Owner.Exec(context.Background(), `UPDATE invitations SET expires_at = now() - interval '1 minute' WHERE token_hash = $1`, hashForTest(expired.RawToken)); err != nil {
		t.Fatal(err)
	}

	revoked, err := svc.CreateInvitation(context.Background(), "revoked@example.com", "free", 24*time.Hour, "", pgtype.UUID{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.RevokeInvitation(context.Background(), revoked.Invitation.ID); err != nil {
		t.Fatal(err)
	}

	used, err := svc.CreateInvitation(context.Background(), "used@example.com", "free", 24*time.Hour, "", pgtype.UUID{})
	if err != nil {
		t.Fatal(err)
	}
	usedSignup := doJSON(t, mux, http.MethodPost, "/api/v1/auth/signup", map[string]string{
		"email": "used@example.com", "password": "super-secret-password", "business_name": "Studio", "slug": "studio-used", "inviteToken": used.RawToken,
	}, nil)
	if usedSignup.Code != 201 {
		t.Fatalf("used setup signup=%d %s", usedSignup.Code, usedSignup.Body.String())
	}

	cases := []*httptest.ResponseRecorder{
		mkGet("bogus-token"),
		mkSignup("bogus-token"),
		mkGet(mismatch.RawToken + "_nope"),
		mismatchSignup,
		mkGet(expired.RawToken),
		mkSignup(expired.RawToken),
		mkGet(revoked.RawToken),
		mkSignup(revoked.RawToken),
		mkGet(used.RawToken),
		mkSignup(used.RawToken),
	}
	want := cases[0].Body.Bytes()
	for i, res := range cases {
		if res.Code != http.StatusNotFound {
			t.Fatalf("case %d status=%d body=%s", i, res.Code, res.Body.String())
		}
		if !bytes.Equal(res.Body.Bytes(), want) {
			t.Fatalf("case %d body differs:\nwant=%s\ngot=%s", i, want, res.Body.Bytes())
		}
		if cc := res.Header().Get("Cache-Control"); cc != "no-store" {
			t.Fatalf("case %d cache-control=%q", i, cc)
		}
		if rp := res.Header().Get("Referrer-Policy"); rp != "no-referrer" {
			t.Fatalf("case %d referrer-policy=%q", i, rp)
		}
	}
}

func hashForTest(raw string) []byte { h := sha256Sum(raw); return h[:] }

func sha256Sum(raw string) [32]byte { return sha256.Sum256([]byte(raw)) }

func TestInvitationConcurrentSignupSingleWinner(t *testing.T) {
	tdb := testdb.Open(t)
	svc := auth.NewService(store.New(tdb.App), tdb.App, "invite")
	h := auth.NewHandler(svc, false, "invite")
	mux := http.NewServeMux()
	h.Register(mux, auth.NewMiddleware(svc, false))

	inv, err := svc.CreateInvitation(context.Background(), "race@example.com", "pro", 24*time.Hour, "", pgtype.UUID{})
	if err != nil {
		t.Fatal(err)
	}

	type res struct {
		code int
		body []byte
	}
	ch := make(chan res, 2)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			rec := doJSON(t, mux, http.MethodPost, "/api/v1/auth/signup", map[string]string{
				"email": "race@example.com", "password": "super-secret-password", "business_name": "Studio", "slug": "race-studio-" + string(rune('a'+i)), "inviteToken": inv.RawToken,
			}, nil)
			ch <- res{code: rec.Code, body: append([]byte(nil), rec.Body.Bytes()...)}
		}()
	}
	close(start)
	wg.Wait()
	close(ch)
	var codes []int
	var bodies [][]byte
	for r := range ch {
		codes = append(codes, r.code)
		bodies = append(bodies, r.body)
	}
	if len(codes) != 2 {
		t.Fatalf("codes=%v", codes)
	}
	var okCount, badCount int
	for i, c := range codes {
		if c == http.StatusCreated {
			okCount++
		}
		if c == http.StatusNotFound {
			badCount++
			if !bytes.Equal(bodies[i], bodies[0]) && !bytes.Equal(bodies[i], bodies[1]) {
				// compare against known uniform body by reusing one of the not found ones later
			}
		}
	}
	if okCount != 1 || badCount != 1 {
		t.Fatalf("codes=%v bodies=%q %q", codes, bodies[0], bodies[1])
	}
	// loser body must equal uniform invalid body
	var loser []byte
	for i, c := range codes {
		if c == http.StatusNotFound {
			loser = bodies[i]
		}
	}
	uniform := doJSON(t, mux, http.MethodGet, "/api/v1/auth/invitations/bogus-token", nil, nil).Body.Bytes()
	if !bytes.Equal(loser, uniform) {
		t.Fatalf("loser body not uniform:\n%s\n%s", loser, uniform)
	}
	var usedCount int
	if err := tdb.Owner.QueryRow(context.Background(), `SELECT COUNT(*) FROM invitations WHERE used_at IS NOT NULL`).Scan(&usedCount); err != nil {
		t.Fatal(err)
	}
	if usedCount != 1 {
		t.Fatalf("usedCount=%d", usedCount)
	}
	var photogCount int
	if err := tdb.Owner.QueryRow(context.Background(), `SELECT COUNT(*) FROM photographers WHERE email='race@example.com'`).Scan(&photogCount); err != nil {
		t.Fatal(err)
	}
	if photogCount != 1 {
		t.Fatalf("photogCount=%d", photogCount)
	}
}

func TestInvitationRateLimitAndConfig(t *testing.T) {
	tdb := testdb.Open(t)
	svc := auth.NewService(store.New(tdb.App), tdb.App, "invite")
	h := auth.NewHandler(svc, false, "invite")
	h.SetInviteLimiter(ratelimit.New(2, time.Minute))
	mux := http.NewServeMux()
	h.Register(mux, auth.NewMiddleware(svc, false))

	res := doJSON(t, mux, http.MethodGet, "/api/v1/auth/config", nil, nil)
	if res.Code != 200 {
		t.Fatalf("config code=%d body=%s", res.Code, res.Body.String())
	}
	var cfg map[string]string
	if err := json.Unmarshal(res.Body.Bytes(), &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg["signupMode"] != "invite" {
		t.Fatalf("cfg=%v", cfg)
	}

	for i := 0; i < 2; i++ {
		r := httptest.NewRequest(http.MethodGet, "/api/v1/auth/invitations/bogus", nil)
		r.RemoteAddr = "1.2.3.4:1234"
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, r)
		if rec.Code != 404 {
			t.Fatalf("i=%d code=%d", i, rec.Code)
		}
	}
	r := httptest.NewRequest(http.MethodGet, "/api/v1/auth/invitations/bogus", nil)
	r.RemoteAddr = "1.2.3.4:1234"
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, r)
	if rec.Code != 429 {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Referrer-Policy"); got != "no-referrer" {
		t.Fatalf("rp=%q", got)
	}
}

func TestInvitationRawTokenNotLogged(t *testing.T) {
	tdb := testdb.Open(t)
	logBuf := &bufHandler{}
	old := slog.Default()
	slog.SetDefault(slog.New(logBuf))
	t.Cleanup(func() { slog.SetDefault(old) })

	svc := auth.NewService(store.New(tdb.App), tdb.App, "invite")
	h := auth.NewHandler(svc, false, "invite")
	mux := http.NewServeMux()
	h.Register(mux, auth.NewMiddleware(svc, false))

	inv, err := svc.CreateInvitation(context.Background(), "nolog@example.com", "free", 24*time.Hour, "", pgtype.UUID{})
	if err != nil {
		t.Fatal(err)
	}
	res := doJSON(t, mux, http.MethodGet, "/api/v1/auth/invitations/"+inv.RawToken, nil, nil)
	if res.Code != 200 {
		t.Fatalf("code=%d", res.Code)
	}
	res2 := doJSON(t, mux, http.MethodPost, "/api/v1/auth/signup", map[string]string{
		"email": "nolog@example.com", "password": "super-secret-password", "business_name": "Studio", "slug": "nolog-studio", "inviteToken": inv.RawToken,
	}, nil)
	if res2.Code != 201 {
		t.Fatalf("signup=%d %s", res2.Code, res2.Body.String())
	}
	logs := logBuf.String()
	if strings.Contains(logs, inv.RawToken) {
		t.Fatalf("raw token leaked in logs: %s", logs)
	}
}
