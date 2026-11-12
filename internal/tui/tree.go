package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/CptPhenomeno/kb/internal/core"
)

// treeState is the selection, scroll and collapsed branches of the Tree view.
type treeState struct {
	cursor    int
	scroll    int
	collapsed map[string]bool // items whose children are hidden
	folded    bool            // the first-build fold of done branches already ran
}

// foldDone puts every done item with children in the collapsed set, once per
// session: the first time the Tree is built. Later refreshes and items that
// become done are left alone.
func (t *treeState) foldDone(m *Model) {
	if t.folded || m.snap == nil {
		return
	}
	t.folded = true
	for _, it := range m.snap.Items() {
		if it.Status != core.StatusDone || len(m.snap.Children(it.ID)) == 0 {
			continue
		}
		if t.collapsed == nil {
			t.collapsed = map[string]bool{}
		}
		t.collapsed[it.ID] = true
	}
}

// treeRow is a line of the tree.
type treeRow struct {
	it      *core.Item
	depth   int
	hasKids bool // children shown when the branch is open
}

// rows lists the visible rows: top-level items ordered by ID, children by
// position, hiding collapsed branches. With a search or a filter, an item
// stays when it or one of its descendants matches.
func (t *treeState) rows(m *Model) []treeRow {
	if m.snap == nil {
		return nil
	}
	var out []treeRow
	var walk func(it *core.Item, depth int)
	walk = func(it *core.Item, depth int) {
		var kids []*core.Item
		for _, c := range m.snap.Children(it.ID) {
			if t.shown(m, c) {
				kids = append(kids, c)
			}
		}
		out = append(out, treeRow{it: it, depth: depth, hasKids: len(kids) > 0})
		if t.collapsed[it.ID] {
			return
		}
		for _, c := range kids {
			walk(c, depth+1)
		}
	}
	for _, it := range m.snap.Items() {
		if (it.Parent == "" || m.snap.Get(it.Parent) == nil) && t.shown(m, it) {
			walk(it, 0)
		}
	}
	return out
}

// filtering reports whether a search or a filter narrows the tree.
func filtering(m *Model) bool {
	return strings.TrimSpace(m.board.query) != "" || m.board.onlyBlocked || m.board.onlyStale
}

// shown reports whether an item passes the search and the filters, itself
// or through a descendant; collapsed branches do not count.
func (t *treeState) shown(m *Model, it *core.Item) bool {
	if !filtering(m) || m.board.matchesExceptType(m, it) {
		return true
	}
	for _, d := range m.snap.Descendants(it.ID) {
		if m.board.matchesExceptType(m, d) {
			return true
		}
	}
	return false
}

// matchCount is the number of items that match the search themselves.
func (t *treeState) matchCount(m *Model) int {
	n := 0
	for _, it := range m.snap.Items() {
		if m.board.matchesExceptType(m, it) {
			n++
		}
	}
	return n
}

func (t *treeState) selected(m *Model) *core.Item {
	rows := t.rows(m)
	if t.cursor >= 0 && t.cursor < len(rows) {
		return rows[t.cursor].it
	}
	return nil
}

func (t *treeState) selectedID(m *Model) string {
	if it := t.selected(m); it != nil {
		return it.ID
	}
	return ""
}

// reveal opens the branches above id, so its row shows.
func (t *treeState) reveal(m *Model, id string) {
	if m.snap == nil {
		return
	}
	for _, a := range m.snap.Ancestors(id) {
		delete(t.collapsed, a.ID)
	}
}

// restore re-selects id after a change, or keeps the cursor in range.
func (t *treeState) restore(m *Model, id string) {
	rows := t.rows(m)
	for i, r := range rows {
		if r.it.ID == id {
			t.cursor = i
			t.ensureVisible(m)
			return
		}
	}
	t.cursor = max(0, min(t.cursor, len(rows)-1))
	t.ensureVisible(m)
}

// listHeight is the number of rows shown below the summary and the header.
func (t *treeState) listHeight(m *Model) int { return max(1, m.bodyHeight()-2) }

func (t *treeState) ensureVisible(m *Model) {
	h := t.listHeight(m)
	if t.cursor < t.scroll {
		t.scroll = t.cursor
	}
	if t.cursor >= t.scroll+h {
		t.scroll = t.cursor - h + 1
	}
	t.scroll = max(0, min(t.scroll, len(t.rows(m))-h))
}

// childType is the type the hierarchy suggests for a new child.
var childType = map[core.Type]core.Type{
	core.TypeEpic:  core.TypeStory,
	core.TypeStory: core.TypeTask,
	core.TypeTask:  core.TypeSubtask,
	core.TypeBug:   core.TypeSubtask,
}

