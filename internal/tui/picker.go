package tui

import (
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

// pickerOrder lists the Themes indexes in the order the picker shows them:
// the dark themes, then the light ones; in each group the kb themes first,
// then the others in Themes order.
func pickerOrder() []int {
	var order []int
	for _, light := range []bool{false, true} {
		for _, kb := range []bool{true, false} {
			for i, p := range Themes {
				if p.Light == light && strings.HasPrefix(p.Name, "kb-") == kb {
					order = append(order, i)
				}
			}
		}
	}
	return order
}

// pickerStep moves the cursor n themes along pickerOrder, wrapping around.
// Headings are not in the order, so they are skipped.
func pickerStep(cursor, n int) int {
	order := pickerOrder()
	for k, i := range order {
		if i == cursor {
			return order[(k+n+len(order))%len(order)]
		}
	}
	return order[0]
}

func (m *Model) pickerKey(key string) tea.Cmd {
	p := m.picker
	switch key {
	case "up":
		p.cursor = pickerStep(p.cursor, -1)
		m.setTheme(p.cursor)
	case "down":
		p.cursor = pickerStep(p.cursor, 1)
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
	{"↑ ↓", "preview", "preview the previous or next theme", false},
	{"enter", "apply", "use this theme and save it for your user", false},
	{"esc", "cancel", "go back to the previous theme", false},
}

// renderPicker renders the theme list as a popup over the board, with
// each theme's colors as swatches; the board behind shows the preview.
func (m *Model) renderPicker() string {
	width := min(64, m.width-4)
	iw := width - 6
	lines := []string{
		spread(m.th.bright.Render("Theme"), m.th.muted.Render("esc cancel"), iw),
		m.th.muted.Render("the board behind previews the selected theme"),
		"",
	}
	var rows []string
	sel := 0 // line of the selected theme in rows
	top := 0 // first line to keep visible: its group heading when it leads the group
	groupOf := func(i int) bool { return Themes[i].Light }
	for k, i := range pickerOrder() {
		if k == 0 || groupOf(i) != groupOf(pickerOrder()[k-1]) {
			heading := "Dark"
			if groupOf(i) {
				heading = "Light"
				rows = append(rows, "")
			}
			rows = append(rows, m.th.bright.Render(heading))
		}
		p := Themes[i]
		if i == m.picker.cursor {
			sel = len(rows)
			top = sel
			if k == 0 || groupOf(i) != groupOf(pickerOrder()[k-1]) {
				top = sel - 1
			}
		}
		marker, name := "  ", m.th.text.Render(p.Name)
		if i == m.picker.cursor {
			marker, name = m.th.fg(m.th.p.Accent).Render("› "), m.th.fg(m.th.p.Accent).Bold(true).Render(p.Name)
		}
		if i == m.picker.original {
			name += m.th.muted.Render(" (current)")
		}
		rows = append(rows, spread("  "+marker+name, swatches(p), iw))
	}
	// The popup adds a border and padding (4 rows) around header, list and
	// footer; the list scrolls with the selection when the rest does not fit.
	avail := max(3, m.height-4-len(lines)-2)
	lines = append(lines, scrollWindowAt(rows, top, sel, avail, m.th.muted, 4)...)
	lines = append(lines, "", m.th.muted.Render("enter saves the choice for your user"))
	return m.popupBox(lines, width, m.th.fg(m.th.p.Accent))
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
