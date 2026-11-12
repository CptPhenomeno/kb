package tui

import (
	"context"
	"strings"
	"testing"

	"github.com/CptPhenomeno/kb/internal/core"
)

// testsForm opens the create form on a live service, titled, on the Tests tab.
func testsForm(t *testing.T) (*Model, func(id string) []core.Test) {
	t.Helper()
	m, svc := newLiveModel(t)
	press(t, m, "n")
	typeText(t, m, "Delta task")
	m.form.show(formTests)
	return m, func(id string) []core.Test {
		it, err := svc.Get(context.Background(), id)
		if err != nil {
			t.Fatalf("item %s not created: %v\n%s", id, err, screen(m))
		}
		return it.Tests
	}
}

func TestEnterOnTheTestsTabAddsAPendingTest(t *testing.T) {
	m, _ := testsForm(t)
	if m.form.focus != fTests {
		t.Fatalf("focus = %d", m.form.focus)
	}
	typeText(t, m, "rejects expired token")
	press(t, m, "enter")
	if m.form == nil || m.form.saving {
		t.Fatal("enter saved the form")
	}
	if got := strings.Join(m.form.tests, "|"); got != "rejects expired token" {
		t.Fatalf("tests = %q", got)
	}
	if m.form.fields[fTests].value != "" {
		t.Fatalf("input not cleared: %q", m.form.fields[fTests].value)
	}
	if s := screen(m); !strings.Contains(s, "• rejects expired token") || strings.Contains(s, "No tests yet") {
		t.Fatalf("test not listed:\n%s", s)
	}
	press(t, m, "enter")
	if len(m.form.tests) != 1 || m.form.saving {
		t.Fatal("blank enter had an effect")
	}
}

func TestBackspaceOnAnEmptyTestsInputRemovesTheLastTest(t *testing.T) {
	m, _ := testsForm(t)
	for _, c := range []string{"one", "two"} {
		typeText(t, m, c)
		press(t, m, "enter")
	}
	typeText(t, m, "xy")
	press(t, m, "backspace")
	if m.form.fields[fTests].value != "x" || len(m.form.tests) != 2 {
		t.Fatalf("backspace with text: %q %v", m.form.fields[fTests].value, m.form.tests)
	}
	press(t, m, "backspace", "backspace")
	if got := strings.Join(m.form.tests, "|"); got != "one" {
		t.Fatalf("tests = %q", got)
	}
	if len(m.form.criteria) != 0 {
		t.Fatal("criteria touched")
	}
}

func TestTestTextLeftInTheInputIsAddedOnSave(t *testing.T) {
	m, got := testsForm(t)
	typeText(t, m, "listed")
	press(t, m, "enter")
	typeText(t, m, "pending")
	press(t, m, "ctrl+s")
	tests := got("KB-4")
	if len(tests) != 2 || tests[0].Name != "listed" || tests[1].Name != "pending" {
		t.Fatalf("tests = %+v", tests)
	}
}

func TestCreatedItemHasTheFormTestsPendingInOrder(t *testing.T) {
	m, got := testsForm(t)
	for _, c := range []string{"first", "second", "third"} {
		typeText(t, m, c)
		press(t, m, "enter")
	}
	m.form.show(formCriteria)
	typeText(t, m, "a criterion")
	press(t, m, "ctrl+s")
	tests := got("KB-4")
	if len(tests) != 3 {
		t.Fatalf("tests = %+v", tests)
	}
	for i, want := range []string{"first", "second", "third"} {
		if tests[i].Name != want || tests[i].State != core.TestPending {
			t.Fatalf("test %d = %+v", i, tests[i])
		}
	}
}

func TestDuplicateTestsShowTheErrorNextToTheTestsField(t *testing.T) {
	m, _ := testsForm(t)
	for _, c := range []string{"same", "same"} {
		typeText(t, m, c)
		press(t, m, "enter")
	}
	press(t, m, "ctrl+s")
	if m.form == nil || !strings.Contains(m.form.errs[fTests], "listed twice") {
		t.Fatalf("error not on the tests field:\n%s", screen(m))
	}
}