func (t *treeState) key(m *Model, key string) tea.Cmd {
	rows := t.rows(m)
	if len(rows) == 0 {
		return m.itemKey(key)
	}
	row := rows[min(t.cursor, len(rows)-1)]
	switch key {
	case "up":
		t.cursor = max(0, t.cursor-1)
	case "down":
		t.cursor = min(len(rows)-1, t.cursor+1)
	case "left":
		if row.hasKids && !t.collapsed[row.it.ID] {
			if t.collapsed == nil {
				t.collapsed = map[string]bool{}
			}
			t.collapsed[row.it.ID] = true
			return nil
		}
		// A leaf or a closed branch: go to the parent.
		for i := t.cursor - 1; i >= 0; i-- {
			if rows[i].depth < row.depth {
				t.cursor = i
				break
			}
		}
	case "right":
		switch {
		case t.collapsed[row.it.ID]:
			delete(t.collapsed, row.it.ID)
			return nil
		case row.hasKids:
			t.cursor++
		}
	case "n":
		ct, ok := childType[row.it.Type]
		if !ok {
			m.notice = fmt.Sprintf("a %s has no children: select its parent to add a sibling", row.it.Type)
			return nil
		}
		m.form = newForm(nil)
		m.form.fields[fParent].value = row.it.ID
		m.form.fields[fType].value = ct.String()
		return nil
	default:
		return m.itemKey(key)
	}
	t.ensureVisible(m)
	return m.selectionMoved()
}

// ---- rendering ----

func (t *treeState) filterLabel(m *Model) string {
	if !filtering(m) {
		return "all items"
	}
	var parts []string
	if m.board.onlyBlocked {
		parts = append(parts, "blocked")
	}
	if m.board.onlyStale {
		parts = append(parts, "stale")
	}
	if m.board.query != "" {
		parts = append(parts, `"`+m.board.query+`"`)
	}
	return "showing " + strings.Join(parts, " · ") + " and their ancestors"
}

// treeCols are the widths of the optional columns; 0 hides a column.
type treeCols struct{ id, status, bar, progress, points, who int }

const treeGap = 2

// layout fits the columns to width w, dropping, in order, the assignee,
// the progress bar and the points when the title would get too narrow.
func (t *treeState) layout(rows []treeRow, w int) (treeCols, int) {
	c := treeCols{id: 5, status: 11, bar: 8, progress: 11, points: 6, who: 16}
	for _, r := range rows {
		c.id = max(c.id, displayWidth(r.it.ID))
	}
	title := func() int {
		used := 2 + c.id + treeGap + 7 + treeGap + c.status + treeGap + c.progress + c.points
		if c.bar > 0 {
			used += c.bar + 1
		}
		if c.who > 0 {
			used += treeGap + c.who
		}
		return w - used - treeGap
	}
	for _, drop := range []*int{&c.who, &c.bar, &c.points} {
		if title() >= 32 {
			break
		}
		*drop = 0
	}
	return c, max(10, title())
}

// cell pads or cuts a styled string to exactly w cells.
func cell(s string, w int) string {
	s = truncate(s, w)
	return s + strings.Repeat(" ", max(0, w-displayWidth(s)))
}

// rcell right-aligns a styled string in w cells.
func rcell(s string, w int) string {
	s = truncate(s, w)
	return strings.Repeat(" ", max(0, w-displayWidth(s))) + s
}

func (t *treeState) render(m *Model, height int) string {
	w := m.width
	rows := t.rows(m)
	cols, titleW := t.layout(rows, w)
	lines := []string{t.summary(m, w), t.header(m, cols, titleW)}
	sel := t.selectedID(m)
	end := min(len(rows), t.scroll+max(1, height-2))
	for _, r := range rows[min(t.scroll, len(rows)):end] {
		lines = append(lines, t.renderRow(m, r, cols, titleW, w, r.it.ID == sel))
	}
	if len(rows) == 0 {
		lines = append(lines, m.th.muted.Render("  nothing matches"))
	}
	return strings.Join(fitLines(strings.Join(lines, "\n"), w, height), "\n")
}

// summary counts the parents and the work points of the board.
func (t *treeState) summary(m *Model, w int) string {
	var epics, stories, done, total int
	for _, it := range m.snap.Items() {
		switch it.Type {
		case core.TypeEpic:
			epics++
		case core.TypeStory:
			stories++
		case core.TypeTask, core.TypeBug:
			total += it.Points
			if it.Status == core.StatusDone {
				done += it.Points
			}
		}
	}
	left := m.th.muted.Render(fmt.Sprintf("%s · %s · %s", plural(len(m.snap.Items()), "item", "items"),
		plural(epics, "epic", "epics"), plural(stories, "story", "stories")))
	right := m.th.muted.Render("work points ") + m.th.fg(m.th.p.Green).Render(fmt.Sprint(done)) +
		m.th.muted.Render(fmt.Sprintf(" done / %d", total))
	return spread(left, right, w)
}

