package shoots

import (
	"testing"
	"time"

	"github.com/fiveace-Merill/photobooth/internal/shoots/store"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestOverdue(t *testing.T) {
	today := time.Date(2026, 11, 26, 0, 0, 0, 0, time.UTC)
	d := func(y int, m time.Month, day int) pgtype.Date {
		return pgtype.Date{Time: time.Date(y, m, day, 0, 0, 0, 0, time.UTC), Valid: true}
	}
	tests := []struct {
		name string
		sh   store.Shoot
		want bool
		days int
	}{
		{name: "no due date", sh: store.Shoot{Status: "shot"}, want: false, days: 0},
		{name: "due today not overdue", sh: store.Shoot{Status: "shot", DueDate: d(2026, 11, 26)}, want: false, days: 0},
		{name: "due tomorrow not overdue", sh: store.Shoot{Status: "shot", DueDate: d(2026, 11, 27)}, want: false, days: 0},
		{name: "due yesterday overdue", sh: store.Shoot{Status: "editing", DueDate: d(2026, 11, 25)}, want: true, days: 1},
		{name: "delivered not overdue", sh: store.Shoot{Status: "delivered", DueDate: d(2026, 11, 20)}, want: false, days: 0},
		{name: "expired not overdue", sh: store.Shoot{Status: "expired", DueDate: d(2026, 11, 20)}, want: false, days: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, days := Overdue(today, tt.sh)
			if got != tt.want || days != tt.days {
				t.Fatalf("Overdue()=(%v,%d), want (%v,%d)", got, days, tt.want, tt.days)
			}
		})
	}
}
