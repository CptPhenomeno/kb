package tui

import (
	"context"
	"strings"
	"testing"

	"github.com/CptPhenomeno/kb/internal/core"
)

func testNames(t *testing.T, svc *core.Service, id string) []string {
	t.Helper()
	it, err := svc.Get(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, x := range it.Tests {
		out = append(out, x.Name)
	}
	return out
}

func TestNOnTheTestsTabAddsAPendingTestAtTheEnd(t *testing.T) {
	m, svc := newTestsModel(t)
	press(t, m, "c", "tab", "n")
	typeText(t, m, "keeps the session")
	press(t, m, "enter")
	names := testNames(t, svc, "KB-3")
	if len(names) != 4 || names[3] != "keeps the session" {
		t.Fatalf("tests = %v, want the new one last", names)
	}
	if st := testStates(t, svc, "KB-3"); st[3] != core.TestPending {
		t.Fatalf("new test state = %v, want pending", st[3])
	}
	if !strings.Contains(screen(m), "keeps the session (pending)") || strings.Contains(screen(m), "▌") {
		t.Fatalf("popup must list the test and close the input:\n%s", screen(m))
	}
}

func TestEOnTheTestsTabRenamesTheTestAndSetsItToPending(t *testing.T) {
	m, svc := newTestsModel(t)
	press(t, m, "c", "tab", "e")
	if !strings.Contains(screen(m), "parses the token▌") {
		t.Fatalf("input must start with the test name:\n%s", screen(m))
	}
	typeText(t, m, " fast")
	press(t, m, "enter")
	names := testNames(t, svc, "KB-3")
	if names[0] != "parses the token fast" || names[1] != "rejects an expired token" {
		t.Fatalf("tests = %v, want only the first renamed", names)
	}
	if st := testStates(t, svc, "KB-3"); st[0] != core.TestPending || st[1] != core.TestFailing {
		t.Fatalf("states = %v, want renamed pending and others kept", st)
	}
}

func TestDOnTheTestsTabRemovesTheTestAfterConfirmation(t *testing.T) {
	m, svc := newTestsModel(t)
	press(t, m, "c", "tab", "down", "d")
	if !strings.Contains(screen(m), "Delete test 2") {
		t.Fatalf("no confirmation:\n%s", screen(m))
	}
	press(t, m, "y")
	names := testNames(t, svc, "KB-3")
	if len(names) != 2 || names[0] != "parses the token" || names[1] != "refreshes the token" {
		t.Fatalf("tests = %v", names)
	}
}

func TestDOnTheTestsTabKeepsTheTestWhenRefused(t *testing.T) {
	m, svc := newTestsModel(t)
	press(t, m, "c", "tab", "d", "n")
	if n := len(testNames(t, svc, "KB-3")); n != 3 {
		t.Fatalf("%d tests, want 3", n)
	}
	if m.accept == nil || strings.Contains(screen(m), "Delete test") {
		t.Fatal("confirmation must close and the popup stay open")
	}
}

func TestAnEmptyTestNameShowsAnErrorInThePopup(t *testing.T) {
	m, svc := newTestsModel(t)
	press(t, m, "c", "tab", "e")
	for range len("parses the token") {
		press(t, m, "backspace")
	}
	typeText(t, m, "   ")
	press(t, m, "enter")
	if n := testNames(t, svc, "KB-3"); n[0] != "parses the token" {
		t.Fatalf("tests = %v, want nothing saved", n)
	}
	if m.accept == nil || m.accept.mode == inputNone {
		t.Fatal("the input must stay open")
	}
}

func TestADuplicateTestNameShowsAnErrorInThePopup(t *testing.T) {
	m, svc := newTestsModel(t)
	press(t, m, "c", "tab", "n")
	typeText(t, m, "refreshes the token")
	press(t, m, "enter")
	if len(testNames(t, svc, "KB-3")) != 3 {
		t.Fatal("a duplicate was saved")
	}
	if m.alert != nil {
		t.Fatal("a rule error must show in the popup, not as an alert")
	}
	s := screen(m)
	if !strings.Contains(s, "▌") || !strings.Contains(s, "listed twice") {
		t.Fatalf("input must stay open with the error:\n%s", s)
	}
	typeText(t, m, "!")
	press(t, m, "enter")
	if len(testNames(t, svc, "KB-3")) != 4 {
		t.Fatal("the corrected name was not saved")
	}
	if strings.Contains(screen(m), "listed twice") {
		t.Fatal("the error must clear once the test is saved")
	}
}

func TestAConflictOnATestEditShowsTheAlert(t *testing.T) {
	m, be := newAcceptFake(t)
	be.editErr = core.ErrConflict
	press(t, m, "tab", "e")
	typeText(t, m, "x")
	press(t, m, "enter")
	if m.alert == nil || !strings.Contains(screen(m), "CHANGED MEANWHILE") {
		t.Fatalf("conflict alert missing:\n%s", screen(m))
	}
}
