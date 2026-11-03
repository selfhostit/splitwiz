package ledger

import (
	"strings"
	"unicode"
)

// MaxTags is how many tags one expense can have; MaxTagLen is the longest a
// tag can be, in characters.
const (
	MaxTags   = 10
	MaxTagLen = 32
)

// NormalizeTag turns what was typed into a tag: lower case, no leading "#",
// and runs of spaces or other separators become a single "-". It returns ""
// when nothing usable is left. The web app does the same (web/src/lib/tags.ts).
func NormalizeTag(s string) string {
	var b strings.Builder
	dash := false
	n := 0
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		if n >= MaxTagLen {
			break
		}
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			if dash && b.Len() > 0 {
				b.WriteRune('-')
				n++
			}
			dash = false
			b.WriteRune(r)
			n++
		default:
			// "#", spaces, "-", "_", "/" and the like separate words
			dash = true
		}
	}
	return strings.TrimRight(b.String(), "-")
}

// NormalizeTags cleans up every tag, drops empty ones and repeats, and keeps
// at most MaxTags, in the order given.
func NormalizeTags(tags []string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, t := range tags {
		t = NormalizeTag(t)
		if t == "" || seen[t] {
			continue
		}
		seen[t] = true
		out = append(out, t)
		if len(out) == MaxTags {
			break
		}
	}
	return out
}
