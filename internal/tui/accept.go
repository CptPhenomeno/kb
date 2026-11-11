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
	mode   inputMode
	editAt int // index of the criterion being edited, in edit mode
	// confirmDelete is set while asking to delete the selected criterion.
	confirmDelete bool
	input         string // text typed in the input line
}

// inputMode says what the input line of the popup is doing.
type inputMode int

const (
	inputNone inputMode = iota
	inputAdd
	inputEdit
)

func (m *Model) acceptKey(key, text string) tea.Cmd {
	a := m.accept
	it := m.snap.Get(a.id)
	if it == nil {
		m.accept = nil
		return nil
	}
	a.cursor = clampCursor(a.cursor, len(it.Acceptance))
	if a.mode != inputNone {
		return m.acceptInputKey(it, key, text)
	}
	if a.confirmDelete {
		return m.acceptDeleteKey(it, key)
	}
	if key == "esc" || key == "c" {
		m.accept = nil
		return nil
	}
	switch key {
	case "n":
		a.mode, a.input = inputAdd, ""
	case "e":
		if a.cursor < len(it.Acceptance) {
			a.mode, a.editAt, a.input = inputEdit, a.cursor, it.Acceptance[a.cursor].Text
		}
	case "d":
		a.confirmDelete = a.cursor < len(it.Acceptance)
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

// clampCursor keeps a cursor on a row of a list of n rows, or on 0 when empty.
func clampCursor(cursor, n int) int {
	return max(0, min(cursor, n-1))
}

// acceptDeleteKey handles a key while the deletion of the selected criterion
// awaits confirmation: y or enter removes it, n or esc keeps it.
func (m *Model) acceptDeleteKey(it *core.Item, key string) tea.Cmd {
	a := m.accept
	switch key {
	case "n", "esc":
		a.confirmDelete = false
	case "y", "enter":
		a.confirmDelete = false
		n, item, actor := a.cursor+1, *it, m.opts.Actor
		return func() tea.Msg {
			if _, err := m.be.RemoveCriterion(m.ctx, actor, item.ID, n, item.Version); err != nil {
				return writeMsg{alert: m.failure(item, "DELETE FAILED", err)}
			}
			return writeMsg{}
		}
	}
	return nil
}

// acceptInputKey handles a key while the input line, to add or to edit a
// criterion, is open: every key but esc, enter and backspace is typed as text.
func (m *Model) acceptInputKey(it *core.Item, key, text string) tea.Cmd {
	a := m.accept
	switch key {
	case "esc":
		a.mode = inputNone
	case "backspace":
		if r := []rune(a.input); len(r) > 0 {
			a.input = string(r[:len(r)-1])
		}
	case "enter":
		crit := strings.TrimSpace(a.input)
		if crit == "" {
			return nil
		}
		mode, n := a.mode, a.editAt+1
		a.mode = inputNone
		item, actor := *it, m.opts.Actor
		return func() tea.Msg {
			title, err := "ADD FAILED", error(nil)
			if mode == inputEdit {
				title = "EDIT FAILED"
				_, err = m.be.EditCriterion(m.ctx, actor, item.ID, n, crit, item.Version)
			} else {
				_, err = m.be.AddCriterion(m.ctx, actor, item.ID, crit, item.Version)
			}
			if err != nil {
				return writeMsg{alert: m.failure(item, title, err)}
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
	cursor := clampCursor(m.accept.cursor, len(it.Acceptance))
	for i, c := range it.Acceptance {
		box := "[ ] "
		if c.Done {
			box = "[x] "
		}
		style := m.th.text
		marker := "  "
		if i == cursor {
			style, marker = m.th.fg(m.th.p.Accent).Bold(true), m.th.fg(m.th.p.Accent).Render("› ")
		}
		lines = append(lines, hang(marker+style.Render(box), 6, c.Text, style, iw, 6)...)
	}
	lines = append(lines, m.renderAcceptInput(iw)...)
	if m.accept.confirmDelete {
		lines = append(lines, "")
		lines = append(lines, m.renderAcceptConfirm(cursor+1, it.Acceptance[cursor].Text, iw)...)
	}
	lines = append(lines, "", m.th.muted.Render(fmt.Sprintf("%d/%d checked · status %s · every change is saved at once",
		it.CheckedCriteria(), len(it.Acceptance), it.Status.Label())))
	return m.popupBox(lines, width, m.th.fg(m.th.p.Accent))
}

// renderAcceptInput shows the input line to add or edit a criterion, wrapped
// in width w with the cursor after the last character.
func (m *Model) renderAcceptInput(w int) []string {
	prefix := "+ "
	switch m.accept.mode {
	case inputNone:
		return nil
	case inputEdit:
		prefix = "✎ "
	}
	in := wrapInput(m.accept.input, w-2, 0)
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

// renderAcceptConfirm asks to delete the criterion n (1-based) with the given
// text, truncated to fit the width w.
func (m *Model) renderAcceptConfirm(n int, text string, w int) []string {
	head := fmt.Sprintf("Delete criterion %d ", n)
	q := head + `"` + truncate(text, max(1, w-len([]rune(head))-3)) + `"?`
	return []string{m.th.warn.Bold(true).Render(q),
		m.th.warn.Render("y/enter delete") + m.th.muted.Render(" · n/esc keep")}
}

var acceptKeys = []keyHint{
	{"↑ ↓", "choose", "choose a criterion", false},
	{"space", "check", "check or uncheck the criterion (saved at once)", false},
	{"esc", "close", "close the popup", false},
}
