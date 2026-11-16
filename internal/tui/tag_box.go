package tui

import (
	"image/color"

	"charm.land/lipgloss/v2"

	"github.com/CptPhenomeno/kb/internal/core"
)

// minTagLabelWidth is the narrowest tag label: both brackets and one cell for
// the name.
const minTagLabelWidth = 3

// tagLabel draws a tag on one line as its name between ⟨ and ⟩, brackets and
// name in c, with no background and no spaces inside. A name that does not
// fit maxW is cut with "…" inside the brackets; the label is never wider
// than maxW, and below minTagLabelWidth there is no room and it is "".
func (t *theme) tagLabel(c color.Color, name string, maxW int) string {
	if maxW < minTagLabelWidth {
		return ""
	}
	name = truncate(name, maxW-2)
	return lipgloss.NewStyle().Foreground(c).Render("⟨" + name + "⟩")
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
