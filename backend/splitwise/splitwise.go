// Package splitwise reads a two-person Splitwise group export (Group >
// Export as spreadsheet) and turns each row into a Splitwiz expense or
// settlement that keeps the exact same balance.
package splitwise

import (
	"crypto/sha1"
	"encoding/csv"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math/big"
	"strings"
	"time"

	"github.com/selfhostit/splitwiz/ledger"
)

var (
	ErrHeader = errors.New("this is not a two-person Splitwise export (expected Date, Description, Category, Cost, Currency and two name columns)")
	ErrEmpty  = errors.New("the export has no entries")
)

// Row is one Splitwise line, converted to Splitwiz terms. Person indexes
// (0 or 1) refer to Export.People.
type Row struct {
	Ref         string // stable id, so importing the same file twice adds nothing
	Date        time.Time
	Description string
	Category    string // Splitwise category name
	Currency    string
	AmountCents int64

	Settlement bool
	// settlements: From paid To
	From, To int

	// expenses: Payer paid, the other person owes OwedCents
	Payer     int
	SplitMode string
	OwedCents int64
}

// Export is a parsed Splitwise file.
type Export struct {
	People [2]string
	Rows   []Row
	// Net is each person's final position in cents, positive = is owed
	Net [2]int64
}

// Parse reads a Splitwise CSV export.
func Parse(r io.Reader) (*Export, error) {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = -1

	header, err := cr.Read()
	if err != nil {
		return nil, ErrHeader
	}
	if len(header) > 0 {
		header[0] = strings.TrimPrefix(header[0], "\ufeff")
	}
	if len(header) != 7 || !strings.EqualFold(header[0], "Date") || !strings.EqualFold(header[3], "Cost") {
		return nil, ErrHeader
	}

	exp := &Export{People: [2]string{strings.TrimSpace(header[5]), strings.TrimSpace(header[6])}}
	seen := map[string]int{}

	for line := 2; ; line++ {
		rec, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", line, err)
		}
		if len(rec) < 7 || strings.TrimSpace(strings.Join(rec, "")) == "" {
			continue
		}
		desc := strings.TrimSpace(rec[1])
		if desc == "Total balance" {
			continue
		}

		row, err := parseRow(rec, desc)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", line, err)
		}

		// identical rows (two coffees on the same day) get distinct refs
		key := strings.Join(rec, "\x1f")
		seen[key]++
		sum := sha1.Sum([]byte(fmt.Sprintf("%s\x1f%d", key, seen[key])))
		row.Ref = "splitwise:" + hex.EncodeToString(sum[:10])

		exp.Rows = append(exp.Rows, row)
	}

	if len(exp.Rows) == 0 {
		return nil, ErrEmpty
	}

	for _, row := range exp.Rows {
		if row.Settlement {
			exp.Net[row.From] += row.AmountCents
			exp.Net[row.To] -= row.AmountCents
		} else {
			exp.Net[row.Payer] += row.OwedCents
			exp.Net[1-row.Payer] -= row.OwedCents
		}
	}

	return exp, nil
}

