package deliveries

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/fiveace-Merill/photobooth/internal/auth"
	authstore "github.com/fiveace-Merill/photobooth/internal/auth/store"
	"github.com/fiveace-Merill/photobooth/internal/clients"
	"github.com/fiveace-Merill/photobooth/internal/notify"
	"github.com/fiveace-Merill/photobooth/internal/photos"
	"github.com/fiveace-Merill/photobooth/internal/platform/testdb"
	"github.com/fiveace-Merill/photobooth/internal/shoots"
	photostorage "github.com/fiveace-Merill/photobooth/internal/storage"
)

type captureSender struct {
	mu    sync.Mutex
	msgs  []notify.Message
	err   error
	delay time.Duration
	sendN atomic.Int32
}

func (s *captureSender) Send(ctx context.Context, msg notify.Message) (string, error) {
	s.sendN.Add(1)
	if s.delay > 0 {
		select {
		case <-time.After(s.delay):
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	s.mu.Lock()
	s.msgs = append(s.msgs, msg)
	s.mu.Unlock()
	if s.err != nil {
		return "", s.err
	}
	return "<msg-1@test>", nil
}

func (s *captureSender) all() []notify.Message {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]notify.Message, len(s.msgs))
	copy(out, s.msgs)
	return out
}

type emailEnv struct {
	env
	sender *captureSender
	logBuf *bytes.Buffer
}

func setupEmailEnv(t *testing.T, sender notify.EmailSender) emailEnv {
	t.Helper()
	tdb := testdb.Open(t)
	fakeStore := photostorage.NewFake()
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	authSvc := auth.NewService(authstore.New(tdb.App), true)
	authH := auth.NewHandler(authSvc, false)
	authMw := auth.NewMiddleware(authSvc, false)
	clientsSvc := clients.NewService(tdb.App)
	clientsH := clients.NewHandler(clientsSvc)
	shootsSvc, err := shoots.NewService(tdb.App, clientsSvc)
	if err != nil {
		t.Fatal(err)
	}
	shootsH := shoots.NewHandler(shootsSvc)
	photosSvc := photos.NewService(photos.Deps{Pool: tdb.App, Objects: fakeStore, MaxPhotoBytes: 60 << 20, MaxPhotosPerShoot: 500, MaxImageMegapixels: 100})
	photosH := photos.NewHandler(photosSvc)
	notifySvc := notify.NewService(tdb.App, sender, notify.Config{FromName: "Photobooth", FromAddr: "noreply@test.local", DailyLimit: 50}, logger)
	delSvc := NewService(tdb.App, fakeStore, "https://app.test", NewNotifyAdapter(notifySvc)).WithLogger(logger)
	delH := NewHandler(delSvc)

	mux := http.NewServeMux()
	authH.Register(mux, authMw)
	clientsH.Register(mux, auth.RequireAuth)
	shootsH.Register(mux, auth.RequireAuth)
	photosH.Register(mux, func(h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) { auth.RequireAuth(http.HandlerFunc(h)).ServeHTTP(w, r) }
	})
	delH.Register(mux, func(h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) { auth.RequireAuth(http.HandlerFunc(h)).ServeHTTP(w, r) }
	})
	root := authMw(mux)
	a, pidA := signup(t, root, "a@example.com", "A Studio", "studio-a")
	b, pidB := signup(t, root, "b@example.com", "B Studio", "studio-b")
	cs, ok := sender.(*captureSender)
	if !ok {
		cs = &captureSender{}
	}
	return emailEnv{env: env{tdb: tdb, store: fakeStore, mux: root, authA: a, authB: b, pidA: pidA, pidB: pidB, svc: delSvc}, sender: cs, logBuf: &logBuf}
}

func createClientShootWithEmail(t *testing.T, e emailEnv, cookie *http.Cookie, email string) (clientID, shootID uuid.UUID) {
	t.Helper()
	body := map[string]any{"name": "Client", "phone": "0712000001"}
	if email != "" {
		body["email"] = email
	}
	rec := doJSON(t, e.mux, http.MethodPost, "/api/v1/clients", body, cookie)
	if rec.Code != 201 {
		t.Fatalf("client %d %s", rec.Code, rec.Body.String())
	}
	var c map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &c)
	clientID = uuid.MustParse(c["id"].(string))
	rec = doJSON(t, e.mux, http.MethodPost, "/api/v1/shoots", map[string]any{"clientId": clientID.String(), "title": "Wedding", "shootDate": "2026-11-20", "priceMinor": 300000, "currency": "KES"}, cookie)
	if rec.Code != 201 {
		t.Fatalf("shoot %d %s", rec.Code, rec.Body.String())
	}
	var s map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &s)
	shootID = uuid.MustParse(s["id"].(string))
	return
}

