package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/types"

	"github.com/selfhostit/splitwiz/ledger"
)

// What the other person hears about: expenses they didn't add, comments,
// repeating expenses as they are added, and the month heading over budget.
// Only changes made in the app notify; imports and the server's own saves
// stay quiet.
func registerNotifications(app core.App) {
	app.OnRecordCreateRequest("expenses").BindFunc(func(e *core.RecordRequestEvent) error {
		// the note becomes the first comment (comments.go); mention it here
		note := strings.TrimSpace(e.Record.GetString("note"))
		if err := e.Next(); err != nil {
			return err
		}
		notifyExpense(e.App, e.Auth, e.Record, "added", note)
		checkBudget(e.App)
		checkCategoryBudget(e.App, e.Record.GetString("category"))
		return nil
	})

	app.OnRecordUpdateRequest("expenses").BindFunc(func(e *core.RecordRequestEvent) error {
		before := e.Record.Original()
		if err := e.Next(); err != nil {
			return err
		}
		if changedFields(before, e.Record, "description", "amount_cents", "paid_by", "owed_cents", "date") {
			notifyExpense(e.App, e.Auth, e.Record, "changed", "")
		}
		checkBudget(e.App)
		checkCategoryBudget(e.App, e.Record.GetString("category"))
		return nil
	})

	// raising the budget can make the alert due again this month
	app.OnRecordUpdateRequest("budget").BindFunc(func(e *core.RecordRequestEvent) error {
		if e.Record.GetInt("monthly_cents") != e.Record.Original().GetInt("monthly_cents") {
			e.Record.Set("alerted_month", "")
		}
		if err := e.Next(); err != nil {
			return err
		}
		checkBudget(e.App)
		return nil
	})

	// same for a category's own limit
	app.OnRecordUpdateRequest("categories").BindFunc(func(e *core.RecordRequestEvent) error {
		if e.Record.GetInt("monthly_cents") != e.Record.Original().GetInt("monthly_cents") {
			e.Record.Set("alerted_month", "")
		}
		if err := e.Next(); err != nil {
			return err
		}
		checkCategoryBudget(e.App, e.Record.Id)
		return nil
	})
}

func changedFields(a, b *core.Record, fields ...string) bool {
	for _, f := range fields {
		if fmt.Sprint(a.Get(f)) != fmt.Sprint(b.Get(f)) {
			return true
		}
	}
	return false
}

func displayName(u *core.Record) string {
	if u == nil {
		return "Someone"
	}
	if n := u.GetString("name"); n != "" {
		return n
	}
	return strings.Split(u.Email(), "@")[0]
}

func formatEuro(cents int64) string {
	return fmt.Sprintf("€%d.%02d", cents/100, cents%100)
}

func expenseURL(id string) string {
	return "/?expense=" + id
}

func notifyExpense(app core.App, actor *core.Record, r *core.Record, verb, note string) {
	if actor == nil {
		return
	}
	amount := int64(r.GetInt("amount_cents"))
	owed := int64(r.GetInt("owed_cents"))
	notifyOthers(app, actor.Id, func(to string) notification {
		body := r.GetString("description") + " · " + formatEuro(amount)
		switch {
		case owed == 0:
		case r.GetString("paid_by") == to:
			body += ", you lent " + formatEuro(owed)
		default:
			body += ", you owe " + formatEuro(owed)
		}
		if note != "" {
			body += "\n💬 " + note
		}
		return notification{
			Title: displayName(actor) + " " + verb + " an expense",
			Body:  body,
			URL:   expenseURL(r.Id),
			Tag:   "expense-" + r.Id,
		}
	})
}

// notifyRepeating tells both people (minus whoever just saved the series)
// that a repeating expense was added; n > 1 when missed days were caught up.
func notifyRepeating(app core.App, actor string, s *core.Record, lastID string, n int) {
	body := s.GetString("description") + " · " + formatEuro(int64(s.GetInt("amount_cents")))
	if n > 1 {
		body += fmt.Sprintf(" (%d times, catching up)", n)
	}
	notifyOthers(app, actor, func(string) notification {
		return notification{
			Title: "Repeating expense added",
			Body:  body,
			URL:   expenseURL(lastID),
			Tag:   "expense-" + lastID,
		}
	})
}

// checkBudget sends one alert a month, to both people, the first time the
// month is projected (or already is) over the shared budget.
func checkBudget(app core.App) {
	budget, err := app.FindFirstRecordByFilter("budget", "")
	if err != nil {
		return
	}
	limit := int64(budget.GetInt("monthly_cents"))
	now := time.Now().In(time.Local)
	month := now.Format("2006-01")
	if limit <= 0 || budget.GetString("alerted_month") == month {
		return
	}

	spent, projected, err := projectMonth(app, now)
	if err != nil {
		app.Logger().Error("budget projection failed", "error", err)
		return
	}
	if projected <= limit {
		return
	}

	budget.Set("alerted_month", month)
	if err := app.Save(budget); err != nil {
		return
	}

	n := notification{
		Title: "Heading over budget",
		Body:  fmt.Sprintf("On track for %s this month, the budget is %s.", formatEuro(projected), formatEuro(limit)),
		URL:   "/?tab=stats",
		Tag:   "budget",
	}
	if spent > limit {
		n.Title = "Over budget"
		n.Body = fmt.Sprintf("%s spent this month, the budget is %s.", formatEuro(spent), formatEuro(limit))
	}
	notifyOthers(app, "", func(string) notification { return n })
}

