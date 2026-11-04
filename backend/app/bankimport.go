package app

import (
	"net/http"
	"strings"
	"time"

	"github.com/pocketbase/pocketbase/core"
)

// bankRow is one payment picked on the web app's review screen
// (web/src/components/BankImport.svelte), already filled in by rules.
type bankRow struct {
	Ref          string   `json:"ref"`
	Date         string   `json:"date"` // YYYY-MM-DD
	Description  string   `json:"description"`
	AmountCents  int64    `json:"amount_cents"`
	PaidBy       string   `json:"paid_by"`
	SplitMode    string   `json:"split_mode"`
	PayerPercent float64  `json:"payer_percent"`
	Category     string   `json:"category"`
	Tags         []string `json:"tags"`
}

// importBank handles POST /api/splitwiz/import/bank: saves the rows as
// expenses in one go, skipping any imported before, without a notification
// for each. They count as added by the signed-in person.
func importBank(e *core.RequestEvent) error {
	var body struct {
		Rows []bankRow `json:"rows"`
	}
	if err := e.BindBody(&body); err != nil || len(body.Rows) == 0 {
		return e.BadRequestError("Pick at least one payment to import.", err)
	}
	if len(body.Rows) > 2000 {
		return e.BadRequestError("Import at most 2000 payments at a time.", nil)
	}
	for _, r := range body.Rows {
		if !strings.HasPrefix(r.Ref, "bank:") || len(r.Ref) > 64 {
			return e.BadRequestError("Each payment needs its bank: reference.", nil)
		}
	}

	done, err := importedRefs(e.App)
	if err != nil {
		return e.InternalServerError("", err)
	}

	var imported, already int
	err = e.App.RunInTransaction(func(tx core.App) error {
		imported, already, err = saveBankRows(tx, body.Rows, done, e.Auth.Id)
		return err
	})
	if err != nil {
		return e.BadRequestError("Import failed, nothing was saved: "+err.Error(), nil)
	}
	res := map[string]int{"imported": imported, "already": already}
	if imported > 0 {
		checkBudget(e.App)
	}
	return e.JSON(http.StatusOK, res)
}

// saveBankRows saves the rows not in `done` as expenses by `by`.
func saveBankRows(tx core.App, rows []bankRow, done map[string]bool, by string) (imported, already int, err error) {
	expenses, err := tx.FindCollectionByNameOrId("expenses")
	if err != nil {
		return 0, 0, err
	}
	for _, r := range rows {
		if done[r.Ref] {
			already++
			continue
		}
		day, err := time.Parse("2006-01-02", r.Date)
		if err != nil {
			return 0, 0, err
		}
		rec := core.NewRecord(expenses)
		rec.Set("description", r.Description)
		rec.Set("amount_cents", r.AmountCents)
		rec.Set("paid_by", r.PaidBy)
		rec.Set("split_mode", r.SplitMode)
		rec.Set("payer_percent", r.PayerPercent)
		rec.Set("category", r.Category)
		rec.Set("tags", r.Tags)
		rec.Set("date", day.Add(12*time.Hour)) // noon UTC, like the web app
		rec.Set("import_ref", r.Ref)
		rec.Set("created_by", by)
		rec.Set("updated_by", by)
		if err := tx.Save(rec); err != nil {
			return 0, 0, err
		}
		done[r.Ref] = true
		imported++
	}
	return imported, already, nil
}
