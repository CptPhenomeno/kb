package tui

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/CptPhenomeno/kb/internal/core"
)

// sortField is what the Board columns are sorted by.
type sortField int

const (
	sortDate   sortField = iota // last change (updated_at)
	sortID                      // natural ID order
	sortPoints                  // story points; the sum below for epics and stories
	sortFields
)

var sortFieldNames = [...]string{"date", "id", "points"}
var sortFieldLabels = [...]string{"date (last change)", "id", "story points"}

// boardSort is the order of the Board columns: a field and a direction,
// or the manual order (position) that shift+↑/↓ edits.
type boardSort struct {
	manual bool
	field  sortField
	asc    bool
}

// parseBoardSort reads the saved choice, e.g. "date-desc", "id-asc" or
// "manual". Anything unknown means the default, newest change first.
func parseBoardSort(s string) boardSort {
	if s == "manual" {
		return boardSort{manual: true}
	}
	name, dir, _ := strings.Cut(s, "-")
	for f, n := range sortFieldNames {
		if n == name {
			return boardSort{field: sortField(f), asc: dir == "asc"}
		}
	}
	return boardSort{field: sortDate}
}

// String is the value saved in the user preferences.
func (s boardSort) String() string {
	if s.manual {
		return "manual"
	}
	dir := "desc"
	if s.asc {
		dir = "asc"
	}
	return sortFieldNames[s.field] + "-" + dir
}

func arrow(asc bool) string {
	if asc {
		return "↑"
	}
	return "↓"
}

// label describes the order in the header.
func (s boardSort) label() string {
	if s.manual {
		return "manual order"
	}
	return "sorted by " + sortFieldNames[s.field] + " " + arrow(s.asc)
}

// apply sorts a column in place. The sort is stable, so items that tie
// keep the position order.
func (s boardSort) apply(snap *core.Snapshot, items []*core.Item) {
	if s.manual {
		return
	}
	var by func(a, b *core.Item) int
	switch s.field {
	case sortID:
		by = func(a, b *core.Item) int { return core.CompareIDs(a.ID, b.ID) }
	case sortPoints:
		pts := func(it *core.Item) int {
			if it.Type.IsParentType() {
				return core.PointsSum(snap, it.ID)
			}
			return it.Points
		}
		by = func(a, b *core.Item) int { return cmp.Compare(pts(a), pts(b)) }
	default:
		by = func(a, b *core.Item) int { return a.UpdatedAt.Compare(b.UpdatedAt) }
	}
	if !s.asc {
		asc := by
		by = func(a, b *core.Item) int { return asc(b, a) }
	}
	slices.SortStableFunc(items, by)
}

// ---- popup ----

// sortState is the open sort popup: a row per field, each with its own
// direction, so switching rows keeps what was toggled.
type sortState struct {
	cursor int
	asc    [sortFields]bool
}

func (m *Model) openSort() {
	s := &sortState{}
	if cur := m.board.sort; !cur.manual {
		s.cursor = int(cur.field)
		s.asc[cur.field] = cur.asc
	}
	m.sort = s
}

func (m *Model) sortKey(key string) tea.Cmd {
	s := m.sort
	switch key {
	case "up":
		s.cursor = (s.cursor + int(sortFields) - 1) % int(sortFields)
	case "down":
		s.cursor = (s.cursor + 1) % int(sortFields)
	case "space", "left", "right":
		s.asc[s.cursor] = !s.asc[s.cursor]
	case "enter":
		m.sort = nil
		m.setBoardSort(boardSort{field: sortField(s.cursor), asc: s.asc[s.cursor]})
	case "esc", "s":
		m.sort = nil
	}
	return nil
}

// setBoardSort changes the order of the columns, keeping the selection,
// and saves it for the user.
func (m *Model) setBoardSort(s boardSort) {
	b := &m.board
	prev := b.selectedID(m)
	b.sort = s
	b.restore(m, prev)
	m.notice = "columns: " + s.label()
	if m.opts.SaveBoardSort != nil {
		if err := m.opts.SaveBoardSort(s.String()); err != nil {
			m.notice += " (could not save: " + err.Error() + ")"
		}
	}
}

func (m *Model) renderSort() string {
	s := m.sort
	width := min(58, m.width-4)
	iw := width - 6
	lines := []string{
		spread(m.th.bright.Render("Sort columns"), m.th.muted.Render("now: "+m.board.sort.label()), iw),
		"",
	}
	for f := range sortFields {
		dir := "↓ descending"
		if s.asc[f] {
			dir = "↑ ascending"
		}
		mark, style := "  ", m.th.text
		if int(f) == s.cursor {
			mark, style = m.th.fg(m.th.p.Accent).Render("▸ "), m.th.bright
		}
		lines = append(lines, spread(mark+style.Render(sortFieldLabels[f]), m.th.fg(m.th.p.Cyan).Render(dir), iw))
	}
	lines = append(lines, "")
	lines = append(lines, hang("", 0, "A manual reorder (shift ↑ ↓) starts from the order shown; the next sort chosen here overrides it.",
		m.th.muted, iw, 0)...)
	lines = append(lines, "", m.th.muted.Render(fmt.Sprintf("%s direction  %s apply  %s cancel",
		m.th.chip.Render(" space "), m.th.chip.Render(" enter "), m.th.chip.Render(" esc "))))
	return m.popupBox(lines, width, m.th.fg(m.th.p.Accent))
}

var sortKeys = []keyHint{
	{"↑ ↓", "field", "choose what to sort by", false},
	{"space", "direction", "ascending or descending (← → too)", false},
	{"enter", "apply", "sort every column", false},
	{"esc", "cancel", "keep the current order", false},
}