func (t *treeState) header(m *Model, c treeCols, titleW int) string {
	parts := []string{"  " + cell("id", c.id), cell("type", 7), cell("title", titleW), cell("status", c.status)}
	prog := "progress"
	if c.bar > 0 {
		parts = append(parts, cell(prog, c.bar+1+c.progress))
	} else {
		parts = append(parts, cell(prog, c.progress))
	}
	line := strings.Join(parts, strings.Repeat(" ", treeGap))
	if c.points > 0 {
		line += rcell("points", c.points)
	}
	if c.who > 0 {
		line += strings.Repeat(" ", treeGap) + cell("assignee", c.who)
	}
	return m.th.muted.Render(line)
}

func (t *treeState) renderRow(m *Model, r treeRow, c treeCols, titleW, w int, selected bool) string {
	it := r.it
	now := m.opts.Now()
	mark := "  "
	if selected {
		mark = m.th.fg(m.th.p.Accent).Render("▌ ")
	}
	glyph := "·"
	switch {
	case r.hasKids && t.collapsed[it.ID]:
		glyph = "▸"
	case r.hasKids:
		glyph = "▾"
	}
	titleStyle := m.th.text
	switch {
	case it.Status == core.StatusDone:
		titleStyle = m.th.muted
	case it.Type.IsParentType():
		titleStyle = m.th.bright
	}
	title := strings.Repeat("  ", r.depth) + m.th.muted.Render(glyph) + " " + titleStyle.Render(it.Title)
	var badges []string
	if blocked, _ := core.Blockers(m.snap, it.ID); blocked {
		badges = append(badges, m.th.badge.Render(" BLOCKED "))
	}
	if core.IsStale(it, now) {
		badges = append(badges, m.th.stale.Render("STALE"))
	}
	if len(badges) > 0 {
		title = spread(title, strings.Join(badges, " "), titleW)
	}

	done, total := core.Progress(m.snap, it)
	progress, bar := "—", ""
	if total > 0 {
		unit := " tests"
		if it.Type.IsParentType() {
			unit = " items"
		}
		progress = fmt.Sprintf("%d/%d%s", done, total, unit)
		if c.bar > 0 {
			filled := (done*c.bar + total/2) / total
			color := m.th.p.Cyan
			if it.Status == core.StatusDone {
				color = m.th.p.Green
			}
			bar = m.th.fg(color).Render(strings.Repeat("█", filled)) + m.th.fg(m.th.p.Overlay).Render(strings.Repeat("░", c.bar-filled))
		}
	}
	if c.bar > 0 && bar == "" {
		bar = m.th.fg(m.th.p.Overlay).Render(strings.Repeat("░", c.bar))
	}

	points := "—"
	switch {
	case it.Type.IsParentType():
		if p := core.PointsSum(m.snap, it.ID); p > 0 {
			points = fmt.Sprint(p)
		}
	case it.Points > 0:
		points = fmt.Sprint(it.Points)
	}

	parts := []string{mark + m.th.muted.Render(cell(it.ID, c.id)), m.th.fg(m.th.typeColor[it.Type]).Render(cell(it.Type.String(), 7)),
		cell(title, titleW), m.th.fg(m.th.statusColor[it.Status]).Render(cell(it.Status.Label(), c.status))}
	prog := m.th.muted.Render(cell(progress, c.progress))
	if c.bar > 0 {
		prog = bar + " " + prog
	}
	parts = append(parts, prog)
	line := strings.Join(parts, strings.Repeat(" ", treeGap))
	if c.points > 0 {
		line += m.th.text.Render(rcell(points, c.points))
	}
	if c.who > 0 {
		who := m.th.muted.Render("—")
		if it.Assignee != "" {
			who = m.th.actor(it.Assignee).Render(it.Assignee)
		}
		line += strings.Repeat(" ", treeGap) + cell(who, c.who)
	}
	if selected {
		return paintBlock([]string{line}, m.th.p.Surface, w)
	}
	return line
}

// ---- mouse ----

// rowAt returns the index of the row at screen row y, below the header,
// the summary and the column titles.
func (t *treeState) rowAt(m *Model, x, y int) (int, bool) {
	i := t.scroll + y - 3
	if y < 3 || i >= len(t.rows(m)) || y-3 >= t.listHeight(m) {
		return 0, false
	}
	return i, true
}

func (t *treeState) click(m *Model, x, y int) tea.Cmd {
	i, ok := t.rowAt(m, x, y)
	if !ok {
		return nil
	}
	t.cursor = i
	t.ensureVisible(m)
	return m.selectionMoved()
}

func (t *treeState) wheel(m *Model, x int, up bool) tea.Cmd {
	if up {
		return t.key(m, "up")
	}
	return t.key(m, "down")
}
