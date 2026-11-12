package tui

import (
	"context"
	"strings"
	"testing"
)

func TestCreateFormOpensOnTheDetailsTab(t *testing.T) {
	m := openForm(t)
	if m.form.tab != formDetails {
		t.Fatalf("tab = %v", m.form.tab)
	}
	s := screen(m)
	for _, want := range []string{"Details", "Criteria", "Tests", "title", "description"} {
		if !strings.Contains(s, want) {
			t.Fatalf("%q missing:\n%s", want, s)
		}
	}
	if strings.Contains(s, "acceptance") {
		t.Fatalf("acceptance shown on the details tab:\n%s", s)
	}
}

func TestCtrlRightAndCtrlLeftSwitchTheFormTabs(t *testing.T) {
	m := openForm(t)
	for _, want := range []formTab{formCriteria, formTests, formDetails} {
		press(t, m, "ctrl+right")
		if m.form.tab != want {
			t.Fatalf("after ctrl+right tab = %v, want %v", m.form.tab, want)
		}
	}
	press(t, m, "ctrl+left")
	if m.form.tab != formTests {
		t.Fatalf("after ctrl+left tab = %v", m.form.tab)
	}
	press(t, m, "ctrl+left")
	if m.form.tab != formCriteria || m.form.focus != fAcceptance {
		t.Fatalf("tab = %v focus = %d", m.form.tab, m.form.focus)
	}
	if s := screen(m); !strings.Contains(s, "acceptance") || strings.Contains(s, "priority") {
		t.Fatalf("criteria tab content wrong:\n%s", s)
	}
}

func TestTabMovesAmongTheFieldsOfTheActiveTab(t *testing.T) {
	m := openForm(t)
	m.form.focus = fDescription
	press(t, m, "tab")
	if m.form.focus != fTitle {
		t.Fatalf("tab from description went to %d, want wrap to title", m.form.focus)
	}
	press(t, m, "up")
	if m.form.focus != fDescription {
		t.Fatalf("up from title went to %d", m.form.focus)
	}
	press(t, m, "ctrl+right")
	press(t, m, "tab")
	if m.form.focus != fAcceptance {
		t.Fatalf("tab on the criteria tab left the field: %d", m.form.focus)
	}
}

func TestCtrlSSavesFromEveryTab(t *testing.T) {
	for n := range 3 {
		m, svc := newLiveModel(t)
		press(t, m, "n")
		typeText(t, m, "Saved from tab")
		for range n {
			press(t, m, "ctrl+right")
		}
		press(t, m, "ctrl+s")
		if m.form != nil {
			t.Fatalf("tab %d: form still open:\n%s", n, screen(m))
		}
		snap, err := svc.Board(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, it := range snap.Items() {
			found = found || it.Title == "Saved from tab"
		}
		if !found {
			t.Fatalf("tab %d: item not created", n)
		}
	}
}

func TestCriteriaEnteredOnTheCriteriaTabAreSaved(t *testing.T) {
	m, _, got := acceptForm(t)
	if m.form.tab != formCriteria {
		t.Fatalf("tab = %v", m.form.tab)
	}
	typeText(t, m, "works")
	press(t, m, "enter")
	press(t, m, "ctrl+left")
	press(t, m, "ctrl+s")
	if s := strings.Join(got("KB-4"), "|"); s != "works" {
		t.Fatalf("criteria = %q", s)
	}
}

func TestEditFormHasNoTabs(t *testing.T) {
	m, _ := newLiveModel(t)
	selectID(t, m, "KB-2")
	press(t, m, "e")
	press(t, m, "ctrl+right")
	if m.form.tab != formDetails {
		t.Fatalf("edit form switched tab: %v", m.form.tab)
	}
	s := screen(m)
	if strings.Contains(s, "Criteria") || !strings.Contains(s, "priority") {
		t.Fatalf("edit form wrong:\n%s", s)
	}
}

func TestFormGoldenWithTabs(t *testing.T) {
	m := openForm(t)
	m.form.fields[fTitle].value = "Add tabs"
	golden(t, "form_tabs_details_160x48", screen(m))
	press(t, m, "ctrl+right")
	m.form.criteria = []string{"first criterion"}
	golden(t, "form_tabs_criteria_160x48", screen(m))
	press(t, m, "ctrl+right")
	golden(t, "form_tabs_tests_160x48", screen(m))
}