func assertNoTokenLeak(t *testing.T, e emailEnv, token string) {
	t.Helper()
	if token == "" {
		t.Fatal("empty token")
	}
	if strings.Contains(e.logBuf.String(), token) {
		t.Fatalf("token leaked in logs: %s", e.logBuf.String())
	}
	if strings.Contains(e.logBuf.String(), "/g/"+token) {
		t.Fatalf("url leaked in logs: %s", e.logBuf.String())
	}
	ctx := context.Background()
	var count int
	err := e.tdb.Owner.QueryRow(ctx, `SELECT COUNT(*) FROM notifications WHERE recipient LIKE $1 OR provider_message_id LIKE $1 OR COALESCE(error_code,'') LIKE $1`, "%"+token+"%").Scan(&count)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("token leaked in notifications: %d", count)
	}
	err = e.tdb.Owner.QueryRow(ctx, `SELECT COUNT(*) FROM river_job WHERE args::text LIKE $1 OR errors::text LIKE $1`, "%"+token+"%").Scan(&count)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("token leaked in river_job: %d", count)
	}
}

func TestDeliverEmailChannelUnavailableNoLinkCreated(t *testing.T) {
	sender := &captureSender{}
	e := setupEmailEnv(t, sender)
	_, shootID := createClientShootWithEmail(t, e, e.authA, "")
	seedReadyPhoto(t, e.env, e.pidA, shootID, "a.jpg")
	rec := doJSON(t, e.mux, http.MethodPost, "/api/v1/shoots/"+shootID.String()+"/deliver", map[string]any{"send": map[string]any{"email": true}}, e.authA)
	if rec.Code != 422 {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body["code"] != "channel_unavailable" {
		t.Fatalf("body=%v", body)
	}
	var n int
	if err := e.tdb.Owner.QueryRow(context.Background(), `SELECT COUNT(*) FROM gallery_links WHERE shoot_id=$1`, pgtype.UUID{Bytes: shootID, Valid: true}).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("links created=%d", n)
	}
}

