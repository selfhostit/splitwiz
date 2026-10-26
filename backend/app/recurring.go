package app

import (
	"time"

	validation "github.com/pocketbase/ozzo-validation/v4"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/types"

	"github.com/selfhostit/splitwiz/ledger"
)

// maxCatchUp caps how many missed occurrences one run adds per series, so a
// mistaken start date years back can't flood the history.
const maxCatchUp = 400

func registerRecurring(app core.App) {
	app.OnRecordValidate("recurring_expenses").BindFunc(func(e *core.RecordEvent) error {
		if err := computeOwed(e.Record); err != nil {
			return err
		}
		if _, err := ledger.NextDate(time.Now(), e.Record.GetString("every"), 1); err != nil {
			return validation.Errors{"every": validation.NewError("invalid_every", err.Error())}
		}
		return e.Next()
	})

	// when a person sets the date, that day becomes the series' day of the month
	setAnchor := func(r *core.Record) {
		r.Set("anchor_day", r.GetDateTime("next_date").Time().Day())
	}
	app.OnRecordCreateRequest("recurring_expenses").BindFunc(func(e *core.RecordRequestEvent) error {
		setAnchor(e.Record)
		if e.Auth != nil {
			e.Record.Set("created_by", e.Auth.Id)
		}
		if err := e.Next(); err != nil {
			return err
		}
		// add today's (or any past) occurrence right away
		return addDueExpenses(e.App, e.Record.Id, authID(e))
	})
	app.OnRecordUpdateRequest("recurring_expenses").BindFunc(func(e *core.RecordRequestEvent) error {
		if !e.Record.GetDateTime("next_date").Equal(e.Record.Original().GetDateTime("next_date")) {
			setAnchor(e.Record)
		}
		if err := e.Next(); err != nil {
			return err
		}
		return addDueExpenses(e.App, e.Record.Id, authID(e))
	})

	app.OnBootstrap().BindFunc(func(e *core.BootstrapEvent) error {
		if err := e.Next(); err != nil {
			return err
		}
		e.App.Cron().MustAdd("splitwiz_recurring", "5 * * * *", func() {
			if err := addDueExpenses(e.App, "", ""); err != nil {
				e.App.Logger().Error("recurring expenses failed", "error", err)
			}
		})
		return nil
	})

	// catch up after the NAS was asleep or off
	app.OnServe().BindFunc(func(se *core.ServeEvent) error {
		if err := addDueExpenses(se.App, "", ""); err != nil {
			se.App.Logger().Error("recurring expenses failed", "error", err)
		}
		return se.Next()
	})
}

// noon UTC on the container's local calendar day, the same convention the
// web app uses so a date never shifts between time zones
func localDay(t time.Time) time.Time {
	y, m, d := t.In(time.Local).Date()
	return time.Date(y, m, d, 12, 0, 0, 0, time.UTC)
}

// addDueExpenses creates an expense for every occurrence of a recurring
// expense that is due today or earlier and moves next_date past today.
// An empty id checks every series. Everyone but `actor` (who just saved
// the series, if anyone) is told about what was added.
func addDueExpenses(app core.App, id, actor string) error {
	today := localDay(time.Now())

	filter := "paused = false && next_date <= {:today}"
	params := map[string]any{"today": today.Format(types.DefaultDateLayout)}
	if id != "" {
		filter += " && id = {:id}"
		params["id"] = id
	}
	series, err := app.FindRecordsByFilter("recurring_expenses", filter, "", 0, 0, params)
	if err != nil {
		return err
	}

	expenses, err := app.FindCollectionByNameOrId("expenses")
	if err != nil {
		return err
	}

	added := false
	for _, s := range series {
		n, lastID := 0, ""
		err := app.RunInTransaction(func(tx core.App) error {
			var err error
			next := s.GetDateTime("next_date").Time()
			for i := 0; i < maxCatchUp && !next.After(today); i++ {
				id, err := addOccurrence(tx, expenses, s, next)
				if err != nil {
					return err
				}
				if id != "" {
					n, lastID = n+1, id
				}
				next, err = ledger.NextDate(next, s.GetString("every"), s.GetInt("anchor_day"))
				if err != nil {
					return err
				}
			}
			// more missed occurrences than maxCatchUp: skip the rest
			for !next.After(today) {
				next, err = ledger.NextDate(next, s.GetString("every"), s.GetInt("anchor_day"))
				if err != nil {
					return err
				}
			}
			s.Set("next_date", next)
			return tx.Save(s)
		})
		if err != nil {
			return err
		}
		if n > 0 {
			added = true
			notifyRepeating(app, actor, s, lastID, n)
		}
	}
	if added {
		checkBudget(app)
	}
	return nil
}

// addOccurrence returns the new expense's id, or "" when that day was
// already added.
func addOccurrence(tx core.App, expenses *core.Collection, s *core.Record, day time.Time) (string, error) {
	// never add the same day twice, even if a run is repeated
	exists, err := tx.FindFirstRecordByFilter("expenses", "recurring = {:id} && date = {:day}",
		map[string]any{"id": s.Id, "day": day.Format(types.DefaultDateLayout)})
	if err == nil && exists != nil {
		return "", nil
	}

	r := core.NewRecord(expenses)
	for _, f := range []string{"description", "amount_cents", "paid_by", "split_mode", "payer_percent", "other_share_cents", "category", "note", "created_by"} {
		r.Set(f, s.Get(f))
	}
	r.Set("date", day)
	r.Set("recurring", s.Id)
	if err := tx.Save(r); err != nil {
		return "", err
	}
	return r.Id, nil
}

func authID(e *core.RecordRequestEvent) string {
	if e.Auth == nil {
		return ""
	}
	return e.Auth.Id
}
