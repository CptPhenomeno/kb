package tui

import (
	"fmt"
	"strings"

	"charm.land/glamour/v2"
	"charm.land/glamour/v2/styles"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/CptPhenomeno/kb/internal/core"
)

// renderDetail renders the detail popup of the selected item, w by h cells.
func (m *Model) renderDetail(w, h int) string {
	it := m.selected()
	iw := w - 4
	var lines []string
	add := func(ls ...string) { lines = append(lines, ls...) }
	if it == nil {
		add(m.th.muted.Render("nothing selected"))
	} else {
		add(spread(m.th.muted.Render(it.ID+" · ")+m.th.fg(m.th.typeColor[it.Type]).Render(it.Type.String()), m.th.muted.Render("esc close"), iw))
		for _, l := range wrap(it.Title, iw, 3) {
			add(m.th.bright.Render(l))
		}
		add("")
		field := func(k, v string) { add(m.th.muted.Render(fmt.Sprintf("%-10s", k)) + v) }
		field("status", m.th.fg(m.th.statusColor[it.Status]).Render(it.Status.Label()))
		field("priority", m.th.fg(m.th.prioColor[it.Priority]).Render(prioMarks[it.Priority]+" "+it.Priority.String()))
		if it.Points != 0 {
			field("points", m.th.text.Render(fmt.Sprint(it.Points)))
		}
		if it.Assignee != "" {
			who := m.th.actor(it.Assignee).Render(it.Assignee)
			if it.Claim != nil {
				now := m.opts.Now()
				if core.ClaimState(it, now) == core.ClaimStale {
					who += " " + m.th.stale.Render("claim expired "+shortDuration(now.Sub(it.Claim.ExpiresAt))+" ago")
				} else {
					who += " " + m.th.muted.Render("claim "+shortDuration(it.Claim.ExpiresAt.Sub(now))+" left")
				}
			}
			field("assignee", who)
		}
		if p := m.snap.Get(it.Parent); p != nil {
			add(hang(m.th.muted.Render(fmt.Sprintf("%-10s", "parent")), 10, p.ID+" "+p.Title, m.th.text, iw, 10)...)
		}
		if it.Spec != "" {
			field("spec", m.th.fg(m.th.p.Purple).Render(it.Spec))
		}
		if it.Phase != core.PhaseNone {
			field("tdd phase", m.th.fg(m.th.phaseColor[it.Phase]).Render("● "+strings.ToUpper(it.Phase.String())))
		}
		if d := strings.TrimSpace(it.Description); d != "" {
			add("")
			add(m.md.render(it.ID, it.Version, d, iw, m.themeIdx)...)
		}
		if children := m.snap.Children(it.ID); len(children) > 0 {
			done, _ := core.Progress(m.snap, it)
			if !it.Type.IsParentType() {
				done = 0
				for _, c := range children {
					if c.Status == core.StatusDone {
						done++
					}
				}
			}
			add("", m.th.muted.Render(fmt.Sprintf("children %d/%d done", done, len(children))))
			for _, c := range children {
				dot := m.th.fg(m.th.statusColor[c.Status]).Render("●")
				add(spread(dot+" "+m.th.muted.Render(c.ID)+" "+m.th.text.Render(c.Title),
					m.th.fg(m.th.statusColor[c.Status]).Render(c.Status.Label()), iw))
			}
		}
		if len(it.Tests) > 0 {
			add("", m.th.muted.Render(fmt.Sprintf("tests %d/%d", it.PassingTests(), len(it.Tests))))
			for _, t := range it.Tests {
				add(hang(m.th.fg(m.th.testColor[t.State]).Render(testMarks[t.State])+" ", 2, t.Name, m.th.text, iw, 2)...)
			}
		}
		if len(it.Acceptance) > 0 {
			add("", m.th.muted.Render(fmt.Sprintf("acceptance %d/%d", it.CheckedCriteria(), len(it.Acceptance))))
			for _, c := range it.Acceptance {
				box := "[ ]"
				if c.Done {
					box = "[x]"
				}
				add(hang(m.th.text.Render(box+" "), 4, c.Text, m.th.text, iw, 4)...)
			}
		}
		if links := linkLines(m, it, iw); len(links) > 0 {
			add("", m.th.muted.Render("links (l to manage, enter there to go)"))
			add(links...)
		}
		add("", m.th.muted.Render("history"))
		hist := m.history
		if len(hist) > 8 {
			hist = hist[len(hist)-8:]
		}
		if len(hist) == 0 {
			add(m.th.muted.Render("loading…"))
		}
		for _, e := range hist {
			prefix := m.th.muted.Render(e.At.In(m.opts.Loc).Format("15:04")) + " " + m.th.actor(e.Actor).Render(e.Actor) + " "
			add(hang(prefix, displayWidth(prefix), core.DescribeEvent(e), m.th.text, iw, 6)...)
		}
	}
	if it != nil {
		lines = m.scrollDetail(it.ID, lines, h-2, iw)
	}
	box := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(m.th.p.Accent).
		BorderBackground(m.th.p.Base).Background(m.th.p.Surface).Width(w).Padding(0, 1)
	return box.Render(paintBlock(fitLines(strings.Join(lines, "\n"), iw, h-2), m.th.p.Surface, iw))
}

