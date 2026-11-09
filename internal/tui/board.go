package tui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/CptPhenomeno/kb/internal/core"
)

// boardState is the selection and scroll state of the Board view.
type boardState struct {
	col, row int
	scroll   [5]int
	allTypes bool // false: only tasks and bugs (SPEC.md §8.1)
}

func (b *boardState) filterLabel() string {
	if b.allTypes {
		return "showing all types"
	}
	return "showing task · bug"
}

// columns returns the cards of every column in board order.
func (b *boardState) columns(m *Model) [5][]*core.Item {
	var cols [5][]*core.Item
	if m.snap == nil {
		return cols
	}
	for i, st := range core.Statuses() {
		for _, it := range core.Column(m.snap, st) {
			if b.allTypes || it.Type == core.TypeTask || it.Type == core.TypeBug {
				cols[i] = append(cols[i], it)
			}
		}
	}
	return cols
}

func (b *boardState) selected(m *Model) *core.Item {
	cols := b.columns(m)
	if b.col < len(cols) && b.row < len(cols[b.col]) {
		return cols[b.col][b.row]
	}
	return nil
}

func (b *boardState) selectedID(m *Model) string {
	if it := b.selected(m); it != nil {
		return it.ID
	}
	return ""
}

// restore re-selects prevID after a reload, or keeps the selection inside
// the board when the item is gone.
func (b *boardState) restore(m *Model, prevID string) {
	cols := b.columns(m)
	for c, items := range cols {
		for r, it := range items {
			if it.ID == prevID {
				b.col, b.row = c, r
				b.ensureVisible(m)
				return
			}
		}
	}
	if prevID == "" || len(cols[b.col]) == 0 {
		b.col = firstNonEmpty(cols, 0, 1)
	}
	b.row = max(0, min(b.row, len(cols[b.col])-1))
	b.ensureVisible(m)
}

// firstNonEmpty returns the first non-empty column from start in direction
// dir, or start if there is none.
func firstNonEmpty(cols [5][]*core.Item, start, dir int) int {
	for c := start; c >= 0 && c < len(cols); c += dir {
		if len(cols[c]) > 0 {
			return c
		}
	}
	return start
}

func (b *boardState) key(m *Model, key string) tea.Cmd {
	cols := b.columns(m)
	switch key {
	case "left", "right":
		dir := 1
		if key == "left" {
			dir = -1
		}
		for c := b.col + dir; c >= 0 && c < len(cols); c += dir {
			if len(cols[c]) > 0 {
				b.col = c
				b.row = min(b.row, len(cols[c])-1)
				break
			}
		}
	case "up":
		b.row = max(0, b.row-1)
	case "down":
		b.row = max(0, min(b.row+1, len(cols[b.col])-1))
	case "enter":
		if id := b.selectedID(m); id != "" {
			m.detailOpen = true
			m.history = nil
			b.ensureVisible(m)
			return m.loadHistory(id)
		}
		return nil
	default:
		return nil
	}
	b.ensureVisible(m)
	if m.detailOpen {
		m.history = nil
		return m.loadHistory(b.selectedID(m))
	}
	return nil
}

// ---- layout ----

// geometry of the board area for the current terminal size.
type geometry struct {
	top     int    // first screen row of the board area
	height  int    // rows of the board area
	colX    [5]int // left edge of each column
	colW    [5]int // width of each column, borders included
	detailX int    // left edge of the detail panel
	detailW int    // 0 when the detail panel is closed
}

const colGap = 1

func (b *boardState) geometry(m *Model) geometry {
	g := geometry{top: 1, height: m.height - 2 - len(m.commandBar())}
	boardW := m.width
	if m.detailOpen {
		g.detailW = min(56, max(36, m.width*35/100))
		boardW = m.width - g.detailW - colGap
		g.detailX = boardW + colGap
	}
	base := (boardW - 4*colGap) / 5
	extra := (boardW - 4*colGap) % 5
	x := 0
	for i := range 5 {
		g.colX[i] = x
		g.colW[i] = base
		if i < extra {
			g.colW[i]++
		}
		x += g.colW[i] + colGap
	}
	return g
}

