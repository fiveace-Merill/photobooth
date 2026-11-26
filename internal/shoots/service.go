package shoots

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/fiveace-Merill/photobooth/internal/platform/database"
	"github.com/fiveace-Merill/photobooth/internal/shoots/store"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound       = errors.New("shoot not found")
	ErrClientNotFound = errors.New("client not found")
	ErrInvalidInput   = errors.New("invalid input")
	ErrInvalidStatus  = errors.New("invalid status transition")
	ErrNotDeletable   = errors.New("shoot not deletable")
)

type ClientLookup interface {
	Exists(ctx context.Context, photographerID, id pgtype.UUID) (bool, error)
	NamesByIDs(ctx context.Context, photographerID pgtype.UUID, ids []pgtype.UUID) (map[string]string, error)
}

type Service struct {
	pool    *pgxpool.Pool
	clients ClientLookup
	now     func() time.Time
	loc     *time.Location
}

func NewService(pool *pgxpool.Pool, clients ClientLookup) (*Service, error) {
	loc, err := time.LoadLocation("Africa/Nairobi")
	if err != nil {
		return nil, fmt.Errorf("load Nairobi location: %w", err)
	}
	return &Service{pool: pool, clients: clients, now: time.Now, loc: loc}, nil
}

func (s *Service) WithNow(now func() time.Time) *Service {
	cp := *s
	cp.now = now
	return &cp
}

type CreateInput struct {
	ClientID   pgtype.UUID
	Title      string
	ShootDate  *time.Time
	DueDate    *time.Time
	PriceMinor int64
	Currency   string
}

type PatchInput struct {
	ClientID     *pgtype.UUID
	Title        *string
	ShootDate    *time.Time
	ShootDateSet bool
	DueDate      *time.Time
	DueDateSet   bool
	PriceMinor   *int64
	Currency     *string
}

type ListFilter struct {
	StatusCSV string
	Overdue   *bool
	ClientID  pgtype.UUID
	Q         string
	Sort      string
	Limit     int
	Offset    int
}

type ListItem struct {
	Shoot       store.Shoot
	ClientName  string
	Overdue     bool
	DaysOverdue int
}

type ListResult struct {
	Items  []ListItem
	Total  int64
	Limit  int
	Offset int
}

func (s *Service) Create(ctx context.Context, photographerID pgtype.UUID, in CreateInput) (store.Shoot, error) {
	norm, err := normalizeCreate(in)
	if err != nil {
		return store.Shoot{}, err
	}
	exists, err := s.clients.Exists(ctx, photographerID, norm.ClientID)
	if err != nil {
		return store.Shoot{}, err
	}
	if !exists {
		return store.Shoot{}, ErrClientNotFound
	}
	var out store.Shoot
	err = database.WithTenant(ctx, s.pool, photographerID, func(tx pgx.Tx) error {
		sh, err := store.New(tx).CreateShoot(ctx, store.CreateShootParams{
			PhotographerID: photographerID,
			ClientID:       norm.ClientID,
			Title:          norm.Title,
			ShootDate:      norm.ShootDate,
			Status:         "shot",
			DueDate:        norm.DueDate,
			PriceMinor:     norm.PriceMinor,
			Currency:       "KES",
		})
		if err != nil {
			return err
		}
		out = sh
		return nil
	})
	return out, err
}

func (s *Service) Get(ctx context.Context, photographerID, id pgtype.UUID) (ListItem, error) {
	var out ListItem
	err := database.WithTenant(ctx, s.pool, photographerID, func(tx pgx.Tx) error {
		sh, err := store.New(tx).GetShoot(ctx, store.GetShootParams{PhotographerID: photographerID, ID: id})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		out = s.decorate(sh, map[string]string{})
		return nil
	})
	if err != nil {
		return ListItem{}, err
	}
	names, err := s.clients.NamesByIDs(ctx, photographerID, []pgtype.UUID{out.Shoot.ClientID})
	if err != nil {
		return ListItem{}, err
	}
	out.ClientName = names[uuidString(out.Shoot.ClientID)]
	return out, nil
}

