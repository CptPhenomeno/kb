package tui

import (
	"fmt"
	"strings"
	"testing"
)

// formRows returns the content rows of the open form popup, cut out of the
// screen between its side borders and padding, trailing spaces removed.
func formRows(m *Model) []string {
	var rows []string
	x0, x1 := -1, -1
	for _, l := range strings.Split(screen(m), "\n") {
		r := []rune(l)
		if x0 < 0 {
			if i := strings.Index(l, "╭"+strings.Repeat("─", 40)); i >= 0 {
				x0 = len([]rune(l[:i]))
				x1 = x0 + len([]rune(strings.SplitAfter(string(r[x0:]), "╮")[0])) - 1
			}
			continue
		}
		if len(r) <= x1 || r[x0] == '╰' {
			break
		}
		rows = append(rows, strings.TrimRight(string(r[x0+3:x1-2]), " "))
	}
	return rows
}

// fieldRows returns the rows of the field with the given label: its first row
// and the continuation rows under it, up to the next labelled row.
func fieldRows(t *testing.T, m *Model, label, next string) []string {
	t.Helper()
	var out []string
	in := false
	for _, r := range formRows(m) {
		if strings.HasPrefix(r, "› "+label) || strings.HasPrefix(r, "  "+label) {
			in = true
		} else if in && (strings.HasPrefix(r, "  "+next) || strings.HasPrefix(r, "› "+next)) {
			break
		}
		if in {
			out = append(out, r)
		}
	}
	if len(out) == 0 {
		t.Fatalf("field %q not found:\n%s", label, screen(m))
	}
	return out
}

func openForm(t *testing.T) *Model {
	t.Helper()
	m := newModel(t, sampleBoard(), 160, 48)
	press(t, m, "n")
	if m.form == nil {
		t.Fatal("form did not open")
	}
	return m
}

func TestFormLongTitleWrapsInPopupWidth(t *testing.T) {
	m := openForm(t)
	m.form.fields[fTitle].value = strings.Repeat("alpha bravo ", 12)
	rows := fieldRows(t, m, "title", "type")
	if len(rows) < 3 {
		t.Fatalf("title did not wrap:\n%s", screen(m))
	}
	for i, r := range rows {
		if w := len([]rune(r)); w > 70 {
			t.Fatalf("row %d is %d cells, popup inner width is 70: %q", i, w, r)
		}
		if i > 0 && !strings.HasPrefix(r, strings.Repeat(" ", 14)+"alpha") && !strings.HasPrefix(r, strings.Repeat(" ", 14)+"bravo") {
			t.Fatalf("continuation row %d not aligned under the value: %q", i, r)
		}
	}
	if !strings.HasSuffix(rows[len(rows)-1], "▌") {
		t.Fatalf("cursor missing on the last title row: %q", rows[len(rows)-1])
	}
}

func TestFormDescriptionWrapsInPopupWidth(t *testing.T) {
	m := openForm(t)
	m.form.focus = fDescription
	m.form.fields[fDescription].value = strings.Repeat("alpha bravo ", 12)
	rows := fieldRows(t, m, "description", "zzz")
	if len(rows) < 3 {
		t.Fatalf("description did not wrap:\n%s", screen(m))
	}
	for i, r := range rows {
		if w := len([]rune(r)); w > 70 {
			t.Fatalf("row %d is %d cells, popup inner width is 70: %q", i, w, r)
		}
	}
	if !strings.Contains(rows[1], "alpha") || !strings.HasPrefix(rows[1], strings.Repeat(" ", 14)) {
		t.Fatalf("continuation not aligned under the value: %q", rows[1])
	}
}