// scrollDetail returns the window of the detail lines that fits in height
// rows, with markers for the lines above and below. The scroll goes back
// to the top when another item is shown.
func (m *Model) scrollDetail(id string, lines []string, height, width int) []string {
	if id != m.detailScrollID {
		m.detailScrollID, m.detailScroll = id, 0
	}
	if len(lines) <= height {
		m.detailScroll = 0
		return lines
	}
	window := height - 2 // one row for each marker
	m.detailScroll = max(0, min(m.detailScroll, len(lines)-window))
	s := m.detailScroll
	top := ""
	if s > 0 {
		top = m.th.muted.Render(center(fmt.Sprintf("↑ %d more lines · pgup", s), width))
	}
	out := append([]string{top}, lines[s:min(len(lines), s+window)]...)
	if below := len(lines) - s - window; below > 0 {
		out = append(out, m.th.muted.Render(center(fmt.Sprintf("↓ %d more lines · pgdown", below), width)))
	}
	return out
}

// hang wraps text to width: the first line follows prefix (pw cells wide),
// the next ones are indented by indent cells, so nothing is cut.
func hang(prefix string, pw int, text string, style lipgloss.Style, width, indent int) []string {
	first := wrap(text, max(1, width-pw), 1000)
	if len(first) == 0 {
		return []string{prefix}
	}
	out := []string{prefix + style.Render(first[0])}
	rest := strings.TrimSpace(strings.TrimPrefix(strings.Join(strings.Fields(text), " "), first[0]))
	for _, l := range wrap(rest, max(1, width-indent), 1000) {
		out = append(out, strings.Repeat(" ", indent)+style.Render(l))
	}
	return out
}

// linkLines lists every link of an item with the linked item's title and
// status, after a warning when the item is still blocked.
func linkLines(m *Model, it *core.Item, width int) []string {
	var out []string
	if blocked, by := core.Blockers(m.snap, it.ID); blocked {
		out = append(out, m.th.warn.Render("still blocked by "+strings.Join(by, ", ")))
	}
	for _, l := range core.Links(m.snap, it.ID) {
		left := m.th.muted.Render(fmt.Sprintf("%-14s", l.Label)) + m.th.text.Render(l.Other)
		right := ""
		if o := m.snap.Get(l.Other); o != nil {
			left += " " + m.th.text.Render(o.Title)
			right = m.th.fg(m.th.statusColor[o.Status]).Render(o.Status.Label())
		}
		out = append(out, spread(left, right, width))
	}
	return out
}

// markdown renders item descriptions with Glamour, caching the result per
// item version, width and theme.
type markdown struct {
	cache map[string][]string
}

func newMarkdown() *markdown { return &markdown{cache: map[string][]string{}} }

func (md *markdown) reset() { md.cache = map[string][]string{} }

func (md *markdown) render(id string, version int, src string, width, themeIdx int) []string {
	key := fmt.Sprintf("%s/%d/%d/%d", id, version, width, themeIdx)
	if lines, ok := md.cache[key]; ok {
		return lines
	}
	cfg := styles.DarkStyleConfig
	if Themes[themeIdx].Light {
		cfg = styles.LightStyleConfig
	}
	zero := uint(0)
	cfg.Document.Margin = &zero
	cfg.Document.BlockPrefix, cfg.Document.BlockSuffix = "", ""
	var lines []string
	r, err := glamour.NewTermRenderer(glamour.WithStyles(cfg), glamour.WithWordWrap(width))
	if err == nil {
		var out string
		if out, err = r.Render(src); err == nil {
			for _, l := range strings.Split(out, "\n") {
				lines = append(lines, strings.TrimRight(l, " "))
			}
		}
	}
	if err != nil {
		lines = wrap(src, width, 20)
	}
	for len(lines) > 0 && strings.TrimSpace(ansi.Strip(lines[0])) == "" {
		lines = lines[1:]
	}
	for len(lines) > 0 && strings.TrimSpace(ansi.Strip(lines[len(lines)-1])) == "" {
		lines = lines[:len(lines)-1]
	}
	md.cache[key] = lines
	return lines
}
