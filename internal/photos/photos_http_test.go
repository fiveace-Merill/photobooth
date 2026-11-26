package photos_test

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fiveace-Merill/photobooth/internal/auth"
	authstore "github.com/fiveace-Merill/photobooth/internal/auth/store"
	"github.com/fiveace-Merill/photobooth/internal/clients"
	"github.com/fiveace-Merill/photobooth/internal/photos"
	"github.com/fiveace-Merill/photobooth/internal/platform/testdb"
	"github.com/fiveace-Merill/photobooth/internal/shoots"
	photostorage "github.com/fiveace-Merill/photobooth/internal/storage"
	"github.com/jackc/pgx/v5/pgtype"
)

type api struct {
	mux    *http.ServeMux
	cookie *http.Cookie
	pid    pgtype.UUID
}

func setup(t *testing.T) (*testdb.DB, *photostorage.Fake, api, api) {
	t.Helper()
	tdb := testdb.Open(t)
	fake := photostorage.NewFake()
	a := newPhotographer(t, tdb, fake, "a@example.com", "A Studio", "studio-a")
	b := newPhotographer(t, tdb, fake, "b@example.com", "B Studio", "studio-b")
	return tdb, fake, a, b
}

func newPhotographer(t *testing.T, tdb *testdb.DB, fake *photostorage.Fake, email, biz, slug string) api {
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
	shootsH := shoots.NewHandler(shootsSvc)
	photosSvc := photos.NewService(photos.Deps{Pool: tdb.App, Objects: fake, MaxPhotoBytes: 60 << 20, MaxPhotosPerShoot: 500, MaxImageMegapixels: 100})
	photosH := photos.NewHandler(photosSvc)
	mux := http.NewServeMux()
	authH.Register(mux, mw)
	clientsH.Register(mux, auth.RequireAuth)
	shootsH.Register(mux, auth.RequireAuth)
	photosH.Register(mux, func(h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) { auth.RequireAuth(http.HandlerFunc(h)).ServeHTTP(w, r) }
	})
	handler := mw(mux)
	res := doJSON(t, handler, http.MethodPost, "/api/v1/auth/signup", map[string]string{"email": email, "password": "super-secret-password", "business_name": biz, "slug": slug}, nil)
	if res.Code != 201 {
		t.Fatalf("signup: %d %s", res.Code, res.Body.String())
	}
	cookie := findCookie(res.Result().Cookies(), auth.SessionCookieName)
	var body map[string]any
	_ = json.Unmarshal(res.Body.Bytes(), &body)
	var pid pgtype.UUID
	_ = pid.Scan(body["id"].(string))
	out := http.NewServeMux()
	out.Handle("/", handler)
	return api{mux: out, cookie: cookie, pid: pid}
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

func createClientShoot(t *testing.T, a api) (string, string) {
	t.Helper()
	res := doJSON(t, a.mux, http.MethodPost, "/api/v1/clients", map[string]any{"name": "C", "phone": "0712000000"}, []*http.Cookie{a.cookie})
	if res.Code != 201 {
		t.Fatalf("client: %d %s", res.Code, res.Body.String())
	}
	var c map[string]any
	_ = json.Unmarshal(res.Body.Bytes(), &c)
	res = doJSON(t, a.mux, http.MethodPost, "/api/v1/shoots", map[string]any{"clientId": c["id"], "title": "S", "priceMinor": 0, "currency": "KES"}, []*http.Cookie{a.cookie})
	if res.Code != 201 {
		t.Fatalf("shoot: %d %s", res.Code, res.Body.String())
	}
	var s map[string]any
	_ = json.Unmarshal(res.Body.Bytes(), &s)
	return c["id"].(string), s["id"].(string)
}

func smallJPEG() []byte {
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	for y := 0; y < 10; y++ {
		for x := 0; x < 10; x++ {
			img.SetRGBA(x, y, color.RGBA{R: 200, G: 100, B: 50, A: 255})
		}
	}
	var buf bytes.Buffer
	_ = jpeg.Encode(&buf, img, &jpeg.Options{Quality: 80})
	return buf.Bytes()
}

func TestPhotoEndpointsTenantIsolation(t *testing.T) {
	_, fake, A, B := setup(t)
	_, shootA := createClientShoot(t, A)
	_, shootB := createClientShoot(t, B)

	res := doJSON(t, A.mux, http.MethodPost, "/api/v1/shoots/"+shootA+"/photos/uploads", map[string]any{"filename": "a.jpg", "contentType": "image/jpeg", "sizeBytes": 4}, []*http.Cookie{A.cookie})
	if res.Code != 201 {
		t.Fatalf("create upload: %d %s", res.Code, res.Body.String())
	}
	var up map[string]any
	_ = json.Unmarshal(res.Body.Bytes(), &up)
	photoA := up["photoId"].(string)

	// B cannot access A shoot/photos
	for _, tc := range []struct {
		method, path string
		body         any
	}{
		{"GET", "/api/v1/shoots/" + shootA + "/photos", nil},
		{"POST", "/api/v1/shoots/" + shootA + "/photos/" + photoA + "/uploads/parts", map[string]any{"partNumbers": []int{1}}},
		{"POST", "/api/v1/shoots/" + shootA + "/photos/" + photoA + "/complete", map[string]any{"parts": []map[string]any{{"partNumber": 1, "etag": "e"}}}},
		{"POST", "/api/v1/shoots/" + shootA + "/photos/" + photoA + "/retry", map[string]any{}},
		{"DELETE", "/api/v1/shoots/" + shootA + "/photos/" + photoA, nil},
	} {
		res := doJSON(t, B.mux, tc.method, tc.path, tc.body, []*http.Cookie{B.cookie})
		if res.Code != 404 {
			t.Fatalf("%s %s: %d %s", tc.method, tc.path, res.Code, res.Body.String())
		}
	}

	// B can access own shoot
	res = doJSON(t, B.mux, http.MethodGet, "/api/v1/shoots/"+shootB+"/photos", nil, []*http.Cookie{B.cookie})
	if res.Code != 200 {
		t.Fatalf("B own list: %d %s", res.Code, res.Body.String())
	}
	_ = fake
}

func TestPhotoUploadValidation(t *testing.T) {
	_, _, A, _ := setup(t)
	_, shootA := createClientShoot(t, A)
	res := doJSON(t, A.mux, http.MethodPost, "/api/v1/shoots/"+shootA+"/photos/uploads", map[string]any{"filename": "a.gif", "contentType": "image/gif", "sizeBytes": 4}, []*http.Cookie{A.cookie})
	if res.Code != 415 {
		t.Fatalf("type: %d %s", res.Code, res.Body.String())
	}
	res = doJSON(t, A.mux, http.MethodPost, "/api/v1/shoots/"+shootA+"/photos/uploads", map[string]any{"filename": "a.jpg", "contentType": "image/jpeg", "sizeBytes": 0}, []*http.Cookie{A.cookie})
	if res.Code != 413 {
		t.Fatalf("size: %d %s", res.Code, res.Body.String())
	}
}
