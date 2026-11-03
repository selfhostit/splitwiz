package app

import (
	"testing"

	"github.com/pocketbase/pocketbase/core"
)

// Bank rows become expenses with their split worked out, and importing the
// same statement again adds nothing.
func TestSaveBankRows(t *testing.T) {
	app := newTestApp(t)
	Register(app)

	users, _ := app.FindCollectionByNameOrId("users")
	alex := core.NewRecord(users)
	alex.SetEmail("alex@example.com")
	alex.SetPassword("password-1")
	if err := app.Save(alex); err != nil {
		t.Fatal(err)
	}

	rows := []bankRow{
		{Ref: "bank:1", Date: "2026-10-03", Description: "DELHAIZE", AmountCents: 5420, PaidBy: alex.Id, SplitMode: "equal", Tags: []string{"Weekly Shop"}},
		{Ref: "bank:2", Date: "2026-10-05", Description: "NETFLIX", AmountCents: 1399, PaidBy: alex.Id, SplitMode: "full"},
	}
	imported, already, err := saveBankRows(app, rows, map[string]bool{}, alex.Id)
	if err != nil || imported != 2 || already != 0 {
		t.Fatalf("got %d imported, %d already, %v", imported, already, err)
	}

	done, err := importedRefs(app)
	if err != nil {
		t.Fatal(err)
	}
	if imported, already, _ = saveBankRows(app, rows, done, alex.Id); imported != 0 || already != 2 {
		t.Fatalf("again: got %d imported, %d already", imported, already)
	}

	e, err := app.FindFirstRecordByFilter("expenses", "import_ref = 'bank:1'")
	if err != nil {
		t.Fatal(err)
	}
	if e.GetInt("owed_cents") != 2710 || e.GetString("updated_by") != alex.Id || e.GetDateTime("date").String() != "2026-10-03 12:00:00.000Z" {
		t.Errorf("owed %d, updated_by %q, date %s", e.GetInt("owed_cents"), e.GetString("updated_by"), e.GetDateTime("date"))
	}
	if got := tagsOf(t, e); len(got) != 1 || got[0] != "weekly-shop" {
		t.Errorf("tags = %v", got)
	}
}
