package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/CptPhenomeno/kb/internal/core"
)

// Suggestion limits per group (SRC-1).
const (
	maxTagSuggestions  = 3
	maxTypeSuggestions = 2
	maxCardSuggestions = 5
)

// sugKind is the group a suggestion belongs to.
type sugKind int

const (
	sugTag sugKind = iota
	sugType
	sugCard
)

// suggestion is one entry of the list under the search line. value is the
// tag name, the type name or the card ID, depending on kind: what accepting
// the suggestion acts on.
type suggestion struct {
	kind  sugKind
	value string
}

// typedWord is the word being typed in the search: the last
// whitespace-separated word, or "" when the text ends with a space.
func typedWord(query string) string {
	if query == "" || strings.TrimRight(query, " \t") != query {
		return ""
	}
	f := strings.Fields(query)
	return f[len(f)-1]
}

// suggestions lists what the typed word could mean, in group order: tags,
// types, cards. A word starting with tag: or type: (any case) narrows the
// list to that group and matches on the rest. Matching is contains, without
// case. Cards are the items of the current view that pass its other filters
// (types on the Board, blocked, stale); the search words themselves are not
// applied, since they are what is being completed.
func (m *Model) suggestions() []suggestion {
	word := typedWord(m.board.query)
	if word == "" {
		return nil
	}
	low := strings.ToLower(word)
	var wantTags, wantTypes, wantCards = true, true, true
	switch {
	case strings.HasPrefix(low, "tag:"):
		low, wantTypes, wantCards = low[len("tag:"):], false, false
	case strings.HasPrefix(low, "type:"):
		low, wantTags, wantCards = low[len("type:"):], false, false
	}
	var out []suggestion
	if wantTags {
		n := 0
		for _, t := range m.tags {
			if n < maxTagSuggestions && strings.Contains(strings.ToLower(t.Name), low) {
				out = append(out, suggestion{sugTag, t.Name})
				n++
			}
		}
	}
	if wantTypes {
		n := 0
		for _, t := range core.Types() {
			if n < maxTypeSuggestions && strings.Contains(t.String(), low) {
				out = append(out, suggestion{sugType, t.String()})
				n++
			}
		}
	}
	if wantCards && m.snap != nil {
		n := 0
		for _, it := range m.snap.Items() {
			if n >= maxCardSuggestions {
				break
			}
			if m.suggestsCard(it) && (strings.Contains(strings.ToLower(it.ID), low) || strings.Contains(strings.ToLower(it.Title), low)) {
				out = append(out, suggestion{sugCard, it.ID})
				n++
			}
		}
	}
	return out
}

// suggestsCard reports whether the current view lists the item, leaving the
// search text aside.
func (m *Model) suggestsCard(it *core.Item) bool {
	b := &m.board
	if m.view == viewBoard && !b.types[it.Type] {
		return false
	}
	if b.onlyBlocked {
		if blocked, _ := core.Blockers(m.snap, it.ID); !blocked {
			return false
		}
	}
	return !b.onlyStale || core.IsStale(it, m.opts.Now())
}

// selectedSuggestion is the suggestion picked with ↑ ↓, nil when none is.
// The pick is remembered as the suggestion itself, not as a position: a live
// reload can reshuffle the list, and the highlight must stay on what the user
// chose. A pick that is no longer in the list counts as no selection.
func (m *Model) selectedSuggestion() *suggestion {
	if m.board.sugPick == nil {
		return nil
	}
	for _, s := range m.suggestions() {
		if s == *m.board.sugPick {
			return &s
		}
	}
	return nil
}

// moveSuggestion moves the selection by delta among the suggestions: from
// none, down picks the first; up from the first goes back to none; the ends
// do not wrap.
func (m *Model) moveSuggestion(delta int) {
	sg := m.suggestions()
	cur := 0 // 1-based position of the pick, 0 for none
	if sel := m.selectedSuggestion(); sel != nil {
		for i, s := range sg {
			if s == *sel {
				cur = i + 1
			}
		}
	}
	cur = max(0, min(len(sg), cur+delta))
	m.board.sugPick = nil
	if cur > 0 {
		m.board.sugPick = &sg[cur-1]
	}
}

// renderSuggestions draws the list as a box, "" when there is nothing to
// suggest. Each group has a heading; the selected row starts with ›.
func (m *Model) renderSuggestions() string {
	sg := m.suggestions()
	if len(sg) == 0 {
		return ""
	}
	sel := m.selectedSuggestion()
	width := min(60, m.width-4)
	iw := width - 6
	var lines []string
	heading := map[sugKind]string{sugTag: "tags", sugType: "types", sugCard: "cards"}
	for i, s := range sg {
		if i == 0 || sg[i-1].kind != s.kind {
			lines = append(lines, m.th.muted.Render(heading[s.kind]))
		}
		row := m.suggestionRow(s, iw-2)
		if sel != nil && s == *sel {
			lines = append(lines, m.th.fg(m.th.p.Accent).Render("› ")+row)
		} else {
			lines = append(lines, "  "+row)
		}
	}
	return m.popupBox(lines, width, m.th.fg(m.th.p.Accent))
}

// suggestionRow draws one suggestion in at most w cells.
func (m *Model) suggestionRow(s suggestion, w int) string {
	switch s.kind {
	case sugTag:
		return m.th.tagLabel(m.tagColor(s.value), s.value, w)
	case sugType:
		t, _ := core.ParseType(s.value)
		return m.th.fg(m.th.typeColor[t]).Render(s.value)
	}
	it := m.snap.Get(s.value)
	if it == nil {
		return ""
	}
	head := it.ID + " " + it.Type.String() + " "
	return m.th.muted.Render(it.ID) + " " + m.th.fg(m.th.typeColor[it.Type]).Render(it.Type.String()) + " " +
		m.th.text.Render(truncate(it.Title, max(1, w-len(head))))
}

// acceptSuggestion is what enter does on a selected suggestion. A tag or a
// type replaces the word being typed with the exact tag: or type: filter and
// a space, so the list closes and the next word starts clean; the search
// stays open. A card closes the search, clears it and selects the card in
// the current view, opening the Tree branches above it.
func (m *Model) acceptSuggestion(s suggestion) tea.Cmd {
	b := &m.board
	b.sugPick = nil
	switch s.kind {
	case sugTag, sugType:
		prefix := "tag:"
		if s.kind == sugType {
			prefix = "type:"
		}
		head := strings.TrimSuffix(b.query, typedWord(b.query))
		b.query = head + prefix + s.value + " "
		b.restore(m, b.selectedID(m))
		m.tree.restore(m, m.tree.selectedID(m))
	case sugCard:
		b.searching, b.query = false, ""
		if m.view == viewTree {
			m.tree.reveal(m, s.value)
			m.tree.restore(m, s.value)
		} else {
			b.restore(m, s.value)
		}
	}
	return nil
}
