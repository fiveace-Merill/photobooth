package shoots

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/fiveace-Merill/photobooth/internal/auth"
	"github.com/fiveace-Merill/photobooth/internal/platform/httpx"
	"github.com/jackc/pgx/v5/pgtype"
)

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Register(mux *http.ServeMux, authMw func(http.Handler) http.Handler) {
	mux.Handle("GET /api/v1/shoots", authMw(http.HandlerFunc(h.handleList)))
	mux.Handle("POST /api/v1/shoots", authMw(http.HandlerFunc(h.handleCreate)))
	mux.Handle("GET /api/v1/shoots/summary", authMw(http.HandlerFunc(h.handleSummary)))
	mux.Handle("GET /api/v1/shoots/{id}", authMw(http.HandlerFunc(h.handleGet)))
	mux.Handle("PATCH /api/v1/shoots/{id}", authMw(http.HandlerFunc(h.handlePatch)))
	mux.Handle("POST /api/v1/shoots/{id}/status", authMw(http.HandlerFunc(h.handleStatus)))
	mux.Handle("DELETE /api/v1/shoots/{id}", authMw(http.HandlerFunc(h.handleDelete)))
}

type shootResponse struct {
	ID          string  `json:"id"`
	ClientID    string  `json:"clientId"`
	ClientName  string  `json:"clientName,omitempty"`
	Title       string  `json:"title"`
	ShootDate   *string `json:"shootDate,omitempty"`
	Status      string  `json:"status"`
	DueDate     *string `json:"dueDate,omitempty"`
	PriceMinor  int64   `json:"priceMinor"`
	Currency    string  `json:"currency"`
	Overdue     bool    `json:"overdue"`
	DaysOverdue int     `json:"daysOverdue"`
	CreatedAt   string  `json:"createdAt"`
	UpdatedAt   string  `json:"updatedAt"`
}

func (h *Handler) handleList(w http.ResponseWriter, r *http.Request) {
	pid, ok := pidFrom(r)
	if !ok {
		httpx.ErrorCode(w, 401, "unauthorized", "unauthorized")
		return
	}
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))
	var overdue *bool
	if v := q.Get("overdue"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			httpx.ErrorCode(w, 400, "invalid_input", "invalid overdue filter")
			return
		}
		overdue = &b
	}
	var clientID pgtype.UUID
	if v := q.Get("clientId"); v != "" {
		id, err := parseUUID(v)
		if err != nil {
			httpx.ErrorCode(w, 400, "invalid_input", "invalid clientId")
			return
		}
		clientID = id
	}
	res, err := h.svc.List(r.Context(), pid, ListFilter{
		StatusCSV: q.Get("status"), Overdue: overdue, ClientID: clientID, Q: q.Get("q"), Sort: q.Get("sort"), Limit: limit, Offset: offset,
	})
	if err != nil {
		writeShootErr(w, err)
		return
	}
	items := make([]shootResponse, 0, len(res.Items))
	for _, it := range res.Items {
		items = append(items, toResponse(it))
	}
	httpx.WriteJSON(w, 200, map[string]any{"items": items, "total": res.Total, "limit": res.Limit, "offset": res.Offset})
}

