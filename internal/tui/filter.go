package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/CptPhenomeno/kb/internal/core"
)

// defaultTypes are the types shown on the board when kb starts: subtasks
// are listed in their task's detail instead.
func defaultTypes() map[core.Type]bool {
	return map[core.Type]bool{core.TypeEpic: true, core.TypeStory: true, core.TypeTask: true, core.TypeBug: true}
}

// matches reports whether an item passes the board's filters and search.
func (b *boardState) matches(m *Model, it *core.Item) bool {
	return b.types[it.Type] && b.matchesExceptType(m, it)
}

// matchesExceptType applies every filter but the types: the Tree view shows
// all types, so the hierarchy stays whole.
func (b *boardState) matchesExceptType(m *Model, it *core.Item) bool {
	if b.onlyBlocked {
		if blocked, _ := core.Blockers(m.snap, it.ID); !blocked {
			return false
		}
	}
	if b.onlyStale && !core.IsStale(it, m.opts.Now()) {
		return false
	}
	if q := strings.ToLower(strings.TrimSpace(b.query)); q != "" {
		hay := strings.ToLower(strings.Join(append([]string{it.ID, it.Title, it.Assignee}, it.Labels...), " "))
		if !strings.Contains(hay, q) {
			return false
		}
	}
	return true
}

func (b *boardState) filterLabel() string {
	var types []string
	for _, t := range core.Types() {
		if b.types[t] {
			types = append(types, t.String())
		}
	}
	parts := []string{strings.Join(types, " · ")}
	switch {
	case len(types) == len(core.Types()):
		parts[0] = "all types"
	case len(types) == 0:
		parts[0] = "no types"
	}
	if b.onlyBlocked {
		parts = append(parts, "blocked")
	}
	if b.onlyStale {
		parts = append(parts, "stale")
	}
	if b.query != "" {
		parts = append(parts, `"`+b.query+`"`)
	}
	return "showing " + strings.Join(parts, " · ") + " · " + b.sort.label()
}

// filterRows are the rows of the filter popup, in order.
var filterRows = []string{"epic", "story", "task", "bug", "subtask", "only blocked", "only stale"}

func (b *boardState) rowOn(i int) bool {
	switch {
	case i < len(core.Types()):
		return b.types[[]core.Type{core.TypeEpic, core.TypeStory, core.TypeTask, core.TypeBug, core.TypeSubtask}[i]]
	case i == 5:
		return b.onlyBlocked
	default:
		return b.onlyStale
	}
}

func (b *boardState) toggleRow(i int) {
	switch {
	case i < 5:
		t := []core.Type{core.TypeEpic, core.TypeStory, core.TypeTask, core.TypeBug, core.TypeSubtask}[i]
		b.types[t] = !b.types[t]
	case i == 5:
		b.onlyBlocked = !b.onlyBlocked
	default:
		b.onlyStale = !b.onlyStale
	}
}

// filterKey handles a key while the filter popup is open. Filters apply
// at once, so the board behind shows the result.
func (m *Model) filterKey(key string) tea.Cmd {
	b := &m.board
	prevBoard, prevTree := b.selectedID(m), m.tree.selectedID(m)
	switch key {
	case "up":
		b.filterCursor = (b.filterCursor + len(filterRows) - 1) % len(filterRows)
	case "down":
		b.filterCursor = (b.filterCursor + 1) % len(filterRows)
	case "space", "enter":
		b.toggleRow(b.filterCursor)
		b.restore(m, prevBoard)
		m.tree.restore(m, prevTree)
	case "esc", "f":
		m.filterOpen = false
	}
	return nil
}

// searchKey handles a key while typing a search.
func (m *Model) searchKey(key, text string) tea.Cmd {
	b := &m.board
	prevBoard, prevTree := b.selectedID(m), m.tree.selectedID(m)
	switch key {
	case "enter":
		b.searching = false
		return nil
	case "esc":
		b.searching, b.query = false, ""
	case "backspace":
		if r := []rune(b.query); len(r) > 0 {
			b.query = string(r[:len(r)-1])
		}
	default:
		if text == "" {
			return nil
		}
		b.query += text
	}
	b.restore(m, prevBoard)
	m.tree.restore(m, prevTree)
	return nil
}

func (m *Model) renderFilter() string {
	width := min(44, m.width-4)
	iw := width - 6
	lines := []string{
		spread(m.th.bright.Render("Filter"), m.th.muted.Render("esc close"), iw),
		m.th.muted.Render("the board updates as you toggle"),
		"",
	}
	for i, row := range filterRows {
		if i == 5 {
			lines = append(lines, "")
		}
		box := "[ ]"
		if m.board.rowOn(i) {
			box = "[x]"
		}
		line := box + " " + row
		if i == m.board.filterCursor {
			lines = append(lines, m.th.fg(m.th.p.Accent).Render("› ")+m.th.fg(m.th.p.Accent).Bold(true).Render(line))
		} else {
			lines = append(lines, "  "+m.th.text.Render(line))
		}
	}
	lines = append(lines, "", m.th.muted.Render("/ searches ID, title, label and assignee"))
	return m.popupBox(lines, width, m.th.fg(m.th.p.Accent))
}

var filterKeys = []keyHint{
	{"↑ ↓", "move", "move between filters", false},
	{"space", "toggle", "turn the filter on or off", false},
	{"esc", "close", "close the filter popup", false},
}

var searchKeys = []keyHint{
	{"type", "search", "search by ID, title, label or assignee", false},
	{"enter", "keep", "keep the search and go back to the board", false},
	{"esc", "clear", "clear the search", false},
}

// shownCount is the number of cards on the board after filters.
func (b *boardState) shownCount(m *Model) int {
	n := 0
	for _, col := range b.columns(m) {
		n += len(col)
	}
	return n
}
