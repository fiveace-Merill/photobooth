package photos

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/fiveace-Merill/photobooth/internal/auth"
	"github.com/fiveace-Merill/photobooth/internal/platform/httpx"
)

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Register(mux *http.ServeMux, requireAuth func(http.HandlerFunc) http.HandlerFunc) {
	mux.HandleFunc("POST /api/v1/shoots/{shootId}/photos/uploads", requireAuth(h.createUpload))
	mux.HandleFunc("POST /api/v1/shoots/{shootId}/photos/{photoId}/uploads/parts", requireAuth(h.presignParts))
	mux.HandleFunc("POST /api/v1/shoots/{shootId}/photos/{photoId}/complete", requireAuth(h.complete))
	mux.HandleFunc("GET /api/v1/shoots/{shootId}/photos", requireAuth(h.list))
	mux.HandleFunc("POST /api/v1/shoots/{shootId}/photos/{photoId}/retry", requireAuth(h.retry))
	mux.HandleFunc("DELETE /api/v1/shoots/{shootId}/photos/{photoId}", requireAuth(h.delete))
}

func (h *Handler) createUpload(w http.ResponseWriter, r *http.Request) {
	p, ok := auth.PhotographerFromContext(r.Context())
	if !ok {
		httpx.ErrorCode(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}
	shootID, err := uuid.Parse(r.PathValue("shootId"))
	if err != nil {
		httpx.ErrorCode(w, http.StatusNotFound, "not_found", "Shoot not found")
		return
	}
	var in CreateUploadInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		httpx.ErrorCode(w, http.StatusBadRequest, "invalid_json", "Invalid JSON")
		return
	}
	res, err := h.svc.CreateUpload(r.Context(), uuid.UUID(p.ID.Bytes), shootID, in)
	if err != nil {
		writeSvcErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, res)
}

func (h *Handler) presignParts(w http.ResponseWriter, r *http.Request) {
	p, ok := auth.PhotographerFromContext(r.Context())
	if !ok {
		httpx.ErrorCode(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}
	shootID, photoID, ok := parseIDs(w, r)
	if !ok {
		return
	}
	var body struct {
		PartNumbers []int32 `json:"partNumbers"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.ErrorCode(w, http.StatusBadRequest, "invalid_json", "Invalid JSON")
		return
	}
	parts, err := h.svc.PresignParts(r.Context(), uuid.UUID(p.ID.Bytes), shootID, photoID, body.PartNumbers)
	if err != nil {
		writeSvcErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"parts": parts})
}

func (h *Handler) complete(w http.ResponseWriter, r *http.Request) {
	p, ok := auth.PhotographerFromContext(r.Context())
	if !ok {
		httpx.ErrorCode(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}
	shootID, photoID, ok := parseIDs(w, r)
	if !ok {
		return
	}
	var body struct {
		Parts []CompletePart `json:"parts"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.ErrorCode(w, http.StatusBadRequest, "invalid_json", "Invalid JSON")
		return
	}
	photo, err := h.svc.CompleteUpload(r.Context(), uuid.UUID(p.ID.Bytes), shootID, photoID, body.Parts)
	if err != nil {
		writeSvcErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, photo)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	p, ok := auth.PhotographerFromContext(r.Context())
	if !ok {
		httpx.ErrorCode(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}
	shootID, err := uuid.Parse(r.PathValue("shootId"))
	if err != nil {
		httpx.ErrorCode(w, http.StatusNotFound, "not_found", "Shoot not found")
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	res, err := h.svc.List(r.Context(), uuid.UUID(p.ID.Bytes), shootID, int32(limit), int32(offset))
	if err != nil {
		writeSvcErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, res)
}

func (h *Handler) retry(w http.ResponseWriter, r *http.Request) {
	p, ok := auth.PhotographerFromContext(r.Context())
	if !ok {
		httpx.ErrorCode(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}
	shootID, photoID, ok := parseIDs(w, r)
	if !ok {
		return
	}
	photo, err := h.svc.Retry(r.Context(), uuid.UUID(p.ID.Bytes), shootID, photoID)
	if err != nil {
		writeSvcErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, photo)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	p, ok := auth.PhotographerFromContext(r.Context())
	if !ok {
		httpx.ErrorCode(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}
	shootID, photoID, ok := parseIDs(w, r)
	if !ok {
		return
	}
	if err := h.svc.Delete(r.Context(), uuid.UUID(p.ID.Bytes), shootID, photoID); err != nil {
		writeSvcErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func parseIDs(w http.ResponseWriter, r *http.Request) (uuid.UUID, uuid.UUID, bool) {
	shootID, err := uuid.Parse(r.PathValue("shootId"))
	if err != nil {
		httpx.ErrorCode(w, http.StatusNotFound, "not_found", "Shoot not found")
		return uuid.UUID{}, uuid.UUID{}, false
	}
	photoID, err := uuid.Parse(r.PathValue("photoId"))
	if err != nil {
		httpx.ErrorCode(w, http.StatusNotFound, "not_found", "Photo not found")
		return uuid.UUID{}, uuid.UUID{}, false
	}
	return shootID, photoID, true
}

func writeSvcErr(w http.ResponseWriter, err error) {
	if status, code, msg, ok := AsServiceError(err); ok {
		httpx.ErrorCode(w, status, code, msg)
		return
	}
	if strings.Contains(err.Error(), "not_found") {
		httpx.ErrorCode(w, http.StatusNotFound, "not_found", "Not found")
		return
	}
	httpx.ErrorCode(w, http.StatusInternalServerError, "internal_error", "Internal server error")
}
