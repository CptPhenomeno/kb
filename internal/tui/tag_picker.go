package tui

import (
	"slices"
	"strings"

	"github.com/CptPhenomeno/kb/internal/core"
)

// tagPicker is the open tag selector of the form's tags field: a filter text
// and a cursor over the rows it leaves.
type tagPicker struct {
	query  string
	cursor int
	// last is the color of the most recently created tag when the selector
	// opened; color is the color picked on the create row, or "" while the
	// proposal is still the default.
	last  core.TagColor
	color core.TagColor
}

// pendingTag is a tag created in the form that is written to the board only
// when the form is saved.
type pendingTag struct {
	name  string
	color core.TagColor
}

// pickerRow is one selectable row of the selector: an existing or pending
// tag, or the row that creates the typed text as a new tag in color.
type pickerRow struct {
	tag    core.Tag
	create bool
	color  core.TagColor
}

// pendingIndex is the position of the pending tag called name, or -1.
func (f *formState) pendingIndex(name string) int {
	k := core.TagKey(name)
	return slices.IndexFunc(f.newTags, func(t pendingTag) bool { return core.TagKey(t.name) == k })
}

// proposedColor is the color the create row offers: the one picked with
// ← →, or the next in rotation after the last tag created, pending ones
// included.
func (f *formState) proposedColor() core.TagColor {
	p := f.picker
	if p.color != "" {
		return p.color
	}
	last := p.last
	if n := len(f.newTags); n > 0 {
		last = f.newTags[n-1].color
	}
	return core.NextTagColor(last)
}

// pickerRows lists the rows the filter leaves: first the create row when
// the text matches no tag exactly (ignoring case), then the existing and
// pending tags whose name contains the query, without regard to case.
func (m *Model) pickerRows() []pickerRow {
	f := m.form
	text := strings.TrimSpace(f.picker.query)
	q := strings.ToLower(f.picker.query)
	var rows []pickerRow
	all := slices.Clone(m.tags)
	for _, t := range f.newTags {
		all = append(all, core.Tag{Name: t.name, Color: t.color})
	}
	exact := false
	for _, t := range all {
		exact = exact || (text != "" && core.TagKey(t.Name) == core.TagKey(text))
	}
	if _, err := core.ValidateTagName(text); err == nil && !exact {
		rows = append(rows, pickerRow{tag: core.Tag{Name: text}, create: true, color: f.proposedColor()})
	}
	for _, t := range all {
		if strings.Contains(strings.ToLower(t.Name), q) {
			rows = append(rows, pickerRow{tag: t, color: t.Color})
		}
	}
	return rows
}

// chosenIndex is the position of the tag called name among the form's tags
// (by core.TagKey), or -1.
func (f *formState) chosenIndex(name string) int {
	k := core.TagKey(name)
	return slices.IndexFunc(f.tags, func(s string) bool { return core.TagKey(s) == k })
}

// toggleTag adds the tag to the form's tags, or removes it when chosen, and
// refreshes the comma list of the field.
func (f *formState) toggleTag(name string) {
	if i := f.chosenIndex(name); i >= 0 {
		f.tags = slices.Delete(f.tags, i, i+1)
	} else {
		f.tags = append(f.tags, name)
	}
	f.fields[fTags].value = strings.Join(f.tags, ", ")
}

// tagPickerKey handles a key while the selector is open: ↑ ↓ move, space
// toggles the row (on the create row ← → pick its color and space creates
// the tag), esc returns to the form, other text edits the filter.
func (m *Model) tagPickerKey(key, text string) {
	f := m.form
	p := f.picker
	rows := m.pickerRows()
	switch key {
	case "esc":
		f.picker = nil
		return
	case "up":
		p.cursor--
	case "down":
		p.cursor++
	case "left", "right":
		if p.cursor == 0 && len(rows) > 0 && rows[0].create {
			i := slices.Index(core.TagColors, rows[0].color)
			d := 1
			if key == "left" {
				d = len(core.TagColors) - 1
			}
			p.color = core.TagColors[(i+d)%len(core.TagColors)]
		}
		return
	case "space":
		if p.cursor >= 0 && p.cursor < len(rows) {
			if r := rows[p.cursor]; r.create {
				f.newTags = append(f.newTags, pendingTag{name: r.tag.Name, color: r.color})
				f.toggleTag(r.tag.Name)
				p.query, p.color, p.cursor = "", "", 0
			} else {
				f.toggleTag(r.tag.Name)
			}
		}
		return
	case "backspace":
		if r := []rune(p.query); len(r) > 0 {
			p.query = string(r[:len(r)-1])
		}
	default:
		if text == "" {
			return
		}
		p.query += text
	}
	p.cursor = max(0, min(p.cursor, len(m.pickerRows())-1))
}

// renderTagPicker draws the selector in the form popup in place of the
// fields, between the form header and footer.
func (m *Model) renderTagPicker(head, foot []string, width, iw int) string {
	f := m.form
	p := f.picker
	accent := m.th.fg(m.th.p.Accent)
	lines := []string{m.th.muted.Render("filter  ") + m.th.bright.Render(p.query) + accent.Render("▌"), ""}
	rows := m.pickerRows()
	lo := len(lines)
	for i, r := range rows {
		marker, box := "  ", "[ ] "
		if i == p.cursor {
			marker = accent.Render("› ")
		}
		if r.create {
			col := m.th.p.TagColor(r.color)
			if col == nil {
				col = m.th.p.Muted
			}
			lines = append(lines, marker+m.th.text.Render("create ")+m.th.fg(col).Render(`"`+r.tag.Name+`"`)+
				m.th.fg(col).Render("  ‹ "+string(r.color)+" ›"))
			continue
		}
		if f.chosenIndex(r.tag.Name) >= 0 {
			box = "[x] "
		}
		lines = append(lines, marker+m.th.text.Render(box)+m.th.tagLabel(m.tagColor(r.tag.Name), r.tag.Name, iw-6))
	}
	if len(rows) == 0 {
		lines = append(lines, m.th.muted.Render("  no tag matches"))
	}
	status := "type to filter · ↑ ↓ move · space adds or removes · ← → color of a new tag · esc back"
	foot = append(foot, "", m.th.muted.Render(status))
	avail := max(3, m.bodyHeight()-4-len(head)-len(foot))
	cur := lo + p.cursor
	body := scrollWindow(lines, cur, cur, avail, m.th.muted)
	return m.popupBox(append(append(head, body...), foot...), width, accent)
}