func TestFormDescriptionOverSixLinesShowsTailWithEllipsis(t *testing.T) {
	m := openForm(t)
	m.form.focus = fDescription
	var words []string
	for i := 1; i <= 60; i++ {
		words = append(words, fmt.Sprintf("word%02d-xxxx", i))
	}
	m.form.fields[fDescription].value = strings.Join(words, " ")
	rows := fieldRows(t, m, "description", "zzz")
	if len(rows) < 6 {
		t.Fatalf("description rows = %d:\n%s", len(rows), screen(m))
	}
	body := rows[:6]
	if first := strings.TrimSpace(string([]rune(body[0])[14:])); !strings.HasPrefix(first, "…") {
		t.Fatalf("first row has no leading ellipsis: %q\n%s", body[0], screen(m))
	}
	joined := strings.Join(body, "\n")
	if !strings.Contains(joined, words[len(words)-1]) || strings.Contains(joined, words[0]) {
		t.Fatalf("not the last lines of the text:\n%s", screen(m))
	}
	if len(rows) > 6 && strings.HasPrefix(rows[6], strings.Repeat(" ", 14)+"word") {
		t.Fatalf("a seventh description row:\n%s", screen(m))
	}
}

func TestFormDescriptionKeepsEditorNewlines(t *testing.T) {
	m := openForm(t)
	m.form.focus = fDescription
	m.form.fields[fDescription].value = "first line\n\nthird after blank\nlast"
	rows := fieldRows(t, m, "description", "zzz")
	want := []string{"first line", "", "third after blank", "last"}
	for i, w := range want {
		got := strings.TrimSpace(string([]rune(rows[i] + strings.Repeat(" ", 14))[14:]))
		if got != w && !(i == 3 && got == w+"▌") {
			t.Fatalf("row %d = %q, want %q\n%s", i, got, w, screen(m))
		}
	}
}

func TestFormCursorVisibleIn500CharDescription(t *testing.T) {
	m := openForm(t)
	m.form.focus = fDescription
	m.form.fields[fDescription].value = strings.Repeat("lorem ipsum ", 42)[:500]
	s := screen(m)
	if !strings.Contains(s, "▌") {
		t.Fatalf("cursor not visible:\n%s", s)
	}
	rows := fieldRows(t, m, "description", "zzz")
	n := 0
	for _, r := range rows {
		if strings.Contains(r, "▌") {
			n++
		}
		if len([]rune(r)) > 70 {
			t.Fatalf("row wider than the popup: %q", r)
		}
	}
	if n != 1 {
		t.Fatalf("cursor appears %d times:\n%s", n, s)
	}
	desc := 0
	for _, r := range rows {
		if strings.HasPrefix(r, strings.Repeat(" ", 14)) || strings.Contains(r, "description") {
			desc++
		}
	}
	if desc > 7 {
		t.Fatalf("description takes %d rows, want at most 6 plus the hint:\n%s", desc, s)
	}
}

func TestFormParentLabelsSpecStayOnOneLine(t *testing.T) {
	m := openForm(t)
	long := strings.Repeat("kb-long-value ", 20)
	m.form.fields[fParent].value = long
	m.form.fields[fLabels].value = long
	m.form.fields[fSpec].value = long
	rows := formRows(m)
	for _, label := range []string{"parent", "labels", "spec"} {
		for i, r := range rows {
			if strings.HasPrefix(r, "  "+label) {
				if i+1 < len(rows) && strings.HasPrefix(rows[i+1], strings.Repeat(" ", 14)) && strings.TrimSpace(rows[i+1]) != "" {
					t.Fatalf("%s continues on a second row:\n%s", label, screen(m))
				}
				if len([]rune(r)) > 70 {
					t.Fatalf("%s row too wide: %q", label, r)
				}
			}
		}
	}
}

func TestFormGoldenWithLongDescription(t *testing.T) {
	m := openForm(t)
	m.form.fields[fTitle].value = "Rotate the refresh token when the session is renewed from a second device"
	m.form.focus = fDescription
	m.form.fields[fDescription].value = strings.Repeat("The refresh token must rotate on every renewal and the old one stops working at once. ", 6)
	golden(t, "form_long_description_160x48", screen(m))
}
