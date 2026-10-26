package splitwise

import (
	"os"
	"strings"
	"testing"

	"github.com/selfhostit/splitwiz/ledger"
)

const sample = "\ufeffDate,Description,Category,Cost,Currency,Ann,Bob\n" +
	"\n" +
	"2019-01-21,Delhaize ,General,18.61,EUR,-9.31,9.31\n" +
	"2019-01-26,Supermarket,Groceries,18.90,EUR,9.45,-9.45\n" +
	"2019-03-27,Monitor ,General,241.00,EUR,-100.00,100.00\n" +
	"2019-05-18,Book,General,4.00,EUR,-4.00,4.00\n" +
	"2019-05-19,Coffee,Dining out,3.00,EUR,1.50,-1.50\n" +
	"2019-05-19,Coffee,Dining out,3.00,EUR,1.50,-1.50\n" +
	"2019-02-14,Ann paid Bob,Payment,484.50,EUR,484.50,-484.50\n" +
	"\n" +
	"2026-10-17,Total balance, , ,EUR,383.64,-383.64\n"

func TestParse(t *testing.T) {
	exp, err := Parse(strings.NewReader(sample))
	if err != nil {
		t.Fatal(err)
	}
	if exp.People != [2]string{"Ann", "Bob"} {
		t.Fatalf("people = %v", exp.People)
	}
	if len(exp.Rows) != 7 {
		t.Fatalf("rows = %d", len(exp.Rows))
	}

	want := []struct {
		payer int
		mode  string
		owed  int64
	}{
		{1, ledger.ModeExact, 931}, // 18.61 split 9.30/9.31: Bob paid, Ann owes the odd cent
		{0, ledger.ModeEqual, 945},
		{1, ledger.ModeExact, 10000},
		{1, ledger.ModeFull, 400},
		{0, ledger.ModeEqual, 150},
		{0, ledger.ModeEqual, 150},
	}
	for i, w := range want {
		r := exp.Rows[i]
		if r.Settlement || r.Payer != w.payer || r.SplitMode != w.mode || r.OwedCents != w.owed {
			t.Errorf("row %d = %+v, want %+v", i, r, w)
		}
	}

	pay := exp.Rows[6]
	if !pay.Settlement || pay.From != 0 || pay.To != 1 || pay.AmountCents != 48450 {
		t.Errorf("payment = %+v", pay)
	}

	if exp.Rows[4].Ref == exp.Rows[5].Ref {
		t.Error("identical rows must get different refs")
	}
	if exp.Rows[0].Description != "Delhaize" {
		t.Errorf("description not trimmed: %q", exp.Rows[0].Description)
	}

	if exp.Net != [2]int64{38364, -38364} {
		t.Errorf("net = %v", exp.Net)
	}

	// owed amounts must survive the server's own split rules unchanged
	for _, r := range exp.Rows {
		if r.Settlement {
			continue
		}
		got, err := ledger.OwedCents(r.AmountCents, r.SplitMode, 0, r.OwedCents)
		if err != nil || got != r.OwedCents {
			t.Errorf("%s: ledger gives %d (%v), want %d", r.Description, got, err, r.OwedCents)
		}
	}
}

func TestParseRejects(t *testing.T) {
	bad := []string{
		"",
		"Date,Description,Cost\n2019-01-01,x,1\n",
		"Date,Description,Category,Cost,Currency,A,B\n",
		"Date,Description,Category,Cost,Currency,A,B\n2019-01-01,x,General,1.00,EUR,1.00,-0.50\n",
		"Date,Description,Category,Cost,Currency,A,B\n2019-01-01,x,General,1.00,EUR,2.00,-2.00\n",
		"Date,Description,Category,Cost,Currency,A,B\n2019-13-01,x,General,1.00,EUR,0.50,-0.50\n",
		"Date,Description,Category,Cost,Currency,A,B\n2019-01-01,x,General,1.005,EUR,0.50,-0.50\n",
	}
	for _, in := range bad {
		if _, err := Parse(strings.NewReader(in)); err == nil {
			t.Errorf("expected an error for %q", in)
		}
	}
}

func TestMapCategory(t *testing.T) {
	if MapCategory("Dining out").Name != "Restaurants" || MapCategory("Something new").Name != "Other" {
		t.Error("category mapping is off")
	}
}

// SPLITWISE_EXPORT=/path/to/export.csv go test -run RealExport -v ./splitwise
func TestRealExport(t *testing.T) {
	path := os.Getenv("SPLITWISE_EXPORT")
	if path == "" {
		t.Skip("set SPLITWISE_EXPORT to check a real export")
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	exp, err := Parse(f)
	if err != nil {
		t.Fatal(err)
	}
	modes := map[string]int{}
	refs := map[string]bool{}
	for _, r := range exp.Rows {
		if r.Settlement {
			modes["settlement"]++
		} else {
			modes[r.SplitMode]++
		}
		if refs[r.Ref] {
			t.Errorf("duplicate ref %s", r.Ref)
		}
		refs[r.Ref] = true
	}
	t.Logf("people %q, %d rows, %v, net %v", exp.People, len(exp.Rows), modes, exp.Net)
}
