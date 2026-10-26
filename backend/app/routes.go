package app

import (
	"encoding/csv"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"

	"github.com/selfhostit/splitwiz/ledger"
)

type userInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type summary struct {
	Users   []userInfo       `json:"users"`
	Balance ledger.Balance   `json:"balance"`
	Net     map[string]int64 `json:"net"`
}

func registerRoutes(se *core.ServeEvent) {
	registerSetupRoutes(se)

	g := se.Router.Group("/api/splitwiz")
	g.Bind(apis.RequireAuth("users"))

	g.GET("/summary", func(e *core.RequestEvent) error {
		s, err := buildSummary(e.App)
		if err != nil {
			return e.BadRequestError(err.Error(), nil)
		}
		return e.JSON(http.StatusOK, s)
	})

	g.GET("/export.csv", func(e *core.RequestEvent) error {
		return exportCSV(e)
	})

	g.POST("/import/splitwise", importSplitwise)

	registerPushRoutes(g)
}

func loadUsers(app core.App) ([]*core.Record, error) {
	return app.FindRecordsByFilter("users", "", "created", 0, 0)
}

func buildSummary(app core.App) (*summary, error) {
	users, err := loadUsers(app)
	if err != nil {
		return nil, err
	}

	ids := make([]string, len(users))
	infos := make([]userInfo, len(users))
	for i, u := range users {
		ids[i] = u.Id
		name := u.GetString("name")
		if name == "" {
			name = u.Email()
		}
		infos[i] = userInfo{ID: u.Id, Name: name}
	}

	var expenseRows []struct {
		PaidBy    string `db:"paid_by"`
		OwedCents int64  `db:"owed_cents"`
	}
	if err := app.DB().Select("paid_by", "owed_cents").From("expenses").All(&expenseRows); err != nil {
		return nil, err
	}
	expenses := make([]ledger.Expense, len(expenseRows))
	for i, r := range expenseRows {
		expenses[i] = ledger.Expense{PaidBy: r.PaidBy, OwedCents: r.OwedCents}
	}

	var settlementRows []struct {
		From        string `db:"from_user"`
		To          string `db:"to_user"`
		AmountCents int64  `db:"amount_cents"`
	}
	if err := app.DB().Select("from_user", "to_user", "amount_cents").From("settlements").All(&settlementRows); err != nil {
		return nil, err
	}
	settlements := make([]ledger.Settlement, len(settlementRows))
	for i, r := range settlementRows {
		settlements[i] = ledger.Settlement{From: r.From, To: r.To, AmountCents: r.AmountCents}
	}

	net, err := ledger.Net(ids, expenses, settlements)
	if err != nil {
		return nil, err
	}

	return &summary{Users: infos, Balance: ledger.Summarize(net), Net: net}, nil
}

// exportCSV streams every expense and settlement as one CSV, newest first.
func exportCSV(e *core.RequestEvent) error {
	users, err := loadUsers(e.App)
	if err != nil {
		return e.InternalServerError("", err)
	}
	names := map[string]string{}
	for _, u := range users {
		names[u.Id] = u.GetString("name")
		if names[u.Id] == "" {
			names[u.Id] = u.Email()
		}
	}

	categories, err := e.App.FindAllRecords("categories")
	if err != nil {
		return e.InternalServerError("", err)
	}
	catNames := map[string]string{}
	for _, c := range categories {
		catNames[c.Id] = c.GetString("name")
	}

	expenses, err := e.App.FindRecordsByFilter("expenses", "", "-date,-created", 0, 0)
	if err != nil {
		return e.InternalServerError("", err)
	}
	settlements, err := e.App.FindRecordsByFilter("settlements", "", "-date,-created", 0, 0)
	if err != nil {
		return e.InternalServerError("", err)
	}

	// comments go in the note column, oldest first
	comments, err := e.App.FindRecordsByFilter("expense_comments", "", "created", 0, 0)
	if err != nil {
		return e.InternalServerError("", err)
	}
	notes := map[string][]string{}
	for _, c := range comments {
		id := c.GetString("expense")
		notes[id] = append(notes[id], names[c.GetString("author")]+": "+c.GetString("text"))
	}

	filename := "splitwiz-" + time.Now().Format("2006-01-02") + ".csv"
	e.Response.Header().Set("Content-Type", "text/csv; charset=utf-8")
	e.Response.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)

	w := csv.NewWriter(e.Response)
	_ = w.Write([]string{"type", "date", "description", "category", "amount", "paid_by", "split_mode", "owed_by_other", "note"})

	money := func(cents int) string {
		return strconv.FormatFloat(float64(cents)/100, 'f', 2, 64)
	}
	day := func(r *core.Record) string {
		return r.GetDateTime("date").Time().Format("2006-01-02")
	}

	for _, r := range expenses {
		_ = w.Write([]string{
			"expense", day(r), r.GetString("description"), catNames[r.GetString("category")],
			money(r.GetInt("amount_cents")), names[r.GetString("paid_by")], r.GetString("split_mode"),
			money(r.GetInt("owed_cents")), strings.Join(append(nonEmpty(r.GetString("note")), notes[r.Id]...), " | "),
		})
	}
	for _, r := range settlements {
		_ = w.Write([]string{
			"settlement", day(r), names[r.GetString("from_user")] + " paid " + names[r.GetString("to_user")], "",
			money(r.GetInt("amount_cents")), names[r.GetString("from_user")], "", "", r.GetString("note"),
		})
	}

	w.Flush()
	return w.Error()
}

func nonEmpty(s string) []string {
	if s == "" {
		return nil
	}
	return []string{s}
}
