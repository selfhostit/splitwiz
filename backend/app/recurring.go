package app

import (
	"sort"
	"time"

	validation "github.com/pocketbase/ozzo-validation/v4"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/types"

	"github.com/selfhostit/splitwiz/ledger"
)

// maxChanges caps how many scheduled amount changes one series can have.
const maxChanges = 36

// maxCatchUp caps how many missed occurrences one run adds per series, so a
// mistaken start date years back can't flood the history.
const maxCatchUp = 400

func registerRecurring(app core.App) {
	app.OnRecordValidate("recurring_expenses").BindFunc(func(e *core.RecordEvent) error {
		if err := applyChanges(e.Record); err != nil {
			return err
		}
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

	// a series past its end date has nothing left to add
	filter := "paused = false && next_date <= {:today} && (ends_on = '' || ends_on >= next_date)"
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
		n, lastID, lastCents := 0, "", int64(0)
		err := app.RunInTransaction(func(tx core.App) error {
			var err error
			next := s.GetDateTime("next_date").Time()
			for i := 0; i < maxCatchUp && !next.After(today); i++ {
				if ended(s, next) {
					break
				}
				id, cents, err := addOccurrence(tx, expenses, s, next)
				if err != nil {
					return err
				}
				if id != "" {
					n, lastID, lastCents = n+1, id, cents
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
			notifyRepeating(app, actor, s, lastID, lastCents, n)
		}
	}
	if added {
		checkBudget(app)
	}
	return nil
}

// addOccurrence returns the new expense's id and amount, or "" when that
// day was already added.
func addOccurrence(tx core.App, expenses *core.Collection, s *core.Record, day time.Time) (string, int64, error) {
	// never add the same day twice, even if a run is repeated
	exists, err := tx.FindFirstRecordByFilter("expenses", "recurring = {:id} && date = {:day}",
		map[string]any{"id": s.Id, "day": day.Format(types.DefaultDateLayout)})
	if err == nil && exists != nil {
		return "", 0, nil
	}

	r := core.NewRecord(expenses)
	for _, f := range []string{"description", "paid_by", "split_mode", "payer_percent", "category", "note", "created_by"} {
		r.Set(f, s.Get(f))
	}
	base := int64(s.GetInt("amount_cents"))
	amount := ledger.AmountOn(base, seriesChanges(s), dayKey(day))
	r.Set("amount_cents", amount)
	r.Set("other_share_cents", ledger.ScaleShare(int64(s.GetInt("other_share_cents")), base, amount))
	r.Set("date", day)
	r.Set("recurring", s.Id)
	if err := tx.Save(r); err != nil {
		return "", 0, err
	}
	return r.Id, amount, nil
}

func dayKey(t time.Time) string {
	return t.Format("2006-01-02")
}

// ended reports whether `day` is after the series' last day, if it has one.
func ended(s *core.Record, day time.Time) bool {
	end := s.GetDateTime("ends_on")
	return !end.IsZero() && dayKey(day) > dayKey(end.Time())
}

// seriesChanges reads the scheduled amount changes; a malformed value
// counts as none.
func seriesChanges(s *core.Record) []ledger.AmountChange {
	var changes []ledger.AmountChange
	_ = s.UnmarshalJSONField("changes", &changes)
	return changes
}

// applyChanges checks the scheduled amount changes, sorts them, and moves
// the ones that have started by next_date into amount_cents, so the series
// always shows what its next expense will cost.
func applyChanges(r *core.Record) error {
	var changes []ledger.AmountChange
	if err := r.UnmarshalJSONField("changes", &changes); err != nil {
		return validation.Errors{"changes": validation.NewError("invalid_changes", "Amount changes must be a list of dates and amounts.")}
	}
	if len(changes) > maxChanges {
		return validation.Errors{"changes": validation.NewError("too_many_changes", "That's too many amount changes.")}
	}
	byDay := map[string]int64{}
	for _, c := range changes {
		if _, err := time.Parse("2006-01-02", c.From); err != nil || c.Cents < 1 {
			return validation.Errors{"changes": validation.NewError("invalid_change", "Each change needs a date and an amount above zero.")}
		}
		// the same day twice: the last one wins
		byDay[c.From] = c.Cents
	}

	next := dayKey(r.GetDateTime("next_date").Time())
	base := int64(r.GetInt("amount_cents"))
	future := []ledger.AmountChange{}
	for from, cents := range byDay {
		if from > next {
			future = append(future, ledger.AmountChange{From: from, Cents: cents})
		}
	}
	sort.Slice(future, func(i, j int) bool { return future[i].From < future[j].From })

	if amount := ledger.AmountOn(base, changes, next); amount != base {
		r.Set("amount_cents", amount)
		r.Set("other_share_cents", ledger.ScaleShare(int64(r.GetInt("other_share_cents")), base, amount))
	}
	r.Set("changes", future)
	return nil
}

func authID(e *core.RecordRequestEvent) string {
	if e.Auth == nil {
		return ""
	}
	return e.Auth.Id
}
