package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
	"github.com/pocketbase/pocketbase/tools/types"
)

// Lets the app show what the other person added or changed since you last
// looked: who last saved each expense and payment, and when each person
// last marked everything as seen.
func init() {
	m.Register(func(app core.App) error {
		users, err := app.FindCollectionByNameOrId("users")
		if err != nil {
			return err
		}
		// set through POST /api/splitwiz/seen
		users.Fields.Add(&core.DateField{Name: "seen_at"})
		if err := app.Save(users); err != nil {
			return err
		}
		for _, name := range []string{"expenses", "settlements"} {
			c, err := app.FindCollectionByNameOrId(name)
			if err != nil {
				return err
			}
			// whoever last added or changed it in the app; empty for the
			// server's own saves (repeating expenses, imports)
			c.Fields.Add(&core.RelationField{Name: "updated_by", MaxSelect: 1, CollectionId: users.Id})
			if err := app.Save(c); err != nil {
				return err
			}
		}
		// what's there already counts as seen
		all, err := app.FindAllRecords("users")
		if err != nil {
			return err
		}
		for _, u := range all {
			u.Set("seen_at", types.NowDateTime())
			if err := app.SaveNoValidate(u); err != nil {
				return err
			}
		}
		return nil
	}, func(app core.App) error {
		for _, name := range []string{"expenses", "settlements"} {
			c, err := app.FindCollectionByNameOrId(name)
			if err != nil {
				return err
			}
			c.Fields.RemoveByName("updated_by")
			if err := app.Save(c); err != nil {
				return err
			}
		}
		users, err := app.FindCollectionByNameOrId("users")
		if err != nil {
			return err
		}
		users.Fields.RemoveByName("seen_at")
		return app.Save(users)
	})
}
