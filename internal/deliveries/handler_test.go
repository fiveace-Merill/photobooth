package deliveries

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/fiveace-Merill/photobooth/internal/auth"
	authstore "github.com/fiveace-Merill/photobooth/internal/auth/store"
	"github.com/fiveace-Merill/photobooth/internal/clients"
	"github.com/fiveace-Merill/photobooth/internal/notify"
	"github.com/fiveace-Merill/photobooth/internal/photos"
	"github.com/fiveace-Merill/photobooth/internal/platform/database"
	"github.com/fiveace-Merill/photobooth/internal/platform/testdb"
	"github.com/fiveace-Merill/photobooth/internal/shoots"
	photostorage "github.com/fiveace-Merill/photobooth/internal/storage"
)

type env struct {
	tdb      *testdb.DB
	store    *photostorage.Fake
	mux      http.Handler
	authA    *http.Cookie
	authB    *http.Cookie
	pidA     pgtype.UUID
	pidB     pgtype.UUID
	svc      *Service
	notifier *fakeNotifier
}

func setupEnv(t *testing.T) env {
	t.Helper()
	tdb := testdb.Open(t)
	fake := photostorage.NewFake()
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
	photosSvc := photos.NewService(photos.Deps{Pool: tdb.App, Objects: fake, MaxPhotoBytes: 60 << 20, MaxPhotosPerShoot: 500, MaxImageMegapixels: 100})
	photosH := photos.NewHandler(photosSvc)
	notifyFake := &fakeNotifier{}
	delSvc := NewService(tdb.App, fake, "https://app.test", notifyFake)
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
	return env{tdb: tdb, store: fake, mux: root, authA: a, authB: b, pidA: pidA, pidB: pidB, svc: delSvc, notifier: notifyFake}
}

func signup(t *testing.T, h http.Handler, email, biz, slug string) (*http.Cookie, pgtype.UUID) {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"email": email, "password": "super-secret-password", "business_name": biz, "slug": slug})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/signup", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 201 {
		t.Fatalf("signup %d %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	var pid pgtype.UUID
	_ = pid.Scan(resp["id"].(string))
	for _, c := range rec.Result().Cookies() {
		if c.Name == auth.SessionCookieName {
			return c, pid
		}
	}
	t.Fatal("no session cookie")
	return nil, pgtype.UUID{}
}

func doJSON(t *testing.T, h http.Handler, method, path string, body any, cookie *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func createClientShoot(t *testing.T, e env, cookie *http.Cookie, pid pgtype.UUID, price int64) (clientID, shootID uuid.UUID) {
	t.Helper()
	rec := doJSON(t, e.mux, http.MethodPost, "/api/v1/clients", map[string]any{"name": "Client", "phone": "0712000001"}, cookie)
	if rec.Code != 201 {
		t.Fatalf("client %d %s", rec.Code, rec.Body.String())
	}
	var c map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &c)
	clientID = uuid.MustParse(c["id"].(string))
	rec = doJSON(t, e.mux, http.MethodPost, "/api/v1/shoots", map[string]any{"clientId": clientID.String(), "title": "Wedding", "shootDate": "2026-11-20", "priceMinor": price, "currency": "KES"}, cookie)
	if rec.Code != 201 {
		t.Fatalf("shoot %d %s", rec.Code, rec.Body.String())
	}
	var s map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &s)
	shootID = uuid.MustParse(s["id"].(string))
	return
}

