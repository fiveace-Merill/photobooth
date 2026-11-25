package auth

import (
	"errors"
	"net/http"
	"time"

	"github.com/fiveace-Merill/photobooth/internal/auth/store"
	"github.com/fiveace-Merill/photobooth/internal/platform/httpx"
	"github.com/jackc/pgx/v5/pgtype"
)

// Handler exposes auth HTTP endpoints. It translates HTTP requests into service
// calls and service results/errors into HTTP responses.
type Handler struct {
	svc          *Service
	cookieSecure bool
}

// NewHandler creates an auth Handler.
// cookieSecure should be true in production (HTTPS); config controls it.
func NewHandler(svc *Service, cookieSecure bool) *Handler {
	return &Handler{svc: svc, cookieSecure: cookieSecure}
}

// Register mounts auth routes on mux. Routes are rooted at /api/v1/auth.
func (h *Handler) Register(mux *http.ServeMux, _ func(http.Handler) http.Handler) {
	// The middleware parameter is accepted so callers can pass it explicitly; in
	// this slice the app wires middleware globally in main. We keep signature
	// flexible and simple.
	mux.HandleFunc("POST /api/v1/auth/signup", h.handleSignup)
	mux.HandleFunc("POST /api/v1/auth/login", h.handleLogin)
	mux.HandleFunc("POST /api/v1/auth/logout", h.handleLogout)
	mux.Handle("GET /api/v1/auth/me", RequireAuth(http.HandlerFunc(h.handleMe)))
}

type signupRequest struct {
	Email        string `json:"email"`
	Password     string `json:"password"`
	BusinessName string `json:"business_name"`
	Slug         string `json:"slug"`
}

type authResponse struct {
	ID           string `json:"id"`
	Email        string `json:"email"`
	BusinessName string `json:"business_name"`
	Slug         string `json:"slug"`
}

func (h *Handler) handleSignup(w http.ResponseWriter, r *http.Request) {
	var req signupRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	photog, sess, err := h.svc.Signup(r.Context(), SignupInput{
		Email:        req.Email,
		Password:     req.Password,
		BusinessName: req.BusinessName,
		Slug:         req.Slug,
	})
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidInput):
			httpx.Error(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, ErrEmailTaken):
			httpx.Error(w, http.StatusConflict, "email already in use")
		default:
			// Do not leak internal errors; log them in real code if logger is added.
			httpx.Error(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	setSessionCookie(w, sess.RawToken, sess.ExpiresAt, h.cookieSecure)
	httpx.WriteJSON(w, http.StatusCreated, toAuthResponse(photog))
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *Handler) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	photog, sess, err := h.svc.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			httpx.Error(w, http.StatusUnauthorized, "invalid email or password")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "internal server error")
		return
	}

	setSessionCookie(w, sess.RawToken, sess.ExpiresAt, h.cookieSecure)
	httpx.WriteJSON(w, http.StatusOK, toAuthResponse(photog))
}

func (h *Handler) handleLogout(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie(SessionCookieName)
	if err == nil && c.Value != "" {
		// Best effort: delete session from DB. Even if it fails, clear cookie.
		_ = h.svc.Logout(r.Context(), c.Value)
	}
	h.clearSessionCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) handleMe(w http.ResponseWriter, r *http.Request) {
	photog, ok := PhotographerFromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toAuthResponse(photog))
}

func setSessionCookie(w http.ResponseWriter, rawToken string, expiresAt time.Time, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    rawToken,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		Expires:  expiresAt,
		MaxAge:   int(time.Until(expiresAt).Seconds()),
	})
}

func (h *Handler) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
	})
}

func toAuthResponse(p store.Photographer) authResponse {
	return authResponse{
		ID:           uuidToString(p.ID),
		Email:        p.Email,
		BusinessName: p.BusinessName,
		Slug:         p.Slug,
	}
}

func uuidToString(id pgtype.UUID) string {
	if !id.Valid {
		return ""
	}
	b := id.Bytes
	// Format 16 bytes as xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx.
	return fmtUUID(b)
}

func fmtUUID(b [16]byte) string {
	hex := "0123456789abcdef"
	out := make([]byte, 36)
	pos := 0
	for i, by := range b {
		if i == 4 || i == 6 || i == 8 || i == 10 {
			out[pos] = '-'
			pos++
		}
		out[pos] = hex[by>>4]
		out[pos+1] = hex[by&0x0f]
		pos += 2
	}
	return string(out)
}
