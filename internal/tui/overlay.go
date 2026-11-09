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
	unmet                   []string
	footer                  string
}

// moveAlert explains why a move did not happen.
func (m *Model) moveAlert(it *core.Item, to core.Status, err error) *alertState {
	a := &alertState{title: "MOVE FAILED", headline: fmt.Sprintf("%s %s can't move to %s", it.ID, it.Title, to.Label()),
		footer: fmt.Sprintf("The card stays in %s.", it.Status.Label())}
	var re *core.RuleError
	switch {
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
		lines = append(lines, "", m.th.muted.Render("REQUIREMENTS"))
		for _, u := range a.unmet {
			lines = append(lines, hang(m.th.warn.Render("✗ "), 2, u, m.th.text, width-6, 2)...)
		}
	}
	lines = append(lines, "", spread(m.th.muted.Render(a.footer), m.th.chip.Render(" enter ")+" "+m.th.muted.Render("OK"), width-6))
	return m.popupBox(lines, width, m.th.warn)
}

var alertKeys = []keyHint{{"enter", "ok", "close the alert"}, {"esc", "close", "close the alert"}}

func asRule(err error, target **core.RuleError) bool { return errors.As(err, target) }
