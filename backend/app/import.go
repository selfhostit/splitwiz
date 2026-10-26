package app

import (
	"net/http"
	"strings"
	"time"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"

	"github.com/selfhostit/splitwiz/ledger"
	"github.com/selfhostit/splitwiz/splitwise"
)

var nonEmptyRef = dbx.NewExp("import_ref != ''")

type importResult struct {
	People      [2]string        `json:"people"`
	Users       [2]string        `json:"users"` // Splitwiz user id for each Splitwise person
	Expenses    int              `json:"expenses"`
	Settlements int              `json:"settlements"`
	First       string           `json:"first"`
	Last        string           `json:"last"`
	Already     int              `json:"already"` // rows imported before, skipped
	Net         map[string]int64 `json:"net"`     // balance the export ends on, by user id
	Categories  map[string]int   `json:"categories"`
	Imported    int              `json:"imported"`
}

// importSplitwise handles POST /api/splitwiz/import/splitwise with a
// multipart "file". Without "confirm=1" it only previews. "user0"/"user1"
// pick the Splitwiz user for each Splitwise person; by default they are
// matched by name.
func importSplitwise(e *core.RequestEvent) error {
	file, _, err := e.Request.FormFile("file")
	if err != nil {
		return e.BadRequestError("Choose the Splitwise CSV export.", err)
	}
	defer file.Close()

	exp, err := splitwise.Parse(file)
	if err != nil {
		return e.BadRequestError(err.Error(), nil)
	}

	users, err := loadUsers(e.App)
	if err != nil {
		return e.InternalServerError("", err)
	}
	if len(users) != 2 {
		return e.BadRequestError("Create both Splitwiz accounts before importing.", nil)
	}

	ids := matchPeople(exp.People, users)
	if u0, u1 := e.Request.FormValue("user0"), e.Request.FormValue("user1"); u0 != "" || u1 != "" {
		valid := map[string]bool{users[0].Id: true, users[1].Id: true}
		if !valid[u0] || !valid[u1] || u0 == u1 {
			return e.BadRequestError("Pick a different Splitwiz person for each Splitwise name.", nil)
		}
		ids = [2]string{u0, u1}
	}

	done, err := importedRefs(e.App)
	if err != nil {
		return e.InternalServerError("", err)
	}

	res := importResult{
		People:     exp.People,
		Users:      ids,
		Net:        map[string]int64{ids[0]: exp.Net[0], ids[1]: exp.Net[1]},
		Categories: map[string]int{},
	}
	for _, r := range exp.Rows {
		if done[r.Ref] {
			res.Already++
			continue
		}
		if r.Settlement {
			res.Settlements++
		} else {
			res.Expenses++
			res.Categories[splitwise.MapCategory(r.Category).Name]++
		}
		day := r.Date.Format("2006-01-02")
		if res.First == "" || day < res.First {
			res.First = day
		}
		if day > res.Last {
			res.Last = day
		}
	}

	if e.Request.FormValue("confirm") != "1" {
		return e.JSON(http.StatusOK, res)
	}

	err = e.App.RunInTransaction(func(tx core.App) error {
		n, err := saveImport(tx, exp, ids, done, e.Auth)
		res.Imported = n
		return err
	})
	if err != nil {
		return e.BadRequestError("Import failed, nothing was saved: "+err.Error(), nil)
	}
	return e.JSON(http.StatusOK, res)
}

// matchPeople pairs Splitwise names with Splitwiz users by first name or
// email, falling back to account order.
func matchPeople(people [2]string, users []*core.Record) [2]string {
	words := func(s string) []string {
		return strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
			return !(r >= 'a' && r <= 'z' || r > 127)
		})
	}
	matches := func(person string, u *core.Record) bool {
		p := strings.ToLower(person)
		for _, w := range append(words(u.GetString("name")), words(strings.Split(u.Email(), "@")[0])...) {
			if len(w) >= 3 && strings.Contains(p, w) {
				return true
			}
		}
		return false
	}

	a, b := users[0].Id, users[1].Id
	if (matches(people[0], users[1]) && !matches(people[0], users[0])) ||
		(matches(people[1], users[0]) && !matches(people[1], users[1])) {
		a, b = b, a
	}
	return [2]string{a, b}
}

