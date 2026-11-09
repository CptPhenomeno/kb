package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
)

// acceptState is the open acceptance popup of an item.
type acceptState struct {
	id     string
	cursor int
}

func (m *Model) acceptKey(key string) tea.Cmd {
	a := m.accept
	it := m.snap.Get(a.id)
	if it == nil || key == "esc" || key == "c" {
		m.accept = nil
		return nil
	}
	switch key {
	case "up":
		a.cursor = max(0, a.cursor-1)
	case "down":
		a.cursor = min(max(0, len(it.Acceptance)-1), a.cursor+1)
	case "space", "enter":
		if a.cursor >= len(it.Acceptance) {
			return nil
		}
		n, done, version, actor := a.cursor+1, !it.Acceptance[a.cursor].Done, it.Version, m.opts.Actor
		id := it.ID
		return func() tea.Msg {
			if _, err := m.be.SetCriterion(m.ctx, actor, id, n, done, version); err != nil {
				return writeMsg{alert: &alertState{title: "CHECK FAILED", headline: fmt.Sprintf("criterion %d of %s was not saved", n, id),
					detail: err.Error(), footer: "The board is reloaded."}}
			}
			return writeMsg{}
		}
	}
	return nil
}

// renderAccept shows the criteria of the item with their checks.
func (m *Model) renderAccept() string {
	width := min(72, m.width-4)
	iw := width - 6
	it := m.snap.Get(m.accept.id)
	lines := []string{
		spread(m.th.bright.Render("Acceptance"), m.th.muted.Render("esc close"), iw),
		m.th.text.Render(it.ID + " " + it.Title),
		"",
	}
	if len(it.Acceptance) == 0 {
		lines = append(lines, m.th.muted.Render(it.ID+" has no acceptance criteria."),
			m.th.muted.Render(fmt.Sprintf("Add them with: kb accept %s add \"<criterion>\"", it.ID)))
		return m.popupBox(lines, width, m.th.fg(m.th.p.Accent))
	}
	for i, c := range it.Acceptance {
		box := "[ ] "
		if c.Done {
			box = "[x] "
		}
		style := m.th.text
		marker := "  "
		if i == m.accept.cursor {
			style, marker = m.th.fg(m.th.p.Accent).Bold(true), m.th.fg(m.th.p.Accent).Render("› ")
		}
		lines = append(lines, hang(marker+style.Render(box), 6, c.Text, style, iw, 6)...)
	}
	lines = append(lines, "", m.th.muted.Render(fmt.Sprintf("%d/%d checked · status %s · every change is saved at once",
		it.CheckedCriteria(), len(it.Acceptance), it.Status.Label())))
	return m.popupBox(lines, width, m.th.fg(m.th.p.Accent))
}

var acceptKeys = []keyHint{
	{"↑ ↓", "choose", "choose a criterion", false},
	{"space", "check", "check or uncheck the criterion (saved at once)", false},
	{"esc", "close", "close the popup", false},
}