// cardHeight is the height of a card of column width w, borders included.
func cardHeight(m *Model, it *core.Item, w int) int { return len(cardLines(m, it, w-6)) + 2 }

// visible returns the range of cards shown in a column and the markers
// for hidden cards above and below.
func (b *boardState) visible(m *Model, items []*core.Item, scroll, w, avail int) (end int, above, below bool) {
	above = scroll > 0
	used := 0
	if above {
		used++
	}
	end = scroll
	for end < len(items) {
		h := cardHeight(m, items[end], w)
		reserve := 0
		if end+1 < len(items) {
			reserve = 1 // room for the "more below" marker
		}
		if used+h+reserve > avail && end > scroll {
			break
		}
		used += h
		end++
	}
	return end, above, end < len(items)
}

// cardsArea is the number of rows inside a column available for cards.
func cardsArea(g geometry) int { return g.height - 3 } // borders and title

// ensureVisible scrolls the selected column so the selected card shows.
func (b *boardState) ensureVisible(m *Model) {
	if m.snap == nil || m.height == 0 {
		return
	}
	cols := b.columns(m)
	g := b.geometry(m)
	items := cols[b.col]
	if b.row < b.scroll[b.col] {
		b.scroll[b.col] = b.row
	}
	for b.scroll[b.col] < b.row {
		end, _, _ := b.visible(m, items, b.scroll[b.col], g.colW[b.col], cardsArea(g))
		if b.row < end {
			break
		}
		b.scroll[b.col]++
	}
	for c := range cols {
		b.scroll[c] = max(0, min(b.scroll[c], len(cols[c])-1))
	}
}

// ---- rendering ----

func (b *boardState) render(m *Model, height int) string {
	g := b.geometry(m)
	cols := b.columns(m)
	sel := b.selectedID(m)
	parts := make([]string, 0, 10)
	for i, st := range core.Statuses() {
		if i > 0 {
			parts = append(parts, strings.Repeat(" ", colGap))
		}
		parts = append(parts, b.renderColumn(m, st, cols[i], b.scroll[i], g.colW[i], height, sel))
	}
	if g.detailW > 0 {
		parts = append(parts, strings.Repeat(" ", colGap), m.renderDetail(g.detailW, height))
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, parts...)
}

func (b *boardState) renderColumn(m *Model, st core.Status, items []*core.Item, scroll, w, height int, sel string) string {
	inner := w - 4
	title := spread(m.th.fg(m.th.statusColor[st]).Bold(true).Render(st.Label()), m.th.muted.Render(fmt.Sprint(len(items))), inner)
	lines := []string{title}
	avail := height - 3
	end, above, below := b.visible(m, items, scroll, w, avail)
	if above {
		lines = append(lines, m.th.muted.Render(center(fmt.Sprintf("↑ %d more", scroll), inner)))
	}
	for _, it := range items[scroll:end] {
		lines = append(lines, strings.Split(renderCard(m, it, inner, it.ID == sel), "\n")...)
	}
	if below {
		lines = append(lines, m.th.muted.Render(center(fmt.Sprintf("↓ %d more", len(items)-end), inner)))
	}
	if len(items) == 0 {
		lines = append(lines, m.th.muted.Render(center("empty", inner)))
	}
	border := m.th.p.Overlay
	if m.view == viewBoard && len(items) > 0 && b.col == int(st) {
		border = m.th.p.Muted
	}
	box := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(border).Width(w).Padding(0, 1)
	return box.Render(strings.Join(fitLines(strings.Join(lines, "\n"), inner, avail+1), "\n"))
}

