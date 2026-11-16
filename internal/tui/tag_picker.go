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
}

// pickerRow is one selectable row of the selector. Today every row is an
// existing tag; a row of another kind (creating a tag) joins here.
type pickerRow struct {
	tag core.Tag
}

// pickerRows lists the rows the filter leaves: the existing tags whose name
// contains the query, without regard to case, in name order.
func (m *Model) pickerRows() []pickerRow {
	q := strings.ToLower(m.form.picker.query)
	var rows []pickerRow
	for _, t := range m.tags {
		if strings.Contains(strings.ToLower(t.Name), q) {
			rows = append(rows, pickerRow{tag: t})
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
// toggles the row, esc returns to the form, other text edits the filter.
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
	case "space":
		if p.cursor >= 0 && p.cursor < len(rows) {
			f.toggleTag(rows[p.cursor].tag.Name)
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
		if f.chosenIndex(r.tag.Name) >= 0 {
			box = "[x] "
		}
		lines = append(lines, marker+m.th.text.Render(box)+m.th.tagLabel(m.tagColor(r.tag.Name), r.tag.Name, iw-6))
	}
	if len(rows) == 0 {
		lines = append(lines, m.th.muted.Render("  no tag matches"))
	}
	status := "type to filter · ↑ ↓ move · space adds or removes · esc back to the form"
	foot = append(foot, "", m.th.muted.Render(status))
	avail := max(3, m.bodyHeight()-4-len(head)-len(foot))
	cur := lo + p.cursor
	body := scrollWindow(lines, cur, cur, avail, m.th.muted)
	return m.popupBox(append(append(head, body...), foot...), width, accent)
}
