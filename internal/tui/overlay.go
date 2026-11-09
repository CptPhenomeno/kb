package tui

import (
	"errors"
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/CptPhenomeno/kb/internal/core"
)

// overlay draws popup centered over the full-screen base.
func overlay(base, popup string, width, height int) string {
	pw, ph := lipgloss.Width(popup), lipgloss.Height(popup)
	x, y := max(0, (width-pw)/2), max(0, (height-ph)/2)
	return lipgloss.NewCompositor(
		lipgloss.NewLayer(base),
		lipgloss.NewLayer(popup).X(x).Y(y).Z(1),
	).Render()
}

// popupBox frames popup content on the surface color.
func (m *Model) popupBox(lines []string, width int, border lipgloss.Style) string {
	inner := width - 6
	box := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(border.GetForeground()).
		BorderBackground(m.th.p.Surface).Background(m.th.p.Surface).Width(width).Padding(1, 2)
	return box.Render(paintBlock(fitLines(strings.Join(lines, "\n"), inner, len(lines)), m.th.p.Surface, inner))
}

// alertState is an open alert: a refused or failed action.
type alertState struct {
	title, headline, detail string
	listTitle               string // heading of unmet; "REQUIREMENTS" when empty
	unmet                   []string
	footer                  string
}

// moveAlert explains why a move did not happen.
func (m *Model) moveAlert(it *core.Item, to core.Status, err error) *alertState {
	a := &alertState{title: "MOVE FAILED", headline: fmt.Sprintf("%s %s can't move to %s", it.ID, it.Title, to.Label()),
		footer: fmt.Sprintf("The card stays in %s.", it.Status.Label())}
	var re *core.RuleError
	switch {
	case errors.Is(err, core.ErrConflict):
		return m.conflictAlert(*it)
	case asRule(err, &re) && len(re.Unmet) > 0:
		a.title = "MOVE REFUSED"
		a.detail = fmt.Sprintf("%s → %s · %s", it.Status.Label(), to.Label(), plural(len(re.Unmet), "requirement not met", "requirements not met"))
		a.unmet = re.Unmet
	default:
		a.detail = err.Error()
	}
	return a
}

func (m *Model) renderAlert() string {
	a := m.alert
	width := min(72, m.width-4)
	lines := []string{
		m.th.warn.Bold(true).Render(a.title),
		m.th.bright.Render(a.headline),
		m.th.muted.Render(a.detail),
	}
	if len(a.unmet) > 0 {
		heading := a.listTitle
		if heading == "" {
			heading = "REQUIREMENTS"
		}
		lines = append(lines, "", m.th.muted.Render(heading))
		for _, u := range a.unmet {
			lines = append(lines, hang(m.th.warn.Render("✗ "), 2, u, m.th.text, width-6, 2)...)
		}
	}
	lines = append(lines, "", spread(m.th.muted.Render(a.footer), m.th.chip.Render(" enter ")+" "+m.th.muted.Render("OK"), width-6))
	return m.popupBox(lines, width, m.th.warn)
}

var alertKeys = []keyHint{{"enter", "ok", "close the alert", false}, {"esc", "close", "close the alert", false}}

func asRule(err error, target **core.RuleError) bool { return errors.As(err, target) }

// conflictAlert explains that an item changed between the moment the TUI
// read it and the write: nothing was overwritten. It runs inside a command,
// so it reads a fresh copy of the board instead of the model's.
func (m *Model) conflictAlert(old core.Item) *alertState {
	a := &alertState{title: "CHANGED MEANWHILE", listTitle: "WHAT CHANGED",
		headline: fmt.Sprintf("%s %s changed while you were looking", old.ID, old.Title),
		footer:   "Nothing was overwritten. Look again and retry."}
	snap, err := m.be.Board(m.ctx)
	if err != nil {
		a.detail = err.Error()
		return a
	}
	fresh := snap.Get(old.ID)
	if fresh == nil {
		a.detail = "it was deleted"
		return a
	}
	if evs, err := m.be.Log(m.ctx, core.LogFilter{Item: old.ID, Limit: 1}); err == nil && len(evs) > 0 {
		a.detail = fmt.Sprintf("by %s · %s", evs[0].Actor, core.DescribeEvent(evs[0]))
	}
	a.unmet = diffItems(&old, fresh)
	return a
}

// diffItems lists the fields that differ between two versions of an item.
func diffItems(a, b *core.Item) []string {
	var out []string
	add := func(field, from, to string) {
		if from != to {
			out = append(out, fmt.Sprintf("%s: %s → %s", field, orDash(from), orDash(to)))
		}
	}
	add("status", a.Status.Label(), b.Status.Label())
	add("title", a.Title, b.Title)
	add("assignee", a.Assignee, b.Assignee)
	add("priority", a.Priority.String(), b.Priority.String())
	add("points", fmt.Sprint(a.Points), fmt.Sprint(b.Points))
	add("tdd phase", a.Phase.String(), b.Phase.String())
	add("parent", a.Parent, b.Parent)
	add("tests passing", fmt.Sprintf("%d/%d", a.PassingTests(), len(a.Tests)), fmt.Sprintf("%d/%d", b.PassingTests(), len(b.Tests)))
	add("acceptance", fmt.Sprintf("%d/%d", a.CheckedCriteria(), len(a.Acceptance)), fmt.Sprintf("%d/%d", b.CheckedCriteria(), len(b.Acceptance)))
	if a.Description != b.Description {
		out = append(out, "description edited")
	}
	if len(out) == 0 {
		out = append(out, "only its version changed (for example a renewed claim)")
	}
	return out
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// failure builds the alert of a write that did not happen.
func (m *Model) failure(old core.Item, title string, err error) *alertState {
	if errors.Is(err, core.ErrConflict) {
		return m.conflictAlert(old)
	}
	a := &alertState{title: title, headline: old.ID + " " + old.Title, detail: err.Error(), footer: "The board is reloaded."}
	var re *core.RuleError
	if asRule(err, &re) {
		a.detail, a.unmet = re.Msg, re.Unmet
	}
	return a
}