func importedRefs(app core.App) (map[string]bool, error) {
	done := map[string]bool{}
	for _, c := range []string{"expenses", "settlements"} {
		var refs []string
		err := app.DB().Select("import_ref").From(c).Where(nonEmptyRef).Column(&refs)
		if err != nil {
			return nil, err
		}
		for _, r := range refs {
			done[r] = true
		}
	}
	return done, nil
}

func saveImport(tx core.App, exp *splitwise.Export, ids [2]string, done map[string]bool, auth *core.Record) (int, error) {
	expenses, err := tx.FindCollectionByNameOrId("expenses")
	if err != nil {
		return 0, err
	}
	settlements, err := tx.FindCollectionByNameOrId("settlements")
	if err != nil {
		return 0, err
	}

	cats, err := categoryIDs(tx)
	if err != nil {
		return 0, err
	}

	createdBy := ""
	if auth != nil {
		createdBy = auth.Id
	}

	n := 0
	for _, r := range exp.Rows {
		if done[r.Ref] {
			continue
		}
		note := ""
		if r.Currency != "" && r.Currency != "EUR" {
			note = "Amount in " + r.Currency + " (imported from Splitwise)"
		}
		day := r.Date.Add(12 * time.Hour) // noon UTC, like the web app

		var rec *core.Record
		if r.Settlement {
			rec = core.NewRecord(settlements)
			rec.Set("from_user", ids[r.From])
			rec.Set("to_user", ids[r.To])
		} else {
			catID, err := cats.get(tx, splitwise.MapCategory(r.Category))
			if err != nil {
				return 0, err
			}
			rec = core.NewRecord(expenses)
			rec.Set("description", truncate(r.Description, 200))
			rec.Set("paid_by", ids[r.Payer])
			rec.Set("split_mode", r.SplitMode)
			if r.SplitMode == ledger.ModeExact {
				rec.Set("other_share_cents", r.OwedCents)
			}
			rec.Set("category", catID)
		}
		rec.Set("amount_cents", r.AmountCents)
		rec.Set("date", day)
		rec.Set("note", note)
		rec.Set("import_ref", r.Ref)
		rec.Set("created_by", createdBy)
		if err := tx.Save(rec); err != nil {
			return 0, err
		}
		done[r.Ref] = true
		n++
	}
	return n, nil
}

type categoryCache struct {
	ids  map[string]string // lower-case name -> id
	next int
}

func categoryIDs(app core.App) (*categoryCache, error) {
	all, err := app.FindAllRecords("categories")
	if err != nil {
		return nil, err
	}
	c := &categoryCache{ids: map[string]string{}}
	for _, r := range all {
		c.ids[strings.ToLower(r.GetString("name"))] = r.Id
		if s := r.GetInt("sort"); s >= c.next {
			c.next = s + 1
		}
	}
	return c, nil
}

// get returns the category's id, creating it when it doesn't exist yet
func (c *categoryCache) get(tx core.App, cat splitwise.Category) (string, error) {
	if id, ok := c.ids[strings.ToLower(cat.Name)]; ok {
		return id, nil
	}
	col, err := tx.FindCollectionByNameOrId("categories")
	if err != nil {
		return "", err
	}
	r := core.NewRecord(col)
	r.Set("name", cat.Name)
	r.Set("emoji", cat.Emoji)
	r.Set("sort", c.next)
	if err := tx.Save(r); err != nil {
		return "", err
	}
	c.next++
	c.ids[strings.ToLower(cat.Name)] = r.Id
	return r.Id, nil
}

func truncate(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max])
}