func (s *Service) Update(ctx context.Context, photographerID, id pgtype.UUID, in PatchInput) (store.Shoot, error) {
	var out store.Shoot
	err := database.WithTenant(ctx, s.pool, photographerID, func(tx pgx.Tx) error {
		q := store.New(tx)
		cur, err := q.GetShoot(ctx, store.GetShootParams{PhotographerID: photographerID, ID: id})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		merged, err := mergeShoot(cur, in)
		if err != nil {
			return err
		}
		if merged.ClientID != cur.ClientID {
			exists, err := s.clients.Exists(ctx, photographerID, merged.ClientID)
			if err != nil {
				return err
			}
			if !exists {
				return ErrClientNotFound
			}
		}
		sh, err := q.UpdateShoot(ctx, store.UpdateShootParams{
			PhotographerID: photographerID,
			ID:             id,
			ClientID:       merged.ClientID,
			Title:          merged.Title,
			ShootDate:      merged.ShootDate,
			DueDate:        merged.DueDate,
			PriceMinor:     merged.PriceMinor,
			Currency:       merged.Currency,
		})
		if err != nil {
			return err
		}
		out = sh
		return nil
	})
	return out, err
}

func (s *Service) SetStatus(ctx context.Context, photographerID, id pgtype.UUID, status string) (store.Shoot, error) {
	status = strings.TrimSpace(status)
	if status != "shot" && status != "editing" && status != "ready" {
		return store.Shoot{}, ErrInvalidStatus
	}
	var out store.Shoot
	err := database.WithTenant(ctx, s.pool, photographerID, func(tx pgx.Tx) error {
		q := store.New(tx)
		cur, err := q.GetShoot(ctx, store.GetShootParams{PhotographerID: photographerID, ID: id})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		_ = cur // photographers can set shot/editing/ready in any order
		sh, err := q.UpdateShootStatus(ctx, store.UpdateShootStatusParams{PhotographerID: photographerID, ID: id, Status: status})
		if err != nil {
			return err
		}
		out = sh
		return nil
	})
	return out, err
}

func (s *Service) Delete(ctx context.Context, photographerID, id pgtype.UUID) error {
	return database.WithTenant(ctx, s.pool, photographerID, func(tx pgx.Tx) error {
		q := store.New(tx)
		cur, err := q.GetShoot(ctx, store.GetShootParams{PhotographerID: photographerID, ID: id})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		if cur.Status != "shot" && cur.Status != "editing" && cur.Status != "ready" {
			return ErrNotDeletable
		}
		_, err = q.DeleteShoot(ctx, store.DeleteShootParams{PhotographerID: photographerID, ID: id})
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23503" {
				return ErrNotDeletable
			}
			return err
		}
		return nil
	})
}