func seedReadyPhoto(t *testing.T, e env, pid pgtype.UUID, shootID uuid.UUID, filename string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	photoID := uuid.New()
	origKey := "orig/" + photoID.String()
	prevKey := "prev/" + photoID.String()
	thumbKey := "thumb/" + photoID.String()
	if err := e.store.Put(ctx, origKey, "image/jpeg", bytes.NewReader([]byte("orig-"+filename)), int64(len("orig-"+filename))); err != nil {
		t.Fatal(err)
	}
	if err := e.store.Put(ctx, prevKey, "image/jpeg", bytes.NewReader([]byte("prev-"+filename)), int64(len("prev-"+filename))); err != nil {
		t.Fatal(err)
	}
	if err := e.store.Put(ctx, thumbKey, "image/jpeg", bytes.NewReader([]byte("thumb-"+filename)), int64(len("thumb-"+filename))); err != nil {
		t.Fatal(err)
	}
	err := database.WithTenant(ctx, e.tdb.App, pid, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO photos (id,photographer_id,shoot_id,filename,content_type,size_bytes,width,height,original_key,preview_key,thumb_key,status,sort_key) VALUES ($1,$2,$3,$4,'image/jpeg',$5,100,80,$6,$7,$8,'ready',0)`, pgtype.UUID{Bytes: photoID, Valid: true}, pid, pgtype.UUID{Bytes: shootID, Valid: true}, filename, int64(len("orig-"+filename)), origKey, prevKey, thumbKey)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return photoID
}

func TestDeliveryTenantIsolation(t *testing.T) {
	e := setupEnv(t)
	_, shootA := createClientShoot(t, e, e.authA, e.pidA, 300000)
	seedReadyPhoto(t, e, e.pidA, shootA, "a.jpg")

	// B cannot deliver A's shoot
	rec := doJSON(t, e.mux, http.MethodPost, "/api/v1/shoots/"+shootA.String()+"/deliver", nil, e.authB)
	if rec.Code != 404 {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, e.mux, http.MethodPost, "/api/v1/shoots/"+shootA.String()+"/deliver", nil, e.authA)
	if rec.Code != 201 {
		t.Fatalf("deliver %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, e.mux, http.MethodPost, "/api/v1/shoots/"+shootA.String()+"/link/regenerate", nil, e.authB)
	if rec.Code != 404 {
		t.Fatalf("regen %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, e.mux, http.MethodPost, "/api/v1/shoots/"+shootA.String()+"/mark-paid", map[string]any{"method": "cash"}, e.authB)
	if rec.Code != 404 {
		t.Fatalf("mark paid %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, e.mux, http.MethodGet, "/api/v1/shoots/"+shootA.String()+"/delivery", nil, e.authB)
	if rec.Code != 404 {
		t.Fatalf("get delivery %d %s", rec.Code, rec.Body.String())
	}
}

func TestRawTokenNeverStoredAnd404BodiesIdentical(t *testing.T) {
	e := setupEnv(t)
	_, shootA := createClientShoot(t, e, e.authA, e.pidA, 300000)
	seedReadyPhoto(t, e, e.pidA, shootA, "a.jpg")
	rec := doJSON(t, e.mux, http.MethodPost, "/api/v1/shoots/"+shootA.String()+"/deliver", nil, e.authA)
	if rec.Code != 201 {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	var created map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &created)
	urlStr := created["url"].(string)
	parts := strings.Split(urlStr, "/g/")
	token := parts[len(parts)-1]

	// DB should store hash only
	sum := sha256.Sum256([]byte(token))
	var count int
	err := e.tdb.Owner.QueryRow(context.Background(), `SELECT COUNT(*) FROM gallery_links WHERE token_hash=$1`, sum[:]).Scan(&count)
	if err != nil || count != 1 {
		t.Fatalf("hash count=%d err=%v", count, err)
	}
	err = e.tdb.Owner.QueryRow(context.Background(), `SELECT COUNT(*) FROM gallery_links WHERE encode(token_hash,'hex') LIKE $1`, "%"+token+"%").Scan(&count)
	if err != nil || count != 0 {
		t.Fatalf("raw token stored? count=%d err=%v", count, err)
	}

	// invalid/revoked/expired identical 404
	invalid := doJSON(t, e.mux, http.MethodGet, "/api/v1/deliveries/not-a-real-token", nil, nil)
	rec2 := doJSON(t, e.mux, http.MethodPost, "/api/v1/shoots/"+shootA.String()+"/link/revoke", nil, e.authA)
	if rec2.Code != 204 {
		t.Fatalf("revoke %d %s", rec2.Code, rec2.Body.String())
	}
	revoked := doJSON(t, e.mux, http.MethodGet, "/api/v1/deliveries/"+token, nil, nil)
	// create another link and expire it
	rec3 := doJSON(t, e.mux, http.MethodPost, "/api/v1/shoots/"+shootA.String()+"/link/regenerate", nil, e.authA)
	if rec3.Code != 200 {
		t.Fatalf("regen %d %s", rec3.Code, rec3.Body.String())
	}
	var regen map[string]any
	_ = json.Unmarshal(rec3.Body.Bytes(), &regen)
	tok2 := strings.Split(regen["url"].(string), "/g/")[1]
	_, err = e.tdb.Owner.Exec(context.Background(), `UPDATE gallery_links SET expires_at=now()-interval '1 hour' WHERE shoot_id=$1 AND revoked_at IS NULL`, pgtype.UUID{Bytes: shootA, Valid: true})
	if err != nil {
		t.Fatal(err)
	}
	expired := doJSON(t, e.mux, http.MethodGet, "/api/v1/deliveries/"+tok2, nil, nil)

	for _, r := range []*httptest.ResponseRecorder{invalid, revoked, expired} {
		if r.Code != 404 {
			t.Fatalf("code=%d body=%s", r.Code, r.Body.String())
		}
		if !bytes.Equal(r.Body.Bytes(), invalid.Body.Bytes()) {
			t.Fatalf("404 body differs: %q vs %q", r.Body.String(), invalid.Body.String())
		}
		if r.Header().Get("Cache-Control") != "private, no-store" {
			t.Fatalf("cache=%q", r.Header().Get("Cache-Control"))
		}
		if r.Header().Get("Referrer-Policy") != "no-referrer" {
			t.Fatalf("ref=%q", r.Header().Get("Referrer-Policy"))
		}
	}
}

func TestLockedPayloadHasNoOriginalAndOriginalPathBehavior(t *testing.T) {
	e := setupEnv(t)
	_, shootA := createClientShoot(t, e, e.authA, e.pidA, 300000)
	photoID := seedReadyPhoto(t, e, e.pidA, shootA, "a.jpg")
	rec := doJSON(t, e.mux, http.MethodPost, "/api/v1/shoots/"+shootA.String()+"/deliver", nil, e.authA)
	if rec.Code != 201 {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	var created map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &created)
	token := strings.Split(created["url"].(string), "/g/")[1]

	locked := doJSON(t, e.mux, http.MethodGet, "/api/v1/deliveries/"+token, nil, nil)
	if locked.Code != 200 {
		t.Fatalf("%d %s", locked.Code, locked.Body.String())
	}
	body := locked.Body.Bytes()
	if bytes.Contains(body, []byte("\"original")) || bytes.Contains(body, []byte("/original")) {
		t.Fatalf("locked payload leaked original: %s", body)
	}

	orig := doJSON(t, e.mux, http.MethodGet, "/api/v1/deliveries/"+token+"/photos/"+photoID.String()+"/original", nil, nil)
	if orig.Code != 403 {
		t.Fatalf("locked original code=%d body=%s", orig.Code, orig.Body.String())
	}
	var errBody map[string]any
	_ = json.Unmarshal(orig.Body.Bytes(), &errBody)
	if errBody["code"] != "originals_locked" {
		t.Fatalf("code=%v", errBody["code"])
	}

	rec = doJSON(t, e.mux, http.MethodPost, "/api/v1/shoots/"+shootA.String()+"/mark-paid", map[string]any{"method": "cash", "note": "paid"}, e.authA)
	if rec.Code != 204 {
		t.Fatalf("mark-paid %d %s", rec.Code, rec.Body.String())
	}
	orig = doJSON(t, e.mux, http.MethodGet, "/api/v1/deliveries/"+token+"/photos/"+photoID.String()+"/original", nil, nil)
	if orig.Code != 302 {
		t.Fatalf("unlocked original code=%d body=%s", orig.Code, orig.Body.String())
	}
	if loc := orig.Header().Get("Location"); loc == "" {
		t.Fatal("missing location")
	}
}

func TestPriceZeroUnlocksAtDeliveryAndMarkPaidIdempotency(t *testing.T) {
	e := setupEnv(t)
	_, shootA := createClientShoot(t, e, e.authA, e.pidA, 0)
	seedReadyPhoto(t, e, e.pidA, shootA, "a.jpg")
	rec := doJSON(t, e.mux, http.MethodPost, "/api/v1/shoots/"+shootA.String()+"/deliver", nil, e.authA)
	if rec.Code != 201 {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	var created map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &created)
	token := strings.Split(created["url"].(string), "/g/")[1]
	pub := doJSON(t, e.mux, http.MethodGet, "/api/v1/deliveries/"+token, nil, nil)
	var payload map[string]any
	_ = json.Unmarshal(pub.Body.Bytes(), &payload)
	if payload["status"] != "unlocked" {
		t.Fatalf("status=%v", payload["status"])
	}
	if _, ok := payload["downloadAllUrl"]; !ok {
		t.Fatalf("missing downloadAllUrl: %s", pub.Body.String())
	}

	// mark-paid not allowed for price 0 / already unlocked path
	rec = doJSON(t, e.mux, http.MethodPost, "/api/v1/shoots/"+shootA.String()+"/mark-paid", map[string]any{"method": "cash"}, e.authA)
	if rec.Code != 409 {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestMarkPaidIdempotency(t *testing.T) {
	e := setupEnv(t)
	_, shootA := createClientShoot(t, e, e.authA, e.pidA, 1000)
	seedReadyPhoto(t, e, e.pidA, shootA, "a.jpg")
	rec := doJSON(t, e.mux, http.MethodPost, "/api/v1/shoots/"+shootA.String()+"/deliver", nil, e.authA)
	if rec.Code != 201 {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, e.mux, http.MethodPost, "/api/v1/shoots/"+shootA.String()+"/mark-paid", map[string]any{"method": "cash"}, e.authA)
	if rec.Code != 204 {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, e.mux, http.MethodPost, "/api/v1/shoots/"+shootA.String()+"/mark-paid", map[string]any{"method": "cash"}, e.authA)
	if rec.Code != 409 {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body["code"] != "already_unlocked" {
		t.Fatalf("code=%v", body["code"])
	}
}

func TestZipContentsMatchPhotos(t *testing.T) {
	e := setupEnv(t)
	_, shootA := createClientShoot(t, e, e.authA, e.pidA, 0)
	seedReadyPhoto(t, e, e.pidA, shootA, "one.jpg")
	seedReadyPhoto(t, e, e.pidA, shootA, "two.jpg")
	rec := doJSON(t, e.mux, http.MethodPost, "/api/v1/shoots/"+shootA.String()+"/deliver", nil, e.authA)
	if rec.Code != 201 {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	var created map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &created)
	token := strings.Split(created["url"].(string), "/g/")[1]

	dl := doJSON(t, e.mux, http.MethodGet, "/api/v1/deliveries/"+token+"/download", nil, nil)
	if dl.Code != 200 {
		t.Fatalf("%d %s", dl.Code, dl.Body.String())
	}
	if ct := dl.Header().Get("Content-Type"); ct != "application/zip" {
		t.Fatalf("ct=%q", ct)
	}
	zr, err := zip.NewReader(bytes.NewReader(dl.Body.Bytes()), int64(dl.Body.Len()))
	if err != nil {
		t.Fatal(err)
	}
	if len(zr.File) != 2 {
		t.Fatalf("files=%d", len(zr.File))
	}
	got := map[string]string{}
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(rc)
		rc.Close()
		got[f.Name] = string(b)
	}
	if got["one.jpg"] != "orig-one.jpg" || got["two.jpg"] != "orig-two.jpg" {
		t.Fatalf("got=%v", got)
	}
}

func TestRateLimiter(t *testing.T) {
	e := setupEnv(t)
	h := NewHandler(e.svc)
	h.limiter = newIPLimiter(2, time.Minute)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/deliveries/badtoken", nil)
	req.RemoteAddr = "1.2.3.4:1234"
	h.rateLimit(h.getPublicDelivery)(rr, req)
	if rr.Code != 404 {
		t.Fatalf("%d", rr.Code)
	}
	rr = httptest.NewRecorder()
	h.rateLimit(h.getPublicDelivery)(rr, req)
	if rr.Code != 404 {
		t.Fatalf("%d", rr.Code)
	}
	rr = httptest.NewRecorder()
	h.rateLimit(h.getPublicDelivery)(rr, req)
	if rr.Code != 429 {
		t.Fatalf("%d %s", rr.Code, rr.Body.String())
	}
}

type fakeNotifier struct {
	available  bool
	reason     string
	remaining  int
	sendErr    error
	sendResult NotificationResult
	sentInputs []DeliveryEmailSendInput
	list       []NotificationView
	checkCalls int
}

func (f *fakeNotifier) CheckEmailAvailability(ctx context.Context, photographerID, shootID uuid.UUID, recipient string) (bool, string, error) {
	f.checkCalls++
	if f.reason == "" && f.available {
		return true, "", nil
	}
	if f.reason == "" && !f.available {
		return false, "channel_unavailable", nil
	}
	return f.available, f.reason, nil
}
func (f *fakeNotifier) ReserveDeliveryEmail(ctx context.Context, tx pgx.Tx, in DeliveryEmailSendInput) (ReservedEmail, error) {
	if !f.available {
		if f.reason == "" {
			f.reason = "channel_unavailable"
		}
		switch f.reason {
		case "too_many_sends":
			return ReservedEmail{}, &notify.LimitError{Status: 429, Code: "too_many_sends", Message: "Too many"}
		case "email_daily_limit_reached":
			return ReservedEmail{}, &notify.LimitError{Status: 429, Code: "email_daily_limit_reached", Message: "Daily limit"}
		default:
			return ReservedEmail{}, &notify.LimitError{Status: 422, Code: "channel_unavailable", Message: "Unavailable"}
		}
	}
	return ReservedEmail{ID: uuid.New()}, nil
}
func (f *fakeNotifier) FinalizeDeliveryEmail(ctx context.Context, in DeliveryEmailSendInput, reservationID uuid.UUID) (NotificationResult, error) {
	f.sentInputs = append(f.sentInputs, in)
	if f.sendErr != nil {
		return NotificationResult{}, f.sendErr
	}
	if f.sendResult.Channel == "" {
		return NotificationResult{Channel: "email", Status: "sent"}, nil
	}
	return f.sendResult, nil
}
func (f *fakeNotifier) ListShootNotifications(ctx context.Context, photographerID, shootID uuid.UUID, limit int) ([]NotificationView, error) {
	return f.list, nil
}
func (f *fakeNotifier) EmailsRemainingToday(ctx context.Context, photographerID uuid.UUID) (int, error) {
	return f.remaining, nil
}