func (h *Handler) handleCreate(w http.ResponseWriter, r *http.Request) {
	pid, ok := pidFrom(r)
	if !ok {
		httpx.ErrorCode(w, 401, "unauthorized", "unauthorized")
		return
	}
	var req struct {
		ClientID   string  `json:"clientId"`
		Title      string  `json:"title"`
		ShootDate  *string `json:"shootDate"`
		DueDate    *string `json:"dueDate"`
		PriceMinor *int64  `json:"priceMinor"`
		Currency   string  `json:"currency"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.ErrorCode(w, 400, "invalid_json", "invalid JSON body")
		return
	}
	cid, err := parseUUID(req.ClientID)
	if err != nil {
		httpx.ErrorCode(w, 400, "invalid_input", "invalid clientId")
		return
	}
	sd, err := parseDatePtr(req.ShootDate)
	if err != nil {
		httpx.ErrorCode(w, 400, "invalid_input", "invalid shootDate")
		return
	}
	dd, err := parseDatePtr(req.DueDate)
	if err != nil {
		httpx.ErrorCode(w, 400, "invalid_input", "invalid dueDate")
		return
	}
	pm := int64(0)
	if req.PriceMinor != nil {
		pm = *req.PriceMinor
	}
	sh, err := h.svc.Create(r.Context(), pid, CreateInput{ClientID: cid, Title: req.Title, ShootDate: sd, DueDate: dd, PriceMinor: pm, Currency: req.Currency})
	if err != nil {
		writeShootErr(w, err)
		return
	}
	// decorate for response
	item, err := h.svc.Get(r.Context(), pid, sh.ID)
	if err != nil {
		writeShootErr(w, err)
		return
	}
	httpx.WriteJSON(w, 201, toResponse(item))
}

func (h *Handler) handleGet(w http.ResponseWriter, r *http.Request) {
	pid, ok := pidFrom(r)
	if !ok {
		httpx.ErrorCode(w, 401, "unauthorized", "unauthorized")
		return
	}
	id, err := parseUUID(r.PathValue("id"))
	if err != nil {
		httpx.ErrorCode(w, 404, "not_found", "shoot not found")
		return
	}
	item, err := h.svc.Get(r.Context(), pid, id)
	if err != nil {
		writeShootErr(w, err)
		return
	}
	httpx.WriteJSON(w, 200, toResponse(item))
}

func (h *Handler) handlePatch(w http.ResponseWriter, r *http.Request) {
	pid, ok := pidFrom(r)
	if !ok {
		httpx.ErrorCode(w, 401, "unauthorized", "unauthorized")
		return
	}
	id, err := parseUUID(r.PathValue("id"))
	if err != nil {
		httpx.ErrorCode(w, 404, "not_found", "shoot not found")
		return
	}
	var raw map[string]any
	if err := httpx.DecodeJSON(r, &raw); err != nil {
		httpx.ErrorCode(w, 400, "invalid_json", "invalid JSON body")
		return
	}
	var req struct {
		ClientID   *string `json:"clientId"`
		Title      *string `json:"title"`
		ShootDate  *string `json:"shootDate"`
		DueDate    *string `json:"dueDate"`
		PriceMinor *int64  `json:"priceMinor"`
		Currency   *string `json:"currency"`
	}
	// re-decode via marshal for simplicity
	// simpler: manual extraction
	if v, ok := raw["clientId"]; ok {
		if s, ok := v.(string); ok {
			req.ClientID = &s
		} else if v == nil {
			s := ""
			req.ClientID = &s
		}
	}
	if v, ok := raw["title"]; ok {
		if s, ok := v.(string); ok {
			req.Title = &s
		}
	}
	_, shootDateSet := raw["shootDate"]
	_, dueDateSet := raw["dueDate"]
	if v, ok := raw["shootDate"]; ok {
		if s, ok := v.(string); ok {
			req.ShootDate = &s
		} else if v == nil {
			req.ShootDate = nil
		}
	}
	if v, ok := raw["dueDate"]; ok {
		if s, ok := v.(string); ok {
			req.DueDate = &s
		} else if v == nil {
			req.DueDate = nil
		}
	}
	if v, ok := raw["priceMinor"]; ok {
		switch n := v.(type) {
		case float64:
			i := int64(n)
			req.PriceMinor = &i
		}
	}
	if v, ok := raw["currency"]; ok {
		if s, ok := v.(string); ok {
			req.Currency = &s
		}
	}
	in := PatchInput{Title: req.Title, PriceMinor: req.PriceMinor, Currency: req.Currency, ShootDateSet: shootDateSet, DueDateSet: dueDateSet}
	if req.ClientID != nil {
		cid, err := parseUUID(*req.ClientID)
		if err != nil {
			httpx.ErrorCode(w, 400, "invalid_input", "invalid clientId")
			return
		}
		in.ClientID = &cid
	}
	if shootDateSet {
		sd, err := parseDatePtr(req.ShootDate)
		if err != nil {
			httpx.ErrorCode(w, 400, "invalid_input", "invalid shootDate")
			return
		}
		in.ShootDate = sd
	}
	if dueDateSet {
		dd, err := parseDatePtr(req.DueDate)
		if err != nil {
			httpx.ErrorCode(w, 400, "invalid_input", "invalid dueDate")
			return
		}
		in.DueDate = dd
	}
	sh, err := h.svc.Update(r.Context(), pid, id, in)
	if err != nil {
		writeShootErr(w, err)
		return
	}
	item, err := h.svc.Get(r.Context(), pid, sh.ID)
	if err != nil {
		writeShootErr(w, err)
		return
	}
	httpx.WriteJSON(w, 200, toResponse(item))
}

func (h *Handler) handleStatus(w http.ResponseWriter, r *http.Request) {
	pid, ok := pidFrom(r)
	if !ok {
		httpx.ErrorCode(w, 401, "unauthorized", "unauthorized")
		return
	}
	id, err := parseUUID(r.PathValue("id"))
	if err != nil {
		httpx.ErrorCode(w, 404, "not_found", "shoot not found")
		return
	}
	var req struct {
		Status string `json:"status"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.ErrorCode(w, 400, "invalid_json", "invalid JSON body")
		return
	}
	sh, err := h.svc.SetStatus(r.Context(), pid, id, req.Status)
	if err != nil {
		writeShootErr(w, err)
		return
	}
	item, err := h.svc.Get(r.Context(), pid, sh.ID)
	if err != nil {
		writeShootErr(w, err)
		return
	}
	httpx.WriteJSON(w, 200, toResponse(item))
}