func (s *Service) List(ctx context.Context, photographerID pgtype.UUID, f ListFilter) (ListResult, error) {
	limit, offset := normalizePage(f.Limit, f.Offset)
	sort := strings.TrimSpace(f.Sort)
	if sort == "" {
		sort = "due"
	}
	if sort != "due" && sort != "created" {
		return ListResult{}, fmt.Errorf("%w: invalid sort", ErrInvalidInput)
	}
	statusCSV, err := normalizeStatusFilter(f.StatusCSV)
	if err != nil {
		return ListResult{}, err
	}
	today := s.today()
	var rows []store.Shoot
	var total int64
	err = database.WithTenant(ctx, s.pool, photographerID, func(tx pgx.Tx) error {
		q := store.New(tx)
		count, err := q.CountShoots(ctx, store.CountShootsParams{
			PhotographerID: photographerID,
			StatusCsv:      textArg(statusCSV),
			ClientID:       f.ClientID,
			Q:              textArg(f.Q),
			Overdue:        boolArg(f.Overdue),
			Today:          pgtype.Date{Time: today, Valid: true},
		})
		if err != nil {
			return err
		}
		total = count
		if sort == "created" {
			r, err := q.ListShootsByCreated(ctx, store.ListShootsByCreatedParams{
				PhotographerID: photographerID,
				Limit:          int32(limit),
				Offset:         int32(offset),
				StatusCsv:      textArg(statusCSV),
				ClientID:       f.ClientID,
				Q:              textArg(f.Q),
				Overdue:        boolArg(f.Overdue),
				Today:          pgtype.Date{Time: today, Valid: true},
			})
			if err != nil {
				return err
			}
			rows = r
			return nil
		}
		r, err := q.ListShootsByDue(ctx, store.ListShootsByDueParams{
			PhotographerID: photographerID,
			Limit:          int32(limit),
			Offset:         int32(offset),
			StatusCsv:      textArg(statusCSV),
			ClientID:       f.ClientID,
			Q:              textArg(f.Q),
			Overdue:        boolArg(f.Overdue),
			Today:          pgtype.Date{Time: today, Valid: true},
		})
		if err != nil {
			return err
		}
		rows = r
		return nil
	})
	if err != nil {
		return ListResult{}, err
	}
	ids := make([]pgtype.UUID, 0, len(rows))
	seen := map[string]bool{}
	for _, r := range rows {
		k := uuidString(r.ClientID)
		if !seen[k] {
			seen[k] = true
			ids = append(ids, r.ClientID)
		}
	}
	names, err := s.clients.NamesByIDs(ctx, photographerID, ids)
	if err != nil {
		return ListResult{}, err
	}
	items := make([]ListItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, s.decorate(r, names))
	}
	return ListResult{Items: items, Total: total, Limit: limit, Offset: offset}, nil
}

type Summary struct {
	Shot      int64 `json:"shot"`
	Editing   int64 `json:"editing"`
	Ready     int64 `json:"ready"`
	Delivered int64 `json:"delivered"`
	Expired   int64 `json:"expired"`
	Overdue   int64 `json:"overdue"`
}

func (s *Service) Summary(ctx context.Context, photographerID pgtype.UUID) (Summary, error) {
	today := s.today()
	var out Summary
	err := database.WithTenant(ctx, s.pool, photographerID, func(tx pgx.Tx) error {
		row, err := store.New(tx).ShootSummary(ctx, store.ShootSummaryParams{
			PhotographerID: photographerID,
			Column2:        pgtype.Date{Time: today, Valid: true},
		})
		if err != nil {
			return err
		}
		out = Summary{Shot: row.Shot, Editing: row.Editing, Ready: row.Ready, Delivered: row.Delivered, Expired: row.Expired, Overdue: row.Overdue}
		return nil
	})
	return out, err
}

func (s *Service) decorate(sh store.Shoot, names map[string]string) ListItem {
	overdue, days := Overdue(s.today(), sh)
	return ListItem{Shoot: sh, ClientName: names[uuidString(sh.ClientID)], Overdue: overdue, DaysOverdue: days}
}

func (s *Service) today() time.Time {
	n := s.now().In(s.loc)
	return time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, time.UTC)
}

func Overdue(today time.Time, sh store.Shoot) (bool, int) {
	if !sh.DueDate.Valid {
		return false, 0
	}
	if sh.Status != "shot" && sh.Status != "editing" && sh.Status != "ready" {
		return false, 0
	}
	due := time.Date(sh.DueDate.Time.Year(), sh.DueDate.Time.Month(), sh.DueDate.Time.Day(), 0, 0, 0, 0, time.UTC)
	if !today.After(due) {
		return false, 0
	}
	days := int(today.Sub(due).Hours() / 24)
	if days < 1 {
		days = 1
	}
	return true, days
}

type normalizedShoot struct {
	ClientID   pgtype.UUID
	Title      string
	ShootDate  pgtype.Date
	DueDate    pgtype.Date
	PriceMinor int64
	Currency   string
}

