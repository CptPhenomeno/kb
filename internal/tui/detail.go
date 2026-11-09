package tui

import (
	"fmt"
	"slices"
	"strings"

	"charm.land/glamour/v2"
	"charm.land/glamour/v2/styles"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/CptPhenomeno/kb/internal/core"
)

// renderDetail renders the side panel of the selected item.
func (m *Model) renderDetail(w, h int) string {
	it := m.board.selected(m)
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
			field("parent", m.th.text.Render(p.ID+" "+p.Title))
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
		if len(it.Tests) > 0 {
			add("", m.th.muted.Render(fmt.Sprintf("tests %d/%d", it.PassingTests(), len(it.Tests))))
			for _, t := range it.Tests {
				add(m.th.fg(m.th.testColor[t.State]).Render(testMarks[t.State]) + " " + m.th.text.Render(t.Name))
			}
		}
		if len(it.Acceptance) > 0 {
			add("", m.th.muted.Render(fmt.Sprintf("acceptance %d/%d", it.CheckedCriteria(), len(it.Acceptance))))
			for _, c := range it.Acceptance {
				box := "[ ]"
				if c.Done {
					box = "[x]"
				}
				add(m.th.text.Render(box + " " + c.Text))
			}
		}
		if rel := relationLines(m, it); len(rel) > 0 {
			add("", m.th.muted.Render("relations"))
			add(rel...)
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
			add(m.th.muted.Render(e.At.In(m.opts.Loc).Format("15:04")) + " " + m.th.actor(e.Actor).Render(e.Actor) + " " +
				m.th.text.Render(core.DescribeEvent(e)))
		}
	}
	box := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(m.th.p.Accent).Width(w).Padding(0, 1)
	return box.Render(strings.Join(fitLines(strings.Join(lines, "\n"), iw, h-2), "\n"))
}

// relationLines lists the relations of an item, blockers first.
func relationLines(m *Model, it *core.Item) []string {
	var blocks, relates, dups []string
	for _, r := range m.snap.Relations {
		switch {
		case r.Kind == core.RelBlocks && r.From == it.ID:
			blocks = append(blocks, r.To)
		case r.Kind == core.RelRelates && (r.From == it.ID || r.To == it.ID):
			other := r.To
			if r.To == it.ID {
				other = r.From
			}
			relates = append(relates, other)
		case r.Kind == core.RelDuplicates && r.From == it.ID:
			dups = append(dups, r.To)
		}
	}
	var out []string
	row := func(label string, ids []string, style lipgloss.Style) {
		if len(ids) > 0 {
			slices.SortFunc(ids, core.CompareIDs)
			out = append(out, m.th.muted.Render(fmt.Sprintf("%-10s", label))+style.Render(strings.Join(ids, ", ")))
		}
	}
	_, by := core.Blockers(m.snap, it.ID)
	row("blocked by", by, m.th.warn)
	row("blocks", blocks, m.th.text)
	row("relates", relates, m.th.text)
	row("duplicates", dups, m.th.text)
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
	if strings.Contains(Themes[themeIdx].Name, "light") || strings.Contains(Themes[themeIdx].Name, "latte") {
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
