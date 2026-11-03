package app

import (
	"reflect"
	"testing"

	"github.com/pocketbase/pocketbase/core"
)

// Tags are cleaned up on save, and a repeating expense passes them on to
// every expense it adds.
func TestTagsNormalizedAndRepeated(t *testing.T) {
	app := newTestApp(t)
	registerRecurring(app)
	registerTags(app)

	users, _ := app.FindCollectionByNameOrId("users")
	alex := core.NewRecord(users)
	alex.SetEmail("alex@example.com")
	alex.SetPassword("password-1")
	if err := app.Save(alex); err != nil {
		t.Fatal(err)
	}

	recurring, _ := app.FindCollectionByNameOrId("recurring_expenses")
	s := core.NewRecord(recurring)
	s.Set("description", "Holiday flat")
	s.Set("amount_cents", 50000)
	s.Set("paid_by", alex.Id)
	s.Set("split_mode", "equal")
	s.Set("every", "month")
	s.Set("anchor_day", 1)
	s.Set("next_date", "2025-01-01 12:00:00.000Z")
	s.Set("ends_on", "2025-01-31 12:00:00.000Z")
	s.Set("tags", []string{"#Lisbon 2026", "lisbon-2026", " "})
	if err := app.Save(s); err != nil {
		t.Fatal(err)
	}
	want := []string{"lisbon-2026"}
	if got := tagsOf(t, s); !reflect.DeepEqual(got, want) {
		t.Fatalf("series tags = %v, want %v", got, want)
	}

	if err := addDueExpenses(app, "", ""); err != nil {
		t.Fatal(err)
	}
	added, err := app.FindRecordsByFilter("expenses", "recurring = {:id}", "date", 0, 0, map[string]any{"id": s.Id})
	if err != nil || len(added) != 1 {
		t.Fatalf("added %d expenses (%v), want 1", len(added), err)
	}
	if got := tagsOf(t, added[0]); !reflect.DeepEqual(got, want) {
		t.Errorf("expense tags = %v, want %v", got, want)
	}
}

func tagsOf(t *testing.T, r *core.Record) []string {
	t.Helper()
	var tags []string
	if err := r.UnmarshalJSONField("tags", &tags); err != nil {
		t.Fatal(err)
	}
	return tags
}
