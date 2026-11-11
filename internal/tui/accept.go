package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/CptPhenomeno/kb/internal/core"
)

// acceptState is the open acceptance popup of an item.
type acceptState struct {
	id     string
	cursor int
	adding bool   // the input line for a new criterion is open
	input  string // text typed in the input line
}

func (m *Model) acceptKey(key, text string) tea.Cmd {
	a := m.accept
	it := m.snap.Get(a.id)
	if it == nil {
		m.accept = nil
		return nil
	}
	if a.adding {
		return m.acceptInputKey(it, key, text)
	}
	if key == "esc" || key == "c" {
		m.accept = nil
		return nil
	}
	switch key {
	case "n":
		a.adding, a.input = true, ""
	case "up":
		a.cursor = max(0, a.cursor-1)
	case "down":
		a.cursor = min(max(0, len(it.Acceptance)-1), a.cursor+1)
	case "space", "enter":
		if a.cursor >= len(it.Acceptance) {
			return nil
		}
		n, done, item, actor := a.cursor+1, !it.Acceptance[a.cursor].Done, *it, m.opts.Actor
		return func() tea.Msg {
			if _, err := m.be.SetCriterion(m.ctx, actor, item.ID, n, done, item.Version); err != nil {
				return writeMsg{alert: m.failure(item, "CHECK FAILED", err)}
			}
			return writeMsg{}
		}
	}
	return nil
}

// acceptInputKey handles a key while the input line of a new criterion is
// open: every key but esc, enter and backspace is typed as text.
func (m *Model) acceptInputKey(it *core.Item, key, text string) tea.Cmd {
	a := m.accept
	switch key {
	case "esc":
		a.adding = false
	case "backspace":
		if r := []rune(a.input); len(r) > 0 {
			a.input = string(r[:len(r)-1])
		}
	case "enter":
		crit := strings.TrimSpace(a.input)
		if crit == "" {
			return nil
		}
		a.adding = false
		item, actor := *it, m.opts.Actor
		return func() tea.Msg {
			if _, err := m.be.AddCriterion(m.ctx, actor, item.ID, crit, item.Version); err != nil {
				return writeMsg{alert: m.failure(item, "ADD FAILED", err)}
			}
			return writeMsg{}
		}
	default:
		a.input += text
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
		lines = append(lines, m.renderAcceptInput(iw)...)
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
	lines = append(lines, m.renderAcceptInput(iw)...)
	lines = append(lines, "", m.th.muted.Render(fmt.Sprintf("%d/%d checked · status %s · every change is saved at once",
		it.CheckedCriteria(), len(it.Acceptance), it.Status.Label())))
	return m.popupBox(lines, width, m.th.fg(m.th.p.Accent))
}

// renderAcceptInput shows the input line of a new criterion, wrapped in
// width w with the cursor after the last character.
func (m *Model) renderAcceptInput(w int) []string {
	if !m.accept.adding {
		return nil
	}
	const prefix = "+ "
	in := wrapInput(m.accept.input, w-len(prefix), 0)
	in[len(in)-1] += m.th.fg(m.th.p.Accent).Render("▌")
	lines := make([]string, len(in))
	for i, l := range in {
		lead := "  "
		if i == 0 {
			lead = m.th.fg(m.th.p.Accent).Render(prefix)
		}
		lines[i] = lead + m.th.text.Render(l)
	}
	return lines
}

var acceptKeys = []keyHint{
	{"↑ ↓", "choose", "choose a criterion", false},
	{"space", "check", "check or uncheck the criterion (saved at once)", false},
	{"esc", "close", "close the popup", false},
}
