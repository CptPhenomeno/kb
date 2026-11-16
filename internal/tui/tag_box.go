package tui

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/CptPhenomeno/kb/internal/core"
)

// minTagBoxWidth is the narrowest box that still shows one character of the
// name: two border cells, two padding cells and the character.
const minTagBoxWidth = 5

// tagBox draws a tag as a rounded box of three rows, border and name in c.
// It sets no background, so the surface under it shows through. A name that
// does not fit maxW is cut with "…"; the box is never wider than maxW, and
// when maxW is below minTagBoxWidth there is no room and the result is nil.
// Board cards and the detail both call it, so they draw the same box.
func (t *theme) tagBox(c color.Color, name string, maxW int) []string {
	if maxW < minTagBoxWidth {
		return nil
	}
	name = truncate(name, maxW-4)
	inner := strings.Repeat("─", displayWidth(name)+2)
	st := lipgloss.NewStyle().Foreground(c)
	return []string{
		st.Render("╭" + inner + "╮"),
		st.Render("│ " + name + " │"),
		st.Render("╰" + inner + "╯"),
	}
}

// tagColor is the theme color of the tag called name, matched without regard
// to case (by core.TagKey). A tag the board does not know, for example one removed since the
// last refresh, is drawn in Muted.
func (m *Model) tagColor(name string) color.Color {
	if tc := m.th.p.TagColor(m.tagColors[core.TagKey(name)]); tc != nil {
		return tc
	}
	return m.th.p.Muted
}

// tagColorsOf maps each tag's core.TagKey to its color name.
func tagColorsOf(tags []core.Tag) map[string]core.TagColor {
	m := make(map[string]core.TagColor, len(tags))
	for _, t := range tags {
		m[core.TagKey(t.Name)] = t.Color
	}
	return m
}
