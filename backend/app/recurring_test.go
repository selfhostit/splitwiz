package app

import (
	"testing"

	"github.com/pocketbase/pocketbase/core"

	"github.com/selfhostit/splitwiz/ledger"
)

// A series that changes price and ends adds each expense at the right
// amount, stops after its last day, and keeps the new price afterwards.
func TestRecurringChangesAndEnd(t *testing.T) {
	app := newTestApp(t)
	registerRecurring(app)

	users, _ := app.FindCollectionByNameOrId("users")
	alex := core.NewRecord(users)
	alex.SetEmail("alex@example.com")
	alex.SetPassword("password-1")
	if err := app.Save(alex); err != nil {
		t.Fatal(err)
	}

	recurring, _ := app.FindCollectionByNameOrId("recurring_expenses")
	s := core.NewRecord(recurring)
	s.Set("description", "Internet")
	s.Set("amount_cents", 8000)
	s.Set("paid_by", alex.Id)
	s.Set("split_mode", "exact")
	s.Set("other_share_cents", 2000)
	s.Set("every", "month")
	s.Set("anchor_day", 10)
	s.Set("next_date", "2025-01-10 12:00:00.000Z")
	s.Set("ends_on", "2025-04-30 12:00:00.000Z")
	s.Set("changes", []ledger.AmountChange{{From: "2025-03-01", Cents: 9000}})
	if err := app.Save(s); err != nil {
		t.Fatal(err)
	}

	if err := addDueExpenses(app, "", ""); err != nil {
		t.Fatal(err)
	}
	added, err := app.FindRecordsByFilter("expenses", "recurring = {:id}", "date", 0, 0, map[string]any{"id": s.Id})
	if err != nil {
		t.Fatal(err)
	}
	want := []struct {
		day           string
		amount, share int
	}{
		{"2025-01-10", 8000, 2000},
		{"2025-02-10", 8000, 2000},
		{"2025-03-10", 9000, 2250},
		{"2025-04-10", 9000, 2250},
	}
	if len(added) != len(want) {
		t.Fatalf("added %d expenses, want %d", len(added), len(want))
	}
	for i, w := range want {
		e := added[i]
		if day := dayKey(e.GetDateTime("date").Time()); day != w.day || e.GetInt("amount_cents") != w.amount || e.GetInt("other_share_cents") != w.share {
			t.Errorf("expense %d = %s %d (share %d), want %s %d (share %d)", i, day, e.GetInt("amount_cents"), e.GetInt("other_share_cents"), w.day, w.amount, w.share)
		}
	}

	s, _ = app.FindRecordById("recurring_expenses", s.Id)
	if s.GetInt("amount_cents") != 9000 || s.GetInt("other_share_cents") != 2250 || len(seriesChanges(s)) != 0 {
		t.Errorf("series should now show the new price, got %d / %d with %v", s.GetInt("amount_cents"), s.GetInt("other_share_cents"), seriesChanges(s))
	}
	if !ended(s, s.GetDateTime("next_date").Time()) {
		t.Error("series should have ended")
	}

	// running again adds nothing
	if err := addDueExpenses(app, "", ""); err != nil {
		t.Fatal(err)
	}
	if n, _ := app.CountRecords("expenses"); n != 4 {
		t.Errorf("%d expenses after a second run, want 4", n)
	}
}

func TestRecurringRejectsBadChanges(t *testing.T) {
	app := newTestApp(t)
	registerRecurring(app)
	users, _ := app.FindCollectionByNameOrId("users")
	alex := core.NewRecord(users)
	alex.SetEmail("alex@example.com")
	alex.SetPassword("password-1")
	if err := app.Save(alex); err != nil {
		t.Fatal(err)
	}

	recurring, _ := app.FindCollectionByNameOrId("recurring_expenses")
	for _, bad := range []any{
		[]ledger.AmountChange{{From: "next month", Cents: 9000}},
		[]ledger.AmountChange{{From: "2099-01-01", Cents: 0}},
		"not a list",
	} {
		s := core.NewRecord(recurring)
		s.Set("description", "Gym")
		s.Set("amount_cents", 3000)
		s.Set("paid_by", alex.Id)
		s.Set("split_mode", "equal")
		s.Set("every", "month")
		s.Set("next_date", "2099-01-10 12:00:00.000Z")
		s.Set("changes", bad)
		if err := app.Save(s); err == nil {
			t.Errorf("changes %v should be rejected", bad)
		}
	}
}