func parseRow(rec []string, desc string) (Row, error) {
	date, err := time.Parse("2006-01-02", strings.TrimSpace(rec[0]))
	if err != nil {
		return Row{}, fmt.Errorf("bad date %q", rec[0])
	}
	cost, err := cents(rec[3])
	if err != nil {
		return Row{}, err
	}
	a, err := cents(rec[5])
	if err != nil {
		return Row{}, err
	}
	b, err := cents(rec[6])
	if err != nil {
		return Row{}, err
	}
	if a+b != 0 {
		return Row{}, fmt.Errorf("the two balances don't add up to zero (%s, %s)", rec[5], rec[6])
	}

	row := Row{
		Date:        date,
		Description: desc,
		Category:    strings.TrimSpace(rec[2]),
		Currency:    strings.TrimSpace(rec[4]),
		AmountCents: cost,
	}
	if row.Description == "" {
		row.Description = "(no description)"
	}

	// Splitwise shows each person's net effect: positive means they are owed.
	// For a payment that is the person who paid; for an expense the payer.
	payer, owed := 0, a
	if a < 0 {
		payer, owed = 1, b
	}

	if row.Category == "Payment" {
		if owed == 0 {
			return Row{}, errors.New("payment without an amount")
		}
		row.Settlement = true
		row.From, row.To = payer, 1-payer
		row.AmountCents = owed
		return row, nil
	}

	if cost <= 0 {
		return Row{}, fmt.Errorf("cost must be positive, got %s", rec[3])
	}
	if owed > cost {
		return Row{}, fmt.Errorf("share %s is larger than the cost %s", rec[5], rec[3])
	}

	row.Payer, row.OwedCents = payer, owed
	switch owed {
	case cost / 2:
		row.SplitMode = ledger.ModeEqual
	case cost:
		row.SplitMode = ledger.ModeFull
	default:
		row.SplitMode = ledger.ModeExact
	}
	return row, nil
}

// cents parses "18.61" or "-9.31" exactly, without float rounding.
func cents(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	r, ok := new(big.Rat).SetString(s)
	if !ok {
		return 0, fmt.Errorf("bad amount %q", s)
	}
	r.Mul(r, big.NewRat(100, 1))
	if !r.IsInt() {
		return 0, fmt.Errorf("amount %q has more than two decimals", s)
	}
	return r.Num().Int64(), nil
}

// Category is a Splitwiz category the importer files entries under.
type Category struct{ Name, Emoji string }

// categoryMap folds Splitwise's many categories into Splitwiz's short list.
// Pets and Insurance get their own because they are common in real exports.
var categoryMap = map[string]Category{
	"Groceries":              {"Groceries", "🛒"},
	"Liquor":                 {"Groceries", "🛒"},
	"Food and drink - Other": {"Groceries", "🛒"},
	"Dining out":             {"Restaurants", "🍽️"},
	"Rent":                   {"Rent", "🏠"},
	"Mortgage":               {"Rent", "🏠"},
	"Electricity":            {"Utilities", "💡"},
	"Water":                  {"Utilities", "💡"},
	"Heat/gas":               {"Utilities", "💡"},
	"Trash":                  {"Utilities", "💡"},
	"TV/Phone/Internet":      {"Utilities", "💡"},
	"Utilities - Other":      {"Utilities", "💡"},
	"Gas/fuel":               {"Transport", "🚗"},
	"Car":                    {"Transport", "🚗"},
	"Parking":                {"Transport", "🚗"},
	"Bus/train":              {"Transport", "🚗"},
	"Taxi":                   {"Transport", "🚗"},
	"Bicycle":                {"Transport", "🚗"},
	"Transportation - Other": {"Transport", "🚗"},
	"Hotel":                  {"Travel", "✈️"},
	"Plane":                  {"Travel", "✈️"},
	"Movies":                 {"Entertainment", "🎬"},
	"Music":                  {"Entertainment", "🎬"},
	"Games":                  {"Entertainment", "🎬"},
	"Sports":                 {"Entertainment", "🎬"},
	"Entertainment - Other":  {"Entertainment", "🎬"},
	"Medical expenses":       {"Health", "💊"},
	"Clothing":               {"Shopping", "🛍️"},
	"Electronics":            {"Shopping", "🛍️"},
	"Furniture":              {"Shopping", "🛍️"},
	"Household supplies":     {"Shopping", "🛍️"},
	"Gifts":                  {"Shopping", "🛍️"},
	"Pets":                   {"Pets", "🐾"},
	"Insurance":              {"Insurance", "🛡️"},
}

// MapCategory returns the Splitwiz category for a Splitwise one.
func MapCategory(name string) Category {
	if c, ok := categoryMap[name]; ok {
		return c
	}
	return Category{"Other", "📦"}
}
