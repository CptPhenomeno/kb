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
)

var sortFieldNames = [...]string{"date", "id", "points"}
var sortFieldLabels = [...]string{"date", "ID", "story points"}

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

// sortOption is a choice of the sort popup.
type sortOption struct {
	sort boardSort
	desc string
}

// sortOptions are the choices of the sort popup, one at a time.
var sortOptions = []sortOption{
	{boardSort{field: sortDate}, "newest change first"},
	{boardSort{field: sortDate, asc: true}, "oldest change first"},
	{boardSort{field: sortID}, "highest ID first"},
	{boardSort{field: sortID, asc: true}, "lowest ID first"},
	{boardSort{field: sortPoints}, "most points first"},
	{boardSort{field: sortPoints, asc: true}, "fewest points first"},
}

// sortState is the open sort popup.
type sortState struct{ cursor int }

func (m *Model) openSort() {
	m.sort = &sortState{cursor: max(0, slices.IndexFunc(sortOptions, func(o sortOption) bool { return o.sort == m.board.sort }))}
}

func (m *Model) sortKey(key string) tea.Cmd {
	s := m.sort
	switch key {
	case "up":
		s.cursor = (s.cursor + len(sortOptions) - 1) % len(sortOptions)
	case "down":
		s.cursor = (s.cursor + 1) % len(sortOptions)
	case "enter":
		m.sort = nil
		m.setBoardSort(sortOptions[s.cursor].sort)
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
	lines := []string{spread(m.th.bright.Render("Sort columns"), m.th.muted.Render("esc close"), iw), ""}
	for i, o := range sortOptions {
		cursor, style := "  ", m.th.text
		if i == s.cursor {
			cursor, style = m.th.fg(m.th.p.Accent).Render("▸ "), m.th.bright
		}
		radio := m.th.muted.Render("○ ")
		if o.sort == m.board.sort {
			radio = m.th.fg(m.th.p.Green).Render("● ")
		}
		name := fmt.Sprintf("%-17s", sortFieldLabels[o.sort.field]+" "+arrow(o.sort.asc))
		lines = append(lines, cursor+radio+style.Render(name)+m.th.muted.Render(o.desc))
	}
	if m.board.sort.manual {
		lines = append(lines, "", m.th.muted.Render("Now: manual order."))
	}
	lines = append(lines, "")
	lines = append(lines, hang("", 0, "A manual reorder (shift ↑ ↓) starts from the order shown; the next sort chosen here overrides it.",
		m.th.muted, iw, 0)...)
	return m.popupBox(lines, width, m.th.fg(m.th.p.Accent))
}

var sortKeys = []keyHint{
	{"↑ ↓", "choose", "choose how to sort", false},
	{"enter", "apply", "sort every column", false},
	{"esc", "cancel", "keep the current order", false},
}