func TestDeliverEmailSuccessAndNoTokenLeak(t *testing.T) {
	sender := &captureSender{}
	e := setupEmailEnv(t, sender)
	_, shootID := createClientShootWithEmail(t, e, e.authA, "client@example.com")
	seedReadyPhoto(t, e.env, e.pidA, shootID, "a.jpg")
	rec := doJSON(t, e.mux, http.MethodPost, "/api/v1/shoots/"+shootID.String()+"/deliver", map[string]any{"send": map[string]any{"email": true}}, e.authA)
	if rec.Code != 201 {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	urlStr := body["url"].(string)
	token := strings.Split(urlStr, "/g/")[1]
	notes, _ := body["notifications"].([]any)
	if len(notes) != 1 {
		t.Fatalf("notifications=%v", body["notifications"])
	}
	n0 := notes[0].(map[string]any)
	if n0["status"] != "sent" || n0["channel"] != "email" {
		t.Fatalf("n=%v", n0)
	}
	if len(sender.all()) != 1 {
		t.Fatalf("sent=%d", len(sender.all()))
	}
	msg := sender.all()[0]
	if strings.Count(msg.Text, urlStr) != 1 {
		t.Fatalf("text=%s", msg.Text)
	}
	if strings.Count(msg.HTML, urlStr) < 1 {
		t.Fatalf("html missing url")
	}
	assertNoTokenLeak(t, e, token)
}

func TestDeliverEmailProviderFailureLeavesLink(t *testing.T) {
	sender := &captureSender{err: notify.ErrSendTimeout}
	e := setupEmailEnv(t, sender)
	_, shootID := createClientShootWithEmail(t, e, e.authA, "client@example.com")
	seedReadyPhoto(t, e.env, e.pidA, shootID, "a.jpg")
	rec := doJSON(t, e.mux, http.MethodPost, "/api/v1/shoots/"+shootID.String()+"/deliver", map[string]any{"send": map[string]any{"email": true}}, e.authA)
	if rec.Code != 201 {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	token := strings.Split(body["url"].(string), "/g/")[1]
	notes := body["notifications"].([]any)
	n0 := notes[0].(map[string]any)
	if n0["status"] != "failed" || n0["errorCode"] != "send_timeout" {
		t.Fatalf("n=%v", n0)
	}
	// link usable
	pub := doJSON(t, e.mux, http.MethodGet, "/api/v1/deliveries/"+token, nil, nil)
	if pub.Code != 200 {
		t.Fatalf("public=%d %s", pub.Code, pub.Body.String())
	}
	var status string
	if err := e.tdb.Owner.QueryRow(context.Background(), `SELECT status FROM notifications WHERE shoot_id=$1`, pgtype.UUID{Bytes: shootID, Valid: true}).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "failed" {
		t.Fatalf("status=%s", status)
	}
	assertNoTokenLeak(t, e, token)
}

func TestDeliverEmailIsolation(t *testing.T) {
	sender := &captureSender{}
	e := setupEmailEnv(t, sender)
	_, shootA := createClientShootWithEmail(t, e, e.authA, "a-client@example.com")
	_, shootB := createClientShootWithEmail(t, e, e.authB, "b-client@example.com")
	seedReadyPhoto(t, e.env, e.pidA, shootA, "a.jpg")
	seedReadyPhoto(t, e.env, e.pidB, shootB, "b.jpg")

	rec := doJSON(t, e.mux, http.MethodPost, "/api/v1/shoots/"+shootA.String()+"/deliver", map[string]any{"send": map[string]any{"email": true}}, e.authA)
	if rec.Code != 201 {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}

	// B cannot read A notifications
	rec = doJSON(t, e.mux, http.MethodGet, "/api/v1/shoots/"+shootA.String()+"/notifications", nil, e.authB)
	if rec.Code != 404 {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	// B cannot trigger send for A shoot
	rec = doJSON(t, e.mux, http.MethodPost, "/api/v1/shoots/"+shootA.String()+"/deliver", map[string]any{"send": map[string]any{"email": true}}, e.authB)
	if rec.Code != 404 {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestDeliverEmailPerShootThrottle(t *testing.T) {
	sender := &captureSender{}
	e := setupEmailEnv(t, sender)
	_, shootID := createClientShootWithEmail(t, e, e.authA, "client@example.com")
	seedReadyPhoto(t, e.env, e.pidA, shootID, "a.jpg")

	// first deliver creates link and counts as 1 send
	rec := doJSON(t, e.mux, http.MethodPost, "/api/v1/shoots/"+shootID.String()+"/deliver", map[string]any{"send": map[string]any{"email": true}}, e.authA)
	if rec.Code != 201 {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	for i := 0; i < 4; i++ {
		rec = doJSON(t, e.mux, http.MethodPost, "/api/v1/shoots/"+shootID.String()+"/link/regenerate", map[string]any{"send": map[string]any{"email": true}}, e.authA)
		if rec.Code != 200 {
			t.Fatalf("i=%d code=%d body=%s", i, rec.Code, rec.Body.String())
		}
	}
	rec = doJSON(t, e.mux, http.MethodPost, "/api/v1/shoots/"+shootID.String()+"/link/regenerate", map[string]any{"send": map[string]any{"email": true}}, e.authA)
	if rec.Code != 429 {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body["code"] != "too_many_sends" {
		t.Fatalf("body=%v", body)
	}
}

func TestDeliverEmailDailyCapRace(t *testing.T) {
	sender := &captureSender{}
	e := setupEmailEnv(t, sender)
	// set daily limit to 1 by recreating notify service? easier: create env with custom limit via direct service.
	// build a new env with limit 1
	tdb := e.tdb
	fakeStore := e.store
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	notifySvc := notify.NewService(tdb.App, sender, notify.Config{FromName: "Photobooth", FromAddr: "noreply@test.local", DailyLimit: 1}, logger)
	delSvc := NewService(tdb.App, fakeStore, "https://app.test", NewNotifyAdapter(notifySvc)).WithLogger(logger)
	delH := NewHandler(delSvc)
	// Rebuild mux minimally by replacing handler is hard; instead call service directly with concurrent goroutines.
	_, shootA := createClientShootWithEmail(t, e, e.authA, "a@example.com")
	bodyB := map[string]any{"name": "Client2", "phone": "0712000002", "email": "b@example.com"}
	rec := doJSON(t, e.mux, http.MethodPost, "/api/v1/clients", bodyB, e.authA)
	if rec.Code != 201 {
		t.Fatalf("client2 %d %s", rec.Code, rec.Body.String())
	}
	var c2 map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &c2)
	clientB := uuid.MustParse(c2["id"].(string))
	rec = doJSON(t, e.mux, http.MethodPost, "/api/v1/shoots", map[string]any{"clientId": clientB.String(), "title": "Wedding2", "shootDate": "2026-11-20", "priceMinor": 300000, "currency": "KES"}, e.authA)
	if rec.Code != 201 {
		t.Fatalf("shoot2 %d %s", rec.Code, rec.Body.String())
	}
	var s2 map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &s2)
	shootB := uuid.MustParse(s2["id"].(string))
	seedReadyPhoto(t, e.env, e.pidA, shootA, "a.jpg")
	seedReadyPhoto(t, e.env, e.pidA, shootB, "b.jpg")
	_ = delH

	type res struct {
		code string
		err  error
	}
	ch := make(chan res, 2)
	send := func(shootID uuid.UUID) {
		_, err := delSvc.Deliver(context.Background(), uuid.UUID(e.pidA.Bytes), shootID, SendOptions{Email: true})
		if err == nil {
			ch <- res{code: "ok"}
			return
		}
		var sc interface{ StatusCode() int }
		if errors.As(err, &sc) {
			ch <- res{code: http.StatusText(sc.StatusCode())}
			return
		}
		// deliveries errCode returns *httpx-like? inspect
		type coded interface{ Error() string }
		if le, ok := notify.AsLimitError(err); ok {
			ch <- res{code: le.Code}
			return
		}
		ch <- res{err: err}
	}
	go send(shootA)
	go send(shootB)
	r1, r2 := <-ch, <-ch
	_ = r1
	_ = r2
	var sentCount int
	if err := e.tdb.Owner.QueryRow(context.Background(), `SELECT COUNT(*) FROM notifications WHERE photographer_id=$1 AND status IN ('queued','sent','failed')`, e.pidA).Scan(&sentCount); err != nil {
		t.Fatal(err)
	}
	if sentCount != 1 {
		t.Fatalf("sentCount=%d r1=%+v r2=%+v", sentCount, r1, r2)
	}
}

func TestNotificationAndRiverJobContainNoTokenOrURL(t *testing.T) {
	sender := &captureSender{}
	e := setupEmailEnv(t, sender)
	_, shootID := createClientShootWithEmail(t, e, e.authA, "client@example.com")
	seedReadyPhoto(t, e.env, e.pidA, shootID, "a.jpg")
	rec := doJSON(t, e.mux, http.MethodPost, "/api/v1/shoots/"+shootID.String()+"/deliver", map[string]any{"send": map[string]any{"email": true}}, e.authA)
	if rec.Code != 201 {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	token := strings.Split(body["url"].(string), "/g/")[1]
	urlStr := body["url"].(string)

	var nCount int
	if err := e.tdb.Owner.QueryRow(context.Background(), `SELECT COUNT(*) FROM notifications WHERE recipient LIKE $1 OR COALESCE(provider_message_id,'') LIKE $1 OR COALESCE(error_code,'') LIKE $1`, "%"+token+"%").Scan(&nCount); err != nil {
		t.Fatal(err)
	}
	if nCount != 0 {
		t.Fatalf("notification leak count=%d", nCount)
	}
	var rCount int
	if err := e.tdb.Owner.QueryRow(context.Background(), `SELECT COUNT(*) FROM river_job WHERE args::text LIKE $1 OR errors::text LIKE $1 OR kind LIKE $1`, "%"+token+"%").Scan(&rCount); err != nil {
		t.Fatal(err)
	}
	if rCount != 0 {
		t.Fatalf("river leak count=%d", rCount)
	}
	// also no /g/ url
	if err := e.tdb.Owner.QueryRow(context.Background(), `SELECT COUNT(*) FROM notifications WHERE recipient LIKE $1`, "%/g/%").Scan(&nCount); err != nil {
		t.Fatal(err)
	}
	if nCount != 0 {
		t.Fatalf("notification url leak=%d", nCount)
	}
	if err := e.tdb.Owner.QueryRow(context.Background(), `SELECT COUNT(*) FROM river_job WHERE args::text LIKE $1`, "%"+urlStr+"%").Scan(&rCount); err != nil {
		t.Fatal(err)
	}
	if rCount != 0 {
		t.Fatalf("river url leak=%d", rCount)
	}
}
