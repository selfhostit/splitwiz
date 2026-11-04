package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
	"github.com/pocketbase/pocketbase/tools/types"
)

// Rules fill in a new expense from its description: "Delhaize" is
// Groceries, split 50/50. Applied by the web app (web/src/lib/rules.ts).
func init() {
	m.Register(func(app core.App) error {
		users, err := app.FindCollectionByNameOrId("users")
		if err != nil {
			return err
		}
		categories, err := app.FindCollectionByNameOrId("categories")
		if err != nil {
			return err
		}
		rules := core.NewBaseCollection("rules")
		setMemberRules(rules)
		rules.Fields.Add(
			// matched anywhere in the description, ignoring case
			&core.TextField{Name: "match", Required: true, Max: 100},
			// each one empty = leave as it is
			&core.RelationField{Name: "category", MaxSelect: 1, CollectionId: categories.Id},
			&core.RelationField{Name: "paid_by", MaxSelect: 1, CollectionId: users.Id},
			&core.SelectField{Name: "split_mode", MaxSelect: 1, Values: []string{"equal", "percent", "full"}},
			&core.NumberField{Name: "payer_percent", Min: types.Pointer(0.0), Max: types.Pointer(100.0)},
			&core.JSONField{Name: "tags", MaxSize: 2048},
			&core.AutodateField{Name: "created", OnCreate: true},
			&core.AutodateField{Name: "updated", OnCreate: true, OnUpdate: true},
		)
		return app.Save(rules)
	}, func(app core.App) error {
		rules, err := app.FindCollectionByNameOrId("rules")
		if err != nil {
			return nil
		}
		return app.Delete(rules)
	})
}
