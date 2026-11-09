package tui

import (
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
)

// action is an entry of the command palette: running it is the same as
// pressing key. views limits the action to some views; nil means all.
type action struct {
	name, key string
	views     []viewKind
}

var boardOnly = []viewKind{viewBoard}

// actions lists only features that work; add an entry when one lands.
var actions = []action{
	{"Move to next column", "shift+right", boardOnly},
	{"Move to previous column", "shift+left", boardOnly},
	{"Reorder up", "shift+up", boardOnly},
	{"Reorder down", "shift+down", boardOnly},
	{"Open details", "enter", boardOnly},
	{"Search", "/", boardOnly},
	{"Filter", "f", boardOnly},
	{"Show or hide epics and stories", "p", boardOnly},
	{"Check acceptance criteria", "c", boardOnly},
	{"Create item", "n", boardOnly},
	{"Edit item", "e", boardOnly},
	{"Edit description in $EDITOR", "ctrl+e", boardOnly},
	{"Delete item", "d", boardOnly},
	{"Claim or release item", "a", boardOnly},
	{"Manage links", "l", boardOnly},
	{"Choose theme", "t", nil},
	{"Next view", "tab", nil},
	{"Previous view", "shift+tab", nil},
	{"Help", "?", nil},
	{"Quit", "q", nil},
}

// paletteState is the open command palette.
type paletteState struct {
	query  string
	cursor int
}

// fuzzyScore reports whether every rune of q appears in name in order
// (case-insensitive) and how spread out the match is; lower is better.
func fuzzyScore(name, q string) (int, bool) {
	name, q = strings.ToLower(name), strings.ToLower(q)
	score, pos := 0, 0
	for _, r := range q {
		i := strings.IndexRune(name[pos:], r)
		if i < 0 {
			return 0, false
		}
		score += i
		pos += i + len(string(r))
	}
	return score, true
}

// matches returns the actions of the current view that match the query,
// best first.
func (p *paletteState) matches(m *Model) []action {
	type scored struct {
		a     action
		score int
	}
	var found []scored
	for _, a := range actions {
		if a.views != nil && !slices.Contains(a.views, m.view) {
			continue
		}
		if s, ok := fuzzyScore(a.name, p.query); ok {
			found = append(found, scored{a, s})
		}
	}
	slices.SortStableFunc(found, func(x, y scored) int { return x.score - y.score })
	out := make([]action, len(found))
	for i, f := range found {
		out[i] = f.a
	}
	return out
}

func (m *Model) paletteKey(key, text string) tea.Cmd {
	p := m.palette
	list := p.matches(m)
	switch key {
	case "esc", "ctrl+p", "ctrl+shift+p":
		m.palette = nil
	case "up":
		p.cursor = max(0, p.cursor-1)
	case "down":
		p.cursor = min(max(0, len(list)-1), p.cursor+1)
	case "enter":
		m.palette = nil
		if p.cursor < len(list) {
			return m.dispatch(list[p.cursor].key, "")
		}
	case "backspace":
		if r := []rune(p.query); len(r) > 0 {
			p.query, p.cursor = string(r[:len(r)-1]), 0
		}
	default:
		if text != "" {
			p.query, p.cursor = p.query+text, 0
		}
	}
	return nil
}

func (m *Model) renderPalette() string {
	width := min(64, m.width-4)
	iw := width - 6
	p := m.palette
	lines := []string{
		spread(m.th.bright.Render("Commands")+m.th.muted.Render(" · "+m.view.String()+" view"), m.th.muted.Render("esc close"), iw),
		m.th.fg(m.th.p.Accent).Render("› ") + m.th.bright.Render(p.query) + m.th.fg(m.th.p.Accent).Render("▌"),
		"",
	}
	list := p.matches(m)
	if len(list) == 0 {
		lines = append(lines, m.th.muted.Render("no command matches"))
	}
	for i, a := range list {
		name := m.th.text.Render(a.name)
		marker := "  "
		if i == p.cursor {
			marker, name = m.th.fg(m.th.p.Accent).Render("› "), m.th.fg(m.th.p.Accent).Bold(true).Render(a.name)
		}
		lines = append(lines, spread(marker+name, m.th.chip.Render(" "+keyLabel(a.key)+" "), iw))
	}
	return m.popupBox(lines, width, m.th.fg(m.th.p.Accent))
}

// keyLabel renders a key name the way the command bar does.
func keyLabel(k string) string {
	r := strings.NewReplacer("shift+right", "shift →", "shift+left", "shift ←", "shift+up", "shift ↑",
		"shift+down", "shift ↓")
	return r.Replace(k)
}

var paletteKeys = []keyHint{
	{"type", "filter", "type part of a command name", false},
	{"↑ ↓", "choose", "choose a command", false},
	{"enter", "run", "run the chosen command", false},
	{"esc", "close", "close the palette", false},
}