func normalizeCreate(in CreateInput) (normalizedShoot, error) {
	title := strings.TrimSpace(in.Title)
	if len(title) < 1 || len(title) > 160 {
		return normalizedShoot{}, fmt.Errorf("%w: title must be 1-160 characters", ErrInvalidInput)
	}
	if !in.ClientID.Valid {
		return normalizedShoot{}, fmt.Errorf("%w: clientId is required", ErrInvalidInput)
	}
	if in.PriceMinor < 0 || in.PriceMinor > 100000000 {
		return normalizedShoot{}, fmt.Errorf("%w: priceMinor must be between 0 and 100000000", ErrInvalidInput)
	}
	cur := strings.TrimSpace(in.Currency)
	if cur == "" {
		cur = "KES"
	}
	if cur != "KES" {
		return normalizedShoot{}, fmt.Errorf("%w: currency must be KES", ErrInvalidInput)
	}
	sd := dateFromPtr(in.ShootDate)
	dd := dateFromPtr(in.DueDate)
	if sd.Valid && dd.Valid && dd.Time.Before(sd.Time) {
		return normalizedShoot{}, fmt.Errorf("%w: dueDate cannot be before shootDate", ErrInvalidInput)
	}
	return normalizedShoot{ClientID: in.ClientID, Title: title, ShootDate: sd, DueDate: dd, PriceMinor: in.PriceMinor, Currency: cur}, nil
}

func mergeShoot(cur store.Shoot, in PatchInput) (store.Shoot, error) {
	if in.ClientID != nil {
		cur.ClientID = *in.ClientID
	}
	if in.Title != nil {
		cur.Title = strings.TrimSpace(*in.Title)
	}
	if in.ShootDateSet {
		cur.ShootDate = dateFromPtr(in.ShootDate)
	}
	if in.DueDateSet {
		cur.DueDate = dateFromPtr(in.DueDate)
	}
	if in.PriceMinor != nil {
		cur.PriceMinor = *in.PriceMinor
	}
	if in.Currency != nil {
		cur.Currency = strings.TrimSpace(*in.Currency)
	}
	if len(cur.Title) < 1 || len(cur.Title) > 160 {
		return store.Shoot{}, fmt.Errorf("%w: title must be 1-160 characters", ErrInvalidInput)
	}
	if cur.PriceMinor < 0 || cur.PriceMinor > 100000000 {
		return store.Shoot{}, fmt.Errorf("%w: priceMinor must be between 0 and 100000000", ErrInvalidInput)
	}
	if cur.Currency != "KES" {
		return store.Shoot{}, fmt.Errorf("%w: currency must be KES", ErrInvalidInput)
	}
	if cur.ShootDate.Valid && cur.DueDate.Valid && cur.DueDate.Time.Before(cur.ShootDate.Time) {
		return store.Shoot{}, fmt.Errorf("%w: dueDate cannot be before shootDate", ErrInvalidInput)
	}
	return cur, nil
}

func normalizeStatusFilter(csv string) (string, error) {
	csv = strings.TrimSpace(csv)
	if csv == "" {
		return "", nil
	}
	parts := strings.Split(csv, ",")
	valid := map[string]bool{"shot": true, "editing": true, "ready": true, "delivered": true, "expired": true}
	seen := map[string]bool{}
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if !valid[p] {
			return "", fmt.Errorf("%w: invalid status filter", ErrInvalidInput)
		}
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	return strings.Join(out, ","), nil
}

func normalizePage(limit, offset int) (int, int) {
	if limit <= 0 {
		limit = 25
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

func textArg(s string) pgtype.Text {
	s = strings.TrimSpace(s)
	if s == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: s, Valid: true}
}

func boolArg(b *bool) pgtype.Bool {
	if b == nil {
		return pgtype.Bool{}
	}
	return pgtype.Bool{Bool: *b, Valid: true}
}

func dateFromPtr(t *time.Time) pgtype.Date {
	if t == nil {
		return pgtype.Date{}
	}
	d := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	return pgtype.Date{Time: d, Valid: true}
}

func uuidString(id pgtype.UUID) string {
	if !id.Valid {
		return ""
	}
	b := id.Bytes
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
