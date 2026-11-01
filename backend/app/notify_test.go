package app

import (
	"testing"
	"time"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/types"
)

func TestCategoryBudgetAlert(t *testing.T) {
	app := newTestApp(t)
	users, _ := app.FindCollectionByNameOrId("users")
	alex := core.NewRecord(users)
	alex.SetEmail("alex@example.com")
	alex.SetPassword("password-1")
	if err := app.Save(alex); err != nil {
		t.Fatal(err)
	}
	groceries, err := app.FindFirstRecordByFilter("categories", "name = 'Groceries'")
	if err != nil {
		t.Fatal(err)
	}
	groceries.Set("monthly_cents", 10000)
	if err := app.Save(groceries); err != nil {
		t.Fatal(err)
	}

	now := time.Now().In(time.Local)
	add := func(cents int, day time.Time) {
		t.Helper()
		expenses, _ := app.FindCollectionByNameOrId("expenses")
		e := core.NewRecord(expenses)
		e.Set("description", "Shop")
		e.Set("amount_cents", cents)
		e.Set("paid_by", alex.Id)
		e.Set("split_mode", "equal")
		e.Set("category", groceries.Id)
		e.Set("date", day.Format("2006-01-02")+" 12:00:00.000Z")
		if err := app.Save(e); err != nil {
			t.Fatal(err)
		}
	}
	add(5000, now)
	// last month doesn't count. Step back from the 1st: on the 31st,
	// AddDate(0, -1, 0) lands on the 1st of this month.
	firstOfMonth := time.Date(now.Year(), now.Month(), 1, 12, 0, 0, 0, now.Location())
	add(9000, firstOfMonth.AddDate(0, 0, -1))

	if spent, err := categorySpent(app, groceries.Id, now); err != nil || spent != 5000 {
		t.Fatalf("spent this month = %d, %v; want 5000", spent, err)
	}

	checkCategoryBudget(app, groceries.Id)
	groceries, _ = app.FindRecordById("categories", groceries.Id)
	if groceries.GetString("alerted_month") != "" {
		t.Fatal("half the limit should not alert")
	}

	add(4000, now)
	checkCategoryBudget(app, groceries.Id)
	groceries, _ = app.FindRecordById("categories", groceries.Id)
	if got := groceries.GetString("alerted_month"); got != now.Format("2006-01") {
		t.Fatalf("90%% of the limit should alert once, alerted_month = %q", got)
	}
}

// Deleting a category keeps its expenses, without a category.
func TestDeleteCategoryKeepsExpenses(t *testing.T) {
	app := newTestApp(t)
	users, _ := app.FindCollectionByNameOrId("users")
	alex := core.NewRecord(users)
	alex.SetEmail("alex@example.com")
	alex.SetPassword("password-1")
	if err := app.Save(alex); err != nil {
		t.Fatal(err)
	}
	other, _ := app.FindFirstRecordByFilter("categories", "name = 'Other'")
	expenses, _ := app.FindCollectionByNameOrId("expenses")
	e := core.NewRecord(expenses)
	e.Set("description", "Thing")
	e.Set("amount_cents", 100)
	e.Set("paid_by", alex.Id)
	e.Set("split_mode", "equal")
	e.Set("category", other.Id)
	e.Set("date", types.NowDateTime())
	if err := app.Save(e); err != nil {
		t.Fatal(err)
	}
	if err := app.Delete(other); err != nil {
		t.Fatal(err)
	}
	e, err := app.FindRecordById("expenses", e.Id)
	if err != nil || e.GetString("category") != "" {
		t.Fatalf("expense should stay without a category: %v %q", err, e.GetString("category"))
	}
}
