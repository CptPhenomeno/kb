package tui

import (
	"context"
	"strings"
	"testing"

	"github.com/CptPhenomeno/kb/internal/core"
)

// newTestsModel is newStoryModel with three tests on KB-3, one per state.
func newTestsModel(t *testing.T) (*Model, *core.Service) {
	t.Helper()
	m, svc := newStoryModel(t)
	ctx := context.Background()
	for _, n := range []string{"parses the token", "rejects an expired token", "refreshes the token"} {
		if _, err := svc.AddTest(ctx, "@dev", "KB-3", n, 0); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := svc.SetTestState(ctx, "@dev", "KB-3", "1", core.TestPassing, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SetTestState(ctx, "@dev", "KB-3", "2", core.TestFailing, 0); err != nil {
		t.Fatal(err)
	}
	runCmd(t, m, m.load(), 0)
	selectID(t, m, "KB-3")
	return m, svc
}

func testStates(t *testing.T, svc *core.Service, id string) []core.TestState {
	t.Helper()
	it, err := svc.Get(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	var out []core.TestState
	for _, x := range it.Tests {
		out = append(out, x.State)
	}
	return out
}

func TestCriteriaPopupOpensOnTheCriteriaTab(t *testing.T) {
	m, _ := newTestsModel(t)
	selectID(t, m, "KB-1")
	press(t, m, "c")
	s := screen(m)
	for _, want := range []string{"Criteria", "Tests", "[ ] opens the board"} {
		if !strings.Contains(s, want) {
			t.Errorf("popup lacks %q", want)
		}
	}
	if m.accept.tab != tabCriteria {
		t.Fatalf("tab = %d, want criteria", m.accept.tab)
	}
}

func TestTabAndShiftTabSwitchBetweenCriteriaAndTests(t *testing.T) {
	m, _ := newTestsModel(t)
	press(t, m, "c")
	press(t, m, "tab")
	if m.accept.tab != tabTests {
		t.Fatal("tab must open Tests")
	}
	press(t, m, "tab")
	if m.accept.tab != tabCriteria {
		t.Fatal("tab again must return to Criteria")
	}
	press(t, m, "shift+tab")
	if m.accept.tab != tabTests {
		t.Fatal("shift+tab must switch tab")
	}
	press(t, m, "shift+tab")
	if m.accept.tab != tabCriteria {
		t.Fatal("shift+tab again must return to Criteria")
	}
}

func TestTestsTabListsTestsWithTheirState(t *testing.T) {
	m, _ := newTestsModel(t)
	press(t, m, "c", "tab")
	s := screen(m)
	for _, want := range []string{"parses the token", "rejects an expired token", "refreshes the token", "passing", "failing", "pending"} {
		if !strings.Contains(s, want) {
			t.Errorf("tests tab lacks %q:\n%s", want, s)
		}
	}
}

func TestTestsTabWithoutTestsSaysSo(t *testing.T) {
	m, _ := newTestsModel(t)
	selectID(t, m, "KB-1")
	press(t, m, "c", "tab")
	if s := screen(m); !strings.Contains(s, "no tests") {
		t.Fatalf("empty tests tab:\n%s", s)
	}
	press(t, m, "space", "down")
}

func TestSpaceCyclesATestPendingPassingFailingPending(t *testing.T) {
	m, svc := newTestsModel(t)
	// third test is pending
	press(t, m, "c", "tab", "down", "down")
	want := []core.TestState{core.TestPassing, core.TestFailing, core.TestPending}
	for i, w := range want {
		press(t, m, "space")
		got := testStates(t, svc, "KB-3")[2]
		if got != w {
			t.Fatalf("after %d spaces the test is %s, want %s", i+1, got, w)
		}
	}
	// the other tests are untouched
	if st := testStates(t, svc, "KB-3"); st[0] != core.TestPassing || st[1] != core.TestFailing {
		t.Fatalf("states = %v", st)
	}
}

func TestSpaceOnCriteriaStillChecksAndUnchecks(t *testing.T) {
	m, svc := newTestsModel(t)
	selectID(t, m, "KB-1")
	press(t, m, "c", "tab", "tab", "space")
	it, _ := svc.Get(context.Background(), "KB-1")
	if !it.Acceptance[0].Done {
		t.Fatal("space on Criteria must check")
	}
	press(t, m, "space")
	if it, _ := svc.Get(context.Background(), "KB-1"); it.Acceptance[0].Done {
		t.Fatal("space again must uncheck")
	}
}

func TestEscAndCCloseThePopupFromTests(t *testing.T) {
	m, _ := newTestsModel(t)
	press(t, m, "c", "tab", "esc")
	if m.accept != nil {
		t.Fatal("esc must close")
	}
	press(t, m, "c", "tab", "c")
	if m.accept != nil {
		t.Fatal("c must close")
	}
}

func TestTestsPopupGolden(t *testing.T) {
	m, _ := newTestsModel(t)
	press(t, m, "c", "tab", "down")
	golden(t, "accept_tests_160x48", screen(m))
}
