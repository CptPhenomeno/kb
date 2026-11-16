package tui

import (
	"strings"

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
// The tag: and type: prefixes are recognized without regard to case. Words
// cannot contain spaces, so a tag name with spaces is matched by its plain
// words instead.
func matchSearch(query string, it *core.Item) bool {
	for _, w := range strings.Fields(query) {
		if !matchWord(w, it) {
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
