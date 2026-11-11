package tui

import (
	"context"
	"strings"
	"testing"

	"github.com/CptPhenomeno/kb/internal/core"
)

// acceptForm opens the create form on a live service with the focus on the
// acceptance field.
func acceptForm(t *testing.T) (*Model, *core.Service, func(id string) []string) {
	t.Helper()
	m, svc := newLiveModel(t)
	press(t, m, "n")
	typeText(t, m, "Delta task")
	m.form.focus = fAcceptance
	return m, svc, func(id string) []string {
		it, err := svc.Get(context.Background(), id)
		if err != nil {
			t.Fatalf("item %s not created: %v\n%s", id, err, screen(m))
		}
		var out []string
		for _, c := range it.Acceptance {
			if c.Done {
				t.Fatalf("criterion %q was created checked", c.Text)
			}
			out = append(out, c.Text)
		}
		return out
	}
}

func TestCreateFormHasAnAcceptanceFieldAfterTheDescription(t *testing.T) {
	m := openForm(t)
	if fAcceptance != fDescription+1 || len(m.form.fields) != fAcceptance+1 {
		t.Fatalf("fields = %d", len(m.form.fields))
	}
	if m.form.fields[fAcceptance].label != "acceptance" {
		t.Fatalf("label = %q", m.form.fields[fAcceptance].label)
	}
	m.form.focus = fDescription
	press(t, m, "tab")
	if m.form.focus != fAcceptance {
		t.Fatal("tab from the description did not reach acceptance")
	}
	press(t, m, "tab")
	if m.form.focus != fTitle {
		t.Fatal("tab from acceptance did not wrap to the title")
	}
	if !strings.Contains(screen(m), "acceptance") {
		t.Fatalf("field not rendered:\n%s", screen(m))
	}
}

func TestEditFormHasNoAcceptanceField(t *testing.T) {
	m, _ := newLiveModel(t)
	selectID(t, m, "KB-2")
	press(t, m, "e")
	if len(m.form.fields) != fAcceptance {
		t.Fatalf("edit form has %d fields", len(m.form.fields))
	}
	if strings.Contains(screen(m), "acceptance") {
		t.Fatalf("edit form shows acceptance:\n%s", screen(m))
	}
	m.form.focus = fDescription
	press(t, m, "tab")
	if m.form.focus != fTitle {
		t.Fatal("tab from the description did not wrap to the title")
	}
}

func TestEnterAddsTheCriterionAndClearsTheInput(t *testing.T) {
	m, _, _ := acceptForm(t)
	typeText(t, m, "first one")
	press(t, m, "enter")
	if m.form == nil || m.form.saving {
		t.Fatal("enter saved the form")
	}
	if got := strings.Join(m.form.criteria, "|"); got != "first one" {
		t.Fatalf("criteria = %q", got)
	}
	if m.form.fields[fAcceptance].value != "" {
		t.Fatalf("input not cleared: %q", m.form.fields[fAcceptance].value)
	}
	if !strings.Contains(screen(m), "• first one") {
		t.Fatalf("criterion not listed:\n%s", screen(m))
	}
}

func TestEnterOnABlankAcceptanceInputDoesNothing(t *testing.T) {
	m, svc := newLiveModel(t)
	press(t, m, "n")
	typeText(t, m, "Delta task")
	m.form.focus = fAcceptance
	before := boardSize(t, svc)
	press(t, m, "enter")
	typeText(t, m, "  ")
	press(t, m, "enter")
	if m.form == nil || m.form.saving || len(m.form.criteria) != 0 {
		t.Fatalf("blank enter had an effect: %+v", m.form)
	}
	if boardSize(t, svc) != before {
		t.Fatal("blank enter created the item")
	}
}

func TestBackspaceOnAnEmptyInputRemovesTheLastCriterion(t *testing.T) {
	m, _, _ := acceptForm(t)
	for _, c := range []string{"one", "two"} {
		typeText(t, m, c)
		press(t, m, "enter")
	}
	typeText(t, m, "xy")
	press(t, m, "backspace")
	if m.form.fields[fAcceptance].value != "x" || len(m.form.criteria) != 2 {
		t.Fatalf("backspace with text: %q %v", m.form.fields[fAcceptance].value, m.form.criteria)
	}
	press(t, m, "backspace", "backspace")
	if got := strings.Join(m.form.criteria, "|"); got != "one" {
		t.Fatalf("criteria = %q, want one", got)
	}
	press(t, m, "backspace", "backspace")
	if len(m.form.criteria) != 0 {
		t.Fatalf("criteria = %v", m.form.criteria)
	}
}

func TestLongCriteriaWrapLikeInThePopup(t *testing.T) {
	m := openForm(t)
	m.form.focus = fAcceptance
	m.form.criteria = []string{"the first criterion is long enough that it has to wrap over several lines of the form popup"}
	rows := fieldRows(t, m, "acceptance", "zzz")
	var list []string
	for _, r := range rows {
		if strings.Contains(r, "•") || strings.HasPrefix(r, strings.Repeat(" ", 16)) && strings.TrimSpace(r) != "" {
			list = append(list, r)
		}
	}
	if len(list) < 2 {
		t.Fatalf("criterion not wrapped:\n%s", strings.Join(rows, "\n"))
	}
	if !strings.HasPrefix(list[0], strings.Repeat(" ", 14)+"• ") && !strings.Contains(list[0], "• the first") {
		t.Fatalf("first row = %q", list[0])
	}
	for _, r := range list[1:] {
		if !strings.HasPrefix(r, strings.Repeat(" ", 16)) || strings.HasPrefix(r, strings.Repeat(" ", 17)) {
			t.Fatalf("continuation not indented under the text: %q", r)
		}
	}
	joined := strings.Join(strings.Fields(strings.Join(list, " ")), " ")
	if !strings.Contains(joined, "over several lines of the form popup") {
		t.Fatalf("text lost: %q", joined)
	}
}

func TestCreateSendsTheCriteriaUnchecked(t *testing.T) {
	m, _, got := acceptForm(t)
	for _, c := range []string{"first", "second"} {
		typeText(t, m, c)
		press(t, m, "enter")
	}
	press(t, m, "ctrl+s")
	if s := strings.Join(got("KB-4"), "|"); s != "first|second" {
		t.Fatalf("criteria = %q", s)
	}
}

func TestPendingCriterionTextIsAddedOnSave(t *testing.T) {
	for _, key := range []string{"ctrl+s", "enter"} {
		m, _, got := acceptForm(t)
		typeText(t, m, "listed")
		press(t, m, "enter")
		typeText(t, m, "pending")
		if key == "enter" {
			m.form.focus = fTitle
		}
		press(t, m, key)
		if s := strings.Join(got("KB-4"), "|"); s != "listed|pending" {
			t.Fatalf("%s: criteria = %q", key, s)
		}
	}
}

func TestDuplicateCriteriaShowTheValidationError(t *testing.T) {
	m, svc, _ := acceptForm(t)
	for _, c := range []string{"same", "same"} {
		typeText(t, m, c)
		press(t, m, "enter")
	}
	before := boardSize(t, svc)
	press(t, m, "ctrl+s")
	if m.form == nil {
		t.Fatal("form closed on a validation error")
	}
	rows := strings.Join(fieldRows(t, m, "acceptance", "zzz"), "\n")
	if !strings.Contains(rows, "listed twice") {
		t.Fatalf("error not next to the acceptance field:\n%s", screen(m))
	}
	if boardSize(t, svc) != before {
		t.Fatal("an invalid form created an item")
	}
}
