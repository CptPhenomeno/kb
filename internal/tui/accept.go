package tui

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/CptPhenomeno/kb/internal/core"
)

// acceptState is the open acceptance popup of an item.
type acceptState struct {
	id         string
	tab        acceptTab
	cursor     int // selected criterion
	testCursor int // selected test
	mode       inputMode
	editAt     int // index of the criterion being edited, in edit mode
	// confirmDelete is set while asking to delete the selected criterion.
	confirmDelete bool
	input         string // text typed in the input line
	// err is the reason the last test save was refused; the input stays open
	// while it is shown.
	err string
}

// acceptTab is the tab of the acceptance popup.
type acceptTab int

const (
	tabCriteria acceptTab = iota
	tabTests
)

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
	a.testCursor = clampCursor(a.testCursor, len(it.Tests))
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
	case "tab", "shift+tab":
		a.tab = tabTests - a.tab
		return nil
	}
	if a.tab == tabTests {
		return m.acceptTestsKey(it, key)
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

// acceptTestsKey handles a key on the Tests tab: the arrows choose a test and
// space cycles its state and saves it at once.
func (m *Model) acceptTestsKey(it *core.Item, key string) tea.Cmd {
	a := m.accept
	a.testCursor = clampCursor(a.testCursor, len(it.Tests))
	switch key {
	case "n":
		a.mode, a.input, a.err = inputAdd, "", ""
	case "e":
		if a.testCursor < len(it.Tests) {
			a.mode, a.editAt, a.input, a.err = inputEdit, a.testCursor, it.Tests[a.testCursor].Name, ""
		}
	case "d":
		a.confirmDelete = a.testCursor < len(it.Tests)
	case "up":
		a.testCursor = max(0, a.testCursor-1)
	case "down":
		a.testCursor = min(max(0, len(it.Tests)-1), a.testCursor+1)
	case "space", "enter":
		if a.testCursor >= len(it.Tests) {
			return nil
		}
		sel, next, item, actor := strconv.Itoa(a.testCursor+1), nextTestState(it.Tests[a.testCursor].State), *it, m.opts.Actor
		return func() tea.Msg {
			if _, err := m.be.SetTestState(m.ctx, actor, item.ID, sel, next, item.Version); err != nil {
				return writeMsg{alert: m.failure(item, "TEST UPDATE FAILED", err)}
			}
			return writeMsg{}
		}
	}
	return nil
}

// nextTestState is the state after s in the cycle
// pending -> passing -> failing -> pending.
func nextTestState(s core.TestState) core.TestState {
	switch s {
	case core.TestPending:
		return core.TestPassing
	case core.TestPassing:
		return core.TestFailing
	}
	return core.TestPending
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
		if a.tab == tabTests {
			sel := strconv.Itoa(a.testCursor + 1)
			return func() tea.Msg {
				if _, err := m.be.RemoveTest(m.ctx, actor, item.ID, sel, item.Version); err != nil {
					return writeMsg{alert: m.failure(item, "DELETE FAILED", err)}
				}
				return writeMsg{}
			}
		}
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
		a.mode, a.err = inputNone, ""
	case "backspace":
		a.err = ""
		if r := []rune(a.input); len(r) > 0 {
			a.input = string(r[:len(r)-1])
		}
	case "enter":
		crit := strings.TrimSpace(a.input)
		if a.tab == tabTests {
			return m.saveTestInput(it, crit)
		}
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
		a.err = ""
		a.input += text
	}
	return nil
}

// saveTestInput saves the test typed in the input line, as a new test or as
// the new name of the selected one. The input stays open until the core accepts
// the name; a refused name (empty, duplicate) is shown in the popup, while a
// version conflict or another failure closes the input and raises the alert.
func (m *Model) saveTestInput(it *core.Item, name string) tea.Cmd {
	a := m.accept
	mode, sel, item, actor := a.mode, strconv.Itoa(a.editAt+1), *it, m.opts.Actor
	return func() tea.Msg {
		title, err := "ADD FAILED", error(nil)
		if mode == inputEdit {
			title = "EDIT FAILED"
			_, err = m.be.EditTest(m.ctx, actor, item.ID, sel, name, item.Version)
		} else {
			_, err = m.be.AddTest(m.ctx, actor, item.ID, name, item.Version)
		}
		var re *core.RuleError
		switch {
		case err == nil:
			return testSavedMsg{}
		case !errors.Is(err, core.ErrConflict) && asRule(err, &re):
			return testSavedMsg{err: re.Msg}
		}
		return testSavedMsg{alert: m.failure(item, title, err)}
	}
}

// testSavedMsg is the outcome of saving the input line of the Tests tab.
type testSavedMsg struct {
	alert *alertState
	err   string // why the core refused the name; empty when saved or alerted
}

