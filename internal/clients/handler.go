package clients

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/fiveace-Merill/photobooth/internal/auth"
	"github.com/fiveace-Merill/photobooth/internal/clients/store"
	"github.com/fiveace-Merill/photobooth/internal/platform/httpx"
	"github.com/jackc/pgx/v5/pgtype"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Register(mux *http.ServeMux, auth func(http.Handler) http.Handler) {
	mux.Handle("GET /api/v1/clients", auth(http.HandlerFunc(h.handleList)))
	mux.Handle("POST /api/v1/clients", auth(http.HandlerFunc(h.handleCreate)))
	mux.Handle("GET /api/v1/clients/{id}", auth(http.HandlerFunc(h.handleGet)))
	mux.Handle("PATCH /api/v1/clients/{id}", auth(http.HandlerFunc(h.handlePatch)))
	mux.Handle("DELETE /api/v1/clients/{id}", auth(http.HandlerFunc(h.handleDelete)))
}

type clientResponse struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Phone     *string `json:"phone,omitempty"`
	Email     *string `json:"email,omitempty"`
	Notes     *string `json:"notes,omitempty"`
	CreatedAt string  `json:"createdAt"`
	UpdatedAt string  `json:"updatedAt"`
}

type listResponse struct {
	Items  []clientResponse `json:"items"`
	Total  int64            `json:"total"`
	Limit  int              `json:"limit"`
	Offset int              `json:"offset"`
}

func (h *Handler) handleList(w http.ResponseWriter, r *http.Request) {
	pid, ok := photographerIDFromContext(r)
	if !ok {
		httpx.ErrorCode(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}
	q := r.URL.Query().Get("q")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	res, err := h.svc.List(r.Context(), pid, q, limit, offset)
	if err != nil {
		httpx.ErrorCode(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	items := make([]clientResponse, 0, len(res.Items))
	for _, c := range res.Items {
		items = append(items, listRowToResponse(c))
	}
	httpx.WriteJSON(w, http.StatusOK, listResponse{Items: items, Total: res.Total, Limit: res.Limit, Offset: res.Offset})
}

func (h *Handler) handleCreate(w http.ResponseWriter, r *http.Request) {
	pid, ok := photographerIDFromContext(r)
	if !ok {
		httpx.ErrorCode(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}
	var req struct {
		Name  string  `json:"name"`
		Phone *string `json:"phone"`
		Email *string `json:"email"`
		Notes *string `json:"notes"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.ErrorCode(w, http.StatusBadRequest, "invalid_json", "invalid JSON body")
		return
	}
	c, err := h.svc.Create(r.Context(), pid, CreateInput{Name: req.Name, Phone: req.Phone, Email: req.Email, Notes: req.Notes})
	if err != nil {
		writeClientErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, clientToResponse(c))
}

func (h *Handler) handleGet(w http.ResponseWriter, r *http.Request) {
	pid, ok := photographerIDFromContext(r)
	if !ok {
		httpx.ErrorCode(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}
	id, err := parseUUID(r.PathValue("id"))
	if err != nil {
		httpx.ErrorCode(w, http.StatusNotFound, "not_found", "client not found")
		return
	}
	c, err := h.svc.Get(r.Context(), pid, id)
	if err != nil {
		writeClientErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, clientToResponse(c))
}

func (h *Handler) handlePatch(w http.ResponseWriter, r *http.Request) {
	pid, ok := photographerIDFromContext(r)
	if !ok {
		httpx.ErrorCode(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}
	id, err := parseUUID(r.PathValue("id"))
	if err != nil {
		httpx.ErrorCode(w, http.StatusNotFound, "not_found", "client not found")
		return
	}
	var req struct {
		Name  *string `json:"name"`
		Phone *string `json:"phone"`
		Email *string `json:"email"`
		Notes *string `json:"notes"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.ErrorCode(w, http.StatusBadRequest, "invalid_json", "invalid JSON body")
		return
	}
	c, err := h.svc.Update(r.Context(), pid, id, PatchInput{Name: req.Name, Phone: req.Phone, Email: req.Email, Notes: req.Notes})
	if err != nil {
		writeClientErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, clientToResponse(c))
}

func (h *Handler) handleDelete(w http.ResponseWriter, r *http.Request) {
	pid, ok := photographerIDFromContext(r)
	if !ok {
		httpx.ErrorCode(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}
	id, err := parseUUID(r.PathValue("id"))
	if err != nil {
		httpx.ErrorCode(w, http.StatusNotFound, "not_found", "client not found")
		return
	}
	if err := h.svc.Delete(r.Context(), pid, id); err != nil {
		writeClientErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeClientErr(w http.ResponseWriter, err error) {
	var conflict *PhoneConflictError
	switch {
	case errors.As(err, &conflict):
		httpx.WriteJSON(w, http.StatusConflict, map[string]any{
			"error":            "client phone already exists",
			"code":             "client_phone_exists",
			"existingClientId": conflict.ExistingID,
		})
	case errors.Is(err, ErrHasShoots):
		httpx.ErrorCode(w, http.StatusConflict, "client_has_shoots", "client has shoots")
	case errors.Is(err, ErrPhoneExists):
		httpx.ErrorCode(w, http.StatusConflict, "client_phone_exists", "client phone already exists")
	case errors.Is(err, ErrNotFound):
		httpx.ErrorCode(w, http.StatusNotFound, "not_found", "client not found")
	case errors.Is(err, ErrInvalidInput):
		httpx.ErrorCode(w, http.StatusBadRequest, "invalid_input", strings.TrimPrefix(err.Error(), "invalid input: "))
	default:
		httpx.ErrorCode(w, http.StatusInternalServerError, "internal", "internal server error")
	}
}

func listRowToResponse(c store.ListClientsRow) clientResponse {
	return clientResponse{
		ID: uuidString(c.ID), Name: c.Name,
		Phone: optText(c.Phone), Email: optText(c.Email), Notes: optText(c.Notes),
		CreatedAt: c.CreatedAt.Time.UTC().Format(timeRFC3339), UpdatedAt: c.UpdatedAt.Time.UTC().Format(timeRFC3339),
	}
}

func clientToResponse(c store.Client) clientResponse {
	return clientResponse{
		ID: uuidString(c.ID), Name: c.Name,
		Phone: optText(c.Phone), Email: optText(c.Email), Notes: optText(c.Notes),
		CreatedAt: c.CreatedAt.Time.UTC().Format(timeRFC3339), UpdatedAt: c.UpdatedAt.Time.UTC().Format(timeRFC3339),
	}
}

func optText(v pgtype.Text) *string {
	if !v.Valid {
		return nil
	}
	s := v.String
	return &s
}

const timeRFC3339 = "2006-01-02T15:04:05Z07:00"

func parseUUID(s string) (pgtype.UUID, error) {
	var id pgtype.UUID
	err := id.Scan(s)
	return id, err
}

func photographerIDFromContext(r *http.Request) (pgtype.UUID, bool) {
	p, ok := auth.PhotographerFromContext(r.Context())
	if !ok {
		return pgtype.UUID{}, false
	}
	return p.ID, true
}