// categoryAlertShare is how much of a category's limit can be spent
// before both people hear about it.
const categoryAlertShare = 0.9

// checkCategoryBudget sends one alert a month, to both people, the first
// time a category's spending this month reaches 90% of its limit.
func checkCategoryBudget(app core.App, categoryID string) {
	if categoryID == "" {
		return
	}
	category, err := app.FindRecordById("categories", categoryID)
	if err != nil {
		return
	}
	limit := int64(category.GetInt("monthly_cents"))
	now := time.Now().In(time.Local)
	month := now.Format("2006-01")
	if limit <= 0 || category.GetString("alerted_month") == month {
		return
	}

	spent, err := categorySpent(app, categoryID, now)
	if err != nil {
		app.Logger().Error("category spending failed", "error", err)
		return
	}
	if float64(spent) < categoryAlertShare*float64(limit) {
		return
	}

	category.Set("alerted_month", month)
	if err := app.Save(category); err != nil {
		return
	}

	name := strings.TrimSpace(category.GetString("emoji") + " " + category.GetString("name"))
	n := notification{
		Title: name + " almost at its limit",
		Body:  fmt.Sprintf("%s spent this month, the limit is %s.", formatEuro(spent), formatEuro(limit)),
		URL:   "/?tab=stats",
		Tag:   "category-" + categoryID,
	}
	if spent > limit {
		n.Title = name + " over its limit"
	}
	notifyOthers(app, "", func(string) notification { return n })
}

// categorySpent adds up this month's expenses in one category.
func categorySpent(app core.App, categoryID string, now time.Time) (int64, error) {
	y, m, _ := now.Date()
	first := time.Date(y, m, 1, 0, 0, 0, 0, time.UTC)
	next := first.AddDate(0, 1, 0)
	var row struct {
		Total int64 `db:"total"`
	}
	err := app.DB().
		Select("COALESCE(SUM(amount_cents), 0) AS total").
		From("expenses").
		Where(dbx.HashExp{"category": categoryID}).
		AndWhere(dbx.NewExp("date >= {:from} AND date < {:to}", dbx.Params{
			"from": first.Format(types.DefaultDateLayout),
			"to":   next.Format(types.DefaultDateLayout),
		})).
		One(&row)
	return row.Total, err
}

// projectMonth gathers this month's expenses and the repeating charges
// still to come, for ledger.ProjectMonth.
func projectMonth(app core.App, now time.Time) (spent, projected int64, err error) {
	y, m, d := now.Date()
	first := time.Date(y, m, 1, 0, 0, 0, 0, time.UTC)
	days := first.AddDate(0, 1, -1).Day()
	endOfToday := time.Date(y, m, d, 23, 59, 59, 0, time.UTC)

	expenses, err := app.FindRecordsByFilter("expenses", "date >= {:from} && date <= {:to}", "", 0, 0, map[string]any{
		"from": first.Format(types.DefaultDateLayout),
		"to":   endOfToday.Format(types.DefaultDateLayout),
	})
	if err != nil {
		return 0, 0, err
	}
	var done []ledger.DayAmount
	for _, e := range expenses {
		done = append(done, ledger.DayAmount{
			Day:       e.GetDateTime("date").Time().Day(),
			Cents:     int64(e.GetInt("amount_cents")),
			Recurring: e.GetString("recurring") != "",
		})
	}

	series, err := app.FindRecordsByFilter("recurring_expenses", "paused = false", "", 0, 0)
	if err != nil {
		return 0, 0, err
	}
	today := time.Date(y, m, d, 12, 0, 0, 0, time.UTC)
	lastDay := time.Date(y, m, days, 12, 0, 0, 0, time.UTC)
	var scheduled []ledger.DayAmount
	for _, s := range series {
		day := s.GetDateTime("next_date").Time()
		for i := 0; i < maxCatchUp && !day.After(lastDay); i++ {
			if !day.Before(today) {
				scheduled = append(scheduled, ledger.DayAmount{Day: day.Day(), Cents: int64(s.GetInt("amount_cents"))})
			}
			if day, err = ledger.NextDate(day, s.GetString("every"), s.GetInt("anchor_day")); err != nil {
				break
			}
		}
	}

	spent, projected = ledger.ProjectMonth(days, d, done, scheduled)
	return spent, projected, nil
}
