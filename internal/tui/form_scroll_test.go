package tui

import (
	"fmt"
	"strings"
	"testing"
)

func openForm80(t *testing.T) *Model {
	t.Helper()
	m := newModel(t, sampleBoard(), 80, 24)
	press(t, m, "n")
	if m.form == nil {
		t.Fatal("form did not open")
	}
	return m
}

func numbered(prefix string, n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = fmt.Sprintf("%s %02d", prefix, i+1)
	}
	return out
}

// assertReads checks the whole popup is on screen: top and bottom border,
// the tab line and the status hint untruncated.
func assertReads(t *testing.T, m *Model) {
	t.Helper()
	s := screen(m)
	if n := len(strings.Split(s, "\n")); n > 24 {
		t.Fatalf("screen has %d rows", n)
	}
	for _, want := range []string{"New item", "ctrl+← → switch tab", "ctrl+s saves", "╰"} {
		if !strings.Contains(s, want) {
			t.Fatalf("%q missing at 80x24:\n%s", want, s)
		}
	}
	if strings.Contains(s, "…  │") || strings.Contains(s, "tab…") {
		t.Fatalf("status hint truncated:\n%s", s)
	}
}

func TestLongTestsListScrollsKeepingTheInputVisible(t *testing.T) {
	m := openForm80(t)
	press(t, m, "ctrl+right", "ctrl+right")
	m.form.tests = numbered("test", 40)
	typeText(t, m, "typing now")
	s := screen(m)
	assertReads(t, m)
	for _, want := range []string{"typing now", "test 40", "earlier"} {
		if !strings.Contains(s, want) {
			t.Fatalf("%q missing:\n%s", want, s)
		}
	}
	if strings.Contains(s, "test 01") {
		t.Fatalf("head of the list shown:\n%s", s)
	}
	golden(t, "form_tabs_tests_long_80x24", s)
}

func TestLongListFormLeavesTheBottomBarVisible(t *testing.T) {
	for _, tab := range []formTab{formCriteria, formTests} {
		m := openForm80(t)
		m.form.show(tab)
		*m.form.listOf(tab.fields()[0]) = numbered("item", 40)
		lines := strings.Split(screen(m), "\n")
		bar := strings.Join(lines[len(lines)-3:], "\n")
		if !strings.Contains(bar, "ctrl+← ctrl+→") {
			t.Fatalf("tab %d: command bar hidden:\n%s", tab, screen(m))
		}
		if strings.Contains(screen(m), "14│") {
			t.Fatalf("tab %d: popup leaks into the activity line:\n%s", tab, screen(m))
		}
	}
}

func TestLongCriteriaListScrollsKeepingTheInputVisible(t *testing.T) {
	m := openForm80(t)
	press(t, m, "ctrl+right")
	m.form.criteria = numbered("criterion", 40)
	typeText(t, m, "typing now")
	s := screen(m)
	assertReads(t, m)
	for _, want := range []string{"typing now", "criterion 40", "earlier"} {
		if !strings.Contains(s, want) {
			t.Fatalf("%q missing:\n%s", want, s)
		}
	}
	golden(t, "form_tabs_criteria_long_80x24", s)
}

func TestFormStatusHintsReadInFullAt80x24(t *testing.T) {
	m := openForm80(t)
	for _, tab := range []formTab{formDetails, formCriteria, formTests} {
		m.form.show(tab)
		assertReads(t, m)
	}
	m.form.focus = fDescription
	assertReads(t, m)
}

func TestDetailsTabWithLongDescriptionFitsAndKeepsFocusVisible(t *testing.T) {
	m := openForm80(t)
	m.form.fields[fTitle].value = strings.Repeat("alpha bravo ", 12)
	m.form.fields[fDescription].value = strings.Join(numbered("line", 30), "\n")
	m.form.focus = fDescription
	assertReads(t, m)
	if s := screen(m); !strings.Contains(s, "line 30") {
		t.Fatalf("description end not shown:\n%s", s)
	}
	m.form.focus = fTitle
	assertReads(t, m)
	if s := screen(m); !strings.Contains(s, "alpha bravo") {
		t.Fatalf("title not shown:\n%s", s)
	}
}

func TestFormGoldenPerTabAt80x24(t *testing.T) {
	m := openForm80(t)
	m.form.fields[fTitle].value = "Add tabs"
	golden(t, "form_tabs_details_80x24", screen(m))
	press(t, m, "ctrl+right")
	m.form.criteria = []string{"first criterion"}
	golden(t, "form_tabs_criteria_80x24", screen(m))
	press(t, m, "ctrl+right")
	m.form.tests = []string{"first test"}
	golden(t, "form_tabs_tests_80x24", screen(m))
}

func TestFormCommandBarAndHelpListTheTabKeys(t *testing.T) {
	m := newModel(t, sampleBoard(), 160, 24)
	press(t, m, "n")
	if s := screen(m); !strings.Contains(s, "ctrl+← ctrl+→") && !strings.Contains(s, "ctrl+←") {
		t.Fatalf("command bar misses the tab keys:\n%s", s)
	}
	found := false
	for _, h := range formKeys {
		found = found || strings.Contains(h.key, "ctrl+←") && strings.Contains(h.key, "ctrl+→")
	}
	if !found {
		t.Fatal("formKeys misses ctrl+← ctrl+→")
	}
}