// renderAccept shows the criteria and the tests of the item.
func (m *Model) renderAccept() string {
	width := min(72, m.width-4)
	iw := width - 6
	it := m.snap.Get(m.accept.id)
	lines := []string{
		spread(m.th.bright.Render("Criteria and tests"), m.th.muted.Render("esc close"), iw),
		m.th.text.Render(it.ID + " " + it.Title),
		m.renderAcceptTabs(),
		"",
	}
	if m.accept.tab == tabTests {
		return m.renderAcceptTests(it, lines, iw, width)
	}
	if len(it.Acceptance) == 0 {
		lines = append(lines, m.th.muted.Render(it.ID+" has no acceptance criteria."),
			m.th.muted.Render("Press n to add one."))
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
		lines = append(lines, m.renderAcceptConfirm("criterion", cursor+1, it.Acceptance[cursor].Text, iw)...)
	}
	lines = append(lines, "", m.th.muted.Render(fmt.Sprintf("%d/%d checked · status %s · every change is saved at once",
		it.CheckedCriteria(), len(it.Acceptance), it.Status.Label())))
	return m.popupBox(lines, width, m.th.fg(m.th.p.Accent))
}

// renderAcceptTabs shows the tab labels, the active one highlighted.
func (m *Model) renderAcceptTabs() string {
	labels := []string{"Criteria", "Tests"}
	out := make([]string, len(labels))
	for i, l := range labels {
		if acceptTab(i) == m.accept.tab {
			out[i] = m.th.fg(m.th.p.Accent).Bold(true).Render("[" + l + "]")
		} else {
			out[i] = m.th.muted.Render(" " + l + " ")
		}
	}
	return strings.Join(out, " ")
}

// renderAcceptTests completes the popup lines with the Tests tab.
func (m *Model) renderAcceptTests(it *core.Item, lines []string, iw, width int) string {
	if len(it.Tests) == 0 {
		lines = append(lines, m.th.muted.Render(it.ID+" has no tests."),
			m.th.muted.Render("Press n to add one."))
		lines = append(lines, m.renderAcceptInput(iw)...)
		return m.popupBox(lines, width, m.th.fg(m.th.p.Accent))
	}
	cursor := clampCursor(m.accept.testCursor, len(it.Tests))
	for i, t := range it.Tests {
		sym, style := "○", m.th.muted
		switch t.State {
		case core.TestPassing:
			sym, style = "●", m.th.fg(m.th.p.Green)
		case core.TestFailing:
			sym, style = "✗", m.th.warn
		}
		row, marker := m.th.text, "  "
		if i == cursor {
			row, marker = m.th.fg(m.th.p.Accent).Bold(true), m.th.fg(m.th.p.Accent).Render("› ")
		}
		lines = append(lines, hang(marker+style.Render(sym+" "), 6, t.Name+" ("+t.State.String()+")", row, iw, 6)...)
	}
	lines = append(lines, m.renderAcceptInput(iw)...)
	if m.accept.confirmDelete {
		lines = append(lines, "")
		lines = append(lines, m.renderAcceptConfirm("test", cursor+1, it.Tests[cursor].Name, iw)...)
	}
	lines = append(lines, "", m.th.muted.Render(fmt.Sprintf("%d/%d passing · status %s · every change is saved at once",
		it.PassingTests(), len(it.Tests), it.Status.Label())))
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
	if m.accept.err != "" {
		lines = append(lines, m.th.warn.Render(truncate(m.accept.err, w)))
	}
	return lines
}

// renderAcceptConfirm asks to delete the row n (1-based) of the given kind
// ("criterion" or "test") with the given text, truncated to fit the width w.
func (m *Model) renderAcceptConfirm(kind string, n int, text string, w int) []string {
	head := fmt.Sprintf("Delete %s %d ", kind, n)
	q := head + `"` + truncate(text, max(1, w-len([]rune(head))-3)) + `"?`
	return []string{m.th.warn.Bold(true).Render(q),
		m.th.warn.Render("y/enter delete") + m.th.muted.Render(" · n/esc keep")}
}

var acceptKeys = []keyHint{
	{"tab", "switch", "switch between the Criteria and Tests tabs (shift+tab: back)", false},
	{"↑ ↓", "choose", "choose a criterion or a test", false},
	{"space", "check", "Criteria: check or uncheck the criterion (saved at once)", false},
	{"space", "cycle", "Tests: cycle the state of a test, pending, passing, failing (saved at once)", false},
	{"n", "new", "add a criterion or a test", false},
	{"e", "edit", "edit the selected criterion (its check is cleared) or test (back to pending)", false},
	{"d", "delete", "delete the selected criterion or test, after confirmation", false},
	{"esc", "close", "close the popup", false},
}

// acceptBarKeys are the keys of the command bar on the active tab: the
// meaning of space depends on it.
func acceptBarKeys(tab acceptTab) []keyHint {
	out := make([]keyHint, 0, len(acceptKeys))
	for _, h := range acceptKeys {
		if h.key == "space" && (h.label == "check") != (tab == tabCriteria) {
			continue
		}
		out = append(out, h)
	}
	return out
}

var acceptInputKeys = []keyHint{
	{"enter", "save", "save the criterion or test", false},
	{"esc", "cancel", "cancel without saving", false},
}

var acceptConfirmKeys = []keyHint{
	{"y/enter", "delete", "confirm: delete the criterion or test", false},
	{"n/esc", "keep", "keep the criterion or test", false},
}

// hints returns the keys that work in the current state of the popup.
func (a *acceptState) hints() []keyHint {
	switch {
	case a.mode != inputNone:
		return acceptInputKeys
	case a.confirmDelete:
		return acceptConfirmKeys
	}
	return acceptBarKeys(a.tab)
}
