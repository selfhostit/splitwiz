package ledger

import (
	"reflect"
	"testing"
)

func TestNormalizeTag(t *testing.T) {
	for in, want := range map[string]string{
		"Lisbon":                               "lisbon",
		"#Lisbon 2026":                         "lisbon-2026",
		"  ##road  trip! ":                     "road-trip",
		"wedding_anna/tom":                     "wedding-anna-tom",
		"Café-Crème":                           "café-crème",
		"---":                                  "",
		"":                                     "",
		"abcdefghijklmnopqrstuvwxyz0123456789": "abcdefghijklmnopqrstuvwxyz012345",
	} {
		if got := NormalizeTag(in); got != want {
			t.Errorf("NormalizeTag(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNormalizeTags(t *testing.T) {
	got := NormalizeTags([]string{"Lisbon", "#lisbon", "", "Food & drinks"})
	if want := []string{"lisbon", "food-drinks"}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	many := make([]string, 0, 15)
	for i := 0; i < 15; i++ {
		many = append(many, string(rune('a'+i)))
	}
	if n := len(NormalizeTags(many)); n != MaxTags {
		t.Errorf("kept %d tags, want %d", n, MaxTags)
	}
	if got := NormalizeTags(nil); got == nil || len(got) != 0 {
		t.Errorf("NormalizeTags(nil) = %#v, want an empty list", got)
	}
}