// cardLines are the text lines of a card, each at most width wide.
func cardLines(m *Model, it *core.Item, width int) []string {
	now := m.opts.Now()
	right := m.th.fg(m.th.prioColor[it.Priority]).Render(prioMarks[it.Priority]) + pointsLabel(m, it.Points)
	left := m.th.muted.Render(it.ID) + " " + m.th.fg(m.th.typeColor[it.Type]).Render(it.Type.String())
	if displayWidth(left)+1+displayWidth(right) > width {
		left = m.th.muted.Render(it.ID) // narrow column: drop the type
	}
	lines := []string{spread(left, right, width)}
	for _, l := range wrap(it.Title, width, 2) {
		lines = append(lines, m.th.bright.Render(l))
	}
	var parts []string
	if it.Phase != core.PhaseNone {
		parts = append(parts, m.th.fg(m.th.phaseColor[it.Phase]).Render("● "+strings.ToUpper(it.Phase.String())))
	}
	if len(it.Tests) > 0 {
		parts = append(parts, m.th.muted.Render(fmt.Sprintf("%d/%d", it.PassingTests(), len(it.Tests))))
	}
	who := ""
	if it.Assignee != "" {
		who = m.th.actor(it.Assignee).Render(it.Assignee)
	}
	status := strings.Join(parts, " ")
	switch {
	case status != "" && who != "" && displayWidth(status)+1+displayWidth(who) > width:
		lines = append(lines, status, truncate(who, width)) // narrow column: two lines
	case status != "" || who != "":
		lines = append(lines, spread(status, who, width))
	}
	if blocked, by := core.Blockers(m.snap, it.ID); blocked {
		lines = append(lines, truncate(m.th.badge.Render(" BLOCKED by "+strings.Join(by, ",")+" "), width))
	}
	if core.ClaimState(it, now) == core.ClaimStale {
		lines = append(lines, m.th.stale.Render("STALE "+shortDuration(now.Sub(it.Claim.ExpiresAt))))
	}
	return lines
}

func renderCard(m *Model, it *core.Item, w int, selected bool) string {
	border := m.th.p.Overlay
	if selected {
		border = m.th.p.Accent
	}
	style := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(border).
		BorderBackground(m.th.p.Base).Background(m.th.p.Surface).Width(w).Padding(0, 1)
	return style.Render(paintBlock(cardLines(m, it, w-4), m.th.p.Surface, w-4))
}

func pointsLabel(m *Model, p int) string {
	if p == 0 {
		return ""
	}
	return m.th.muted.Render(fmt.Sprintf(" %dpt", p))
}

// shortDuration renders 22m, 3h or 2d.
func shortDuration(d time.Duration) string {
	switch {
	case d < time.Minute:
		return "now"
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}

// ---- mouse ----

// cardAt returns the column and row of the card at screen cell (x, y).
func (b *boardState) cardAt(m *Model, x, y int) (col, row int, ok bool) {
	g := b.geometry(m)
	cols := b.columns(m)
	for c := range cols {
		if x < g.colX[c] || x >= g.colX[c]+g.colW[c] {
			continue
		}
		cy := g.top + 2 // column border and title
		scroll := b.scroll[c]
		end, above, _ := b.visible(m, cols[c], scroll, g.colW[c], cardsArea(g))
		if above {
			cy++
		}
		for r := scroll; r < end; r++ {
			h := cardHeight(m, cols[c][r], g.colW[c])
			if y >= cy && y < cy+h {
				return c, r, true
			}
			cy += h
		}
		return c, 0, false
	}
	return 0, 0, false
}

func (b *boardState) click(m *Model, x, y int) tea.Cmd {
	c, r, ok := b.cardAt(m, x, y)
	if !ok {
		return nil
	}
	b.col, b.row = c, r
	b.ensureVisible(m)
	if m.detailOpen {
		m.history = nil
		return m.loadHistory(b.selectedID(m))
	}
	return nil
}

// wheel moves the selection up or down in the column under the mouse.
func (b *boardState) wheel(m *Model, x int, up bool) tea.Cmd {
	g := b.geometry(m)
	cols := b.columns(m)
	for c := range cols {
		if x >= g.colX[c] && x < g.colX[c]+g.colW[c] && len(cols[c]) > 0 {
			if c != b.col {
				b.col, b.row = c, min(b.row, len(cols[c])-1)
			}
			break
		}
	}
	if up {
		return b.key(m, "up")
	}
	return b.key(m, "down")
}
