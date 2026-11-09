package tui

import (
	"fmt"
	"image/color"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// ResolveTheme picks the theme to start with: KB_THEME, then the user's
// saved choice, then the project's config.toml. "" means the default.
func ResolveTheme(env, user, project string) string {
	for _, t := range []string{env, user, project} {
		if t != "" {
			return t
		}
	}
	return ""
}

// pickerState is the open theme picker. Moving the cursor previews the
// theme; esc goes back to original.
type pickerState struct {
	cursor, original int
}

func (m *Model) openPicker() {
	m.picker = &pickerState{cursor: m.themeIdx, original: m.themeIdx}
}

func (m *Model) pickerKey(key string) tea.Cmd {
	p := m.picker
	switch key {
	case "up":
		p.cursor = (p.cursor + len(Themes) - 1) % len(Themes)
		m.setTheme(p.cursor)
	case "down":
		p.cursor = (p.cursor + 1) % len(Themes)
		m.setTheme(p.cursor)
	case "esc", "t":
		m.setTheme(p.original)
		m.picker = nil
	case "enter":
		m.picker = nil
		name := Themes[m.themeIdx].Name
		m.notice = "theme: " + name
		if m.opts.SaveTheme != nil {
			if err := m.opts.SaveTheme(name); err != nil {
				m.notice += " (could not save it: " + err.Error() + ")"
			} else {
				m.notice += " saved for your user"
			}
		}
	}
	return nil
}

var pickerKeys = []keyHint{
	{"↑ ↓", "preview", "preview the previous or next theme"},
	{"enter", "apply", "use this theme and save it for your user"},
	{"esc", "cancel", "go back to the previous theme"},
}

// renderPicker renders the theme list in the side panel, with each
// theme's colors as swatches.
func (m *Model) renderPicker(w, h int) string {
	iw := w - 4
	lines := []string{
		spread(m.th.bright.Render("Theme"), m.th.muted.Render("esc cancel"), iw),
		m.th.muted.Render("the board previews the selected theme"),
		"",
	}
	for i, p := range Themes {
		marker, name := "  ", m.th.text.Render(p.Name)
		if i == m.picker.cursor {
			marker, name = m.th.fg(m.th.p.Accent).Render("› "), m.th.fg(m.th.p.Accent).Bold(true).Render(p.Name)
		}
		if i == m.picker.original {
			name += m.th.muted.Render(" (current)")
		}
		kind := "dark"
		if p.Light {
			kind = "light"
		}
		lines = append(lines, spread(marker+name, m.th.muted.Render(kind)+" "+swatches(p), iw))
	}
	lines = append(lines, "", m.th.muted.Render(fmt.Sprintf("%d themes · enter saves the choice for your user", len(Themes))))
	box := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(m.th.p.Accent).
		BorderBackground(m.th.p.Base).Background(m.th.p.Surface).Width(w).Padding(0, 1)
	return box.Render(paintBlock(fitLines(strings.Join(lines, "\n"), iw, h-2), m.th.p.Surface, iw))
}

// swatches shows a theme's main colors on its own background.
func swatches(p Palette) string {
	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Background(p.Base).Render(" "))
	for _, c := range []color.Color{p.Accent, p.Agent, p.Human, p.Green, p.Red, p.Purple} {
		b.WriteString(lipgloss.NewStyle().Foreground(c).Background(p.Base).Render("■"))
	}
	b.WriteString(lipgloss.NewStyle().Background(p.Base).Render(" "))
	return b.String()
}
