package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

// Lets a repeating expense stop on a date and change its amount over time
// ("€80 until February, then €95").
func init() {
	m.Register(func(app core.App) error {
		recurring, err := app.FindCollectionByNameOrId("recurring_expenses")
		if err != nil {
			return err
		}
		recurring.Fields.Add(
			// last day an expense may be added; empty repeats forever
			&core.DateField{Name: "ends_on"},
			// [{from: "YYYY-MM-DD", amount_cents}], see ledger.AmountChange
			&core.JSONField{Name: "changes", MaxSize: 8192},
		)
		return app.Save(recurring)
	}, func(app core.App) error {
		recurring, err := app.FindCollectionByNameOrId("recurring_expenses")
		if err != nil {
			return err
		}
		recurring.Fields.RemoveByName("ends_on")
		recurring.Fields.RemoveByName("changes")
		return app.Save(recurring)
	})
}