func (h *Handler) handleDelete(w http.ResponseWriter, r *http.Request) {
	pid, ok := pidFrom(r)
	if !ok {
		httpx.ErrorCode(w, 401, "unauthorized", "unauthorized")
		return
	}
	id, err := parseUUID(r.PathValue("id"))
	if err != nil {
		httpx.ErrorCode(w, 404, "not_found", "shoot not found")
		return
	}
	if err := h.svc.Delete(r.Context(), pid, id); err != nil {
		writeShootErr(w, err)
		return
	}
	w.WriteHeader(204)
}

func (h *Handler) handleSummary(w http.ResponseWriter, r *http.Request) {
	pid, ok := pidFrom(r)
	if !ok {
		httpx.ErrorCode(w, 401, "unauthorized", "unauthorized")
		return
	}
	s, err := h.svc.Summary(r.Context(), pid)
	if err != nil {
		writeShootErr(w, err)
		return
	}
	httpx.WriteJSON(w, 200, map[string]any{"counts": map[string]int64{"shot": s.Shot, "editing": s.Editing, "ready": s.Ready, "delivered": s.Delivered, "expired": s.Expired}, "overdue": s.Overdue})
}

func writeShootErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		httpx.ErrorCode(w, 404, "not_found", "shoot not found")
	case errors.Is(err, ErrClientNotFound):
		httpx.ErrorCode(w, 404, "client_not_found", "client not found")
	case errors.Is(err, ErrInvalidStatus):
		httpx.ErrorCode(w, 422, "invalid_status_transition", "invalid status transition")
	case errors.Is(err, ErrNotDeletable):
		httpx.ErrorCode(w, 409, "shoot_not_deletable", "shoot cannot be deleted")
	case errors.Is(err, ErrInvalidInput):
		httpx.ErrorCode(w, 400, "invalid_input", strings.TrimPrefix(err.Error(), "invalid input: "))
	default:
		httpx.ErrorCode(w, 500, "internal", "internal server error")
	}
}

func toResponse(it ListItem) shootResponse {
	sh := it.Shoot
	return shootResponse{
		ID: uuidString(sh.ID), ClientID: uuidString(sh.ClientID), ClientName: it.ClientName,
		Title: sh.Title, ShootDate: datePtr(sh.ShootDate), Status: sh.Status, DueDate: datePtr(sh.DueDate),
		PriceMinor: sh.PriceMinor, Currency: sh.Currency, Overdue: it.Overdue, DaysOverdue: it.DaysOverdue,
		CreatedAt: sh.CreatedAt.Time.UTC().Format(time.RFC3339), UpdatedAt: sh.UpdatedAt.Time.UTC().Format(time.RFC3339),
	}
}

func datePtr(d pgtype.Date) *string {
	if !d.Valid {
		return nil
	}
	s := d.Time.Format("2006-01-02")
	return &s
}
func parseDatePtr(s *string) (*time.Time, error) {
	if s == nil || strings.TrimSpace(*s) == "" {
		return nil, nil
	}
	t, err := time.Parse("2006-01-02", *s)
	if err != nil {
		return nil, err
	}
	return &t, nil
}
func parseUUID(s string) (pgtype.UUID, error) { var id pgtype.UUID; err := id.Scan(s); return id, err }
func pidFrom(r *http.Request) (pgtype.UUID, bool) {
	p, ok := auth.PhotographerFromContext(r.Context())
	if !ok {
		return pgtype.UUID{}, false
	}
	return p.ID, true
}
