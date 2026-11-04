package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

// Lets expenses carry tags such as "lisbon-2026", to follow a trip or an
// event across categories. A repeating expense passes its tags on.
func init() {
	m.Register(func(app core.App) error {
		for _, name := range []string{"expenses", "recurring_expenses"} {
			c, err := app.FindCollectionByNameOrId(name)
			if err != nil {
				return err
			}
			// ["lisbon-2026", ...], cleaned up on save, see ledger.NormalizeTags
			c.Fields.Add(&core.JSONField{Name: "tags", MaxSize: 2048})
			if err := app.Save(c); err != nil {
				return err
			}
		}
		return nil
	}, func(app core.App) error {
		for _, name := range []string{"expenses", "recurring_expenses"} {
			c, err := app.FindCollectionByNameOrId(name)
			if err != nil {
				return err
			}
			c.Fields.RemoveByName("tags")
			if err := app.Save(c); err != nil {
				return err
			}
		}
		return nil
	})
}
