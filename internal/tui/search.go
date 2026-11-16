package tui

import (
	"strings"
	"unicode"

	"github.com/CptPhenomeno/kb/internal/core"
)

// matchSearch reports whether an item satisfies the search text. The text is
// split into words on whitespace and every word must match (AND); an empty
// text matches everything. A word is one of:
//
//   - tag:X   the item has a tag whose core.TagKey equals X's (exact, no case);
//   - type:X  the item is of type X; an unknown type matches nothing;
//   - other   the word is contained, without case, in the ID, title, assignee
//     or any tag.
//
// The tag: and type: prefixes are recognized without regard to case. A value
// with spaces is double-quoted, as in tag:"good first"; see splitSearch.
func matchSearch(query string, it *core.Item) bool {
	for _, w := range splitSearch(query) {
		if !matchWord(w.text, it) {
			return false
		}
	}
	return true
}

func matchWord(w string, it *core.Item) bool {
	low := strings.ToLower(w)
	switch {
	case strings.HasPrefix(low, "tag:"):
		want := core.TagKey(w[len("tag:"):])
		for _, t := range it.Tags {
			if core.TagKey(t) == want {
				return want != ""
			}
		}
		return false
	case strings.HasPrefix(low, "type:"):
		typ, err := core.ParseType(low[len("type:"):])
		return err == nil && it.Type == typ
	}
	hay := strings.ToLower(strings.Join(append([]string{it.ID, it.Title, it.Assignee}, it.Tags...), " "))
	return strings.Contains(hay, low)
}

// searchWord is one word of the search text: raw as typed, text with the
// double quotes removed.
type searchWord struct {
	raw, text string
}

// splitSearch splits the search text into words on whitespace outside double
// quotes. Quotes group a value that has spaces (tag:"a b", "a b") and are
// dropped from the word's text; a quote left open runs to the end of the
// text, since the user is still typing it. Text without quotes splits as
// strings.Fields does.
func splitSearch(query string) []searchWord {
	var words []searchWord
	var raw, text strings.Builder
	inQuote, started := false, false
	flush := func() {
		if started {
			words = append(words, searchWord{raw.String(), text.String()})
		}
		raw.Reset()
		text.Reset()
		started = false
	}
	for _, r := range query {
		switch {
		case r == '"':
			inQuote, started = !inQuote, true
			raw.WriteRune(r)
		case !inQuote && unicode.IsSpace(r):
			flush()
		default:
			started = true
			raw.WriteRune(r)
			text.WriteRune(r)
		}
	}
	flush()
	return words
}

// quoteFilterValue quotes v when it holds whitespace, so a filter built from
// it stays one word.
func quoteFilterValue(v string) string {
	if strings.IndexFunc(v, unicode.IsSpace) >= 0 {
		return `"` + v + `"`
	}
	return v
}
