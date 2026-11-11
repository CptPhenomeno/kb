package tui

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/CptPhenomeno/kb/internal/core"
)

// newAcceptFake opens the popup of KB-4 (two criteria, version 7) on a fake
// backend.
func newAcceptFake(t *testing.T) (*Model, *fakeBackend) {
	t.Helper()
	be := sampleBoard()
	for _, it := range be.snap.Items() {
		if it.ID == "KB-4" {
			it.Version = 7
		}
	}
	m := New(context.Background(), be, Options{Project: "demo", Actor: "@dev", Now: func() time.Time { return testNow }, Loc: time.UTC})
	m.tickEvery = 0
	send(t, m, windowSize(160, 48))
	runCmd(t, m, m.Init(), 0)
	selectID(t, m, "KB-4")
	press(t, m, "c")
	return m, be
}

func TestNOpensAnInputLineAtTheBottomOfThePopup(t *testing.T) {
	m, _ := newAcceptFake(t)
	press(t, m, "n")
	lines := strings.Split(screen(m), "\n")
	last, cur := -1, -1
	for i, l := range lines {
		if strings.Contains(l, "reused refresh token") {
			last = i
		}
		if strings.Contains(l, "▌") {
			cur = i
		}
	}
	if cur <= last || last < 0 {
		t.Fatalf("input cursor on line %d, last criterion on line %d; want the input below the list", cur, last)
	}
}

func TestEnterOnTheInputAddsTheCriterionWithTheItemVersion(t *testing.T) {
	m, be := newAcceptFake(t)
	press(t, m, "n")
	typeText(t, m, "it works")
	press(t, m, "enter")
	want := []addCall{{"KB-4", "it works", 7}}
	if len(be.adds) != 1 || be.adds[0] != want[0] {
		t.Fatalf("adds = %+v, want %+v", be.adds, want)
	}
	if m.accept == nil || strings.Contains(screen(m), "▌") {
		t.Fatal("the input must close and the popup stay open")
	}
}

func TestEnterOnABlankInputDoesNothing(t *testing.T) {
	m, be := newAcceptFake(t)
	press(t, m, "n", "space", "space", "enter")
	if len(be.adds) != 0 {
		t.Fatalf("adds = %+v, want none", be.adds)
	}
	if !strings.Contains(screen(m), "▌") {
		t.Fatal("the input must stay open")
	}
}

func TestEscCancelsTheInputAndKeepsThePopupOpen(t *testing.T) {
	m, be := newAcceptFake(t)
	press(t, m, "n")
	typeText(t, m, "draft")
	press(t, m, "esc")
	s := screen(m)
	if m.accept == nil || !strings.Contains(s, "Acceptance") {
		t.Fatal("the popup must stay open")
	}
	if strings.Contains(s, "▌") || strings.Contains(s, "draft") || len(be.adds) != 0 {
		t.Fatal("esc must drop the input without writing")
	}
	press(t, m, "esc")
	if m.accept != nil {
		t.Fatal("a second esc must close the popup")
	}
}

func TestPopupKeysAreTypedAsTextWhileTheInputIsOpen(t *testing.T) {
	m, be := newAcceptFake(t)
	press(t, m, "n", "n", "e", "d", "space", "c", "backspace", "c")
	if m.accept == nil {
		t.Fatal("c must not close the popup while typing")
	}
	press(t, m, "enter")
	if len(be.adds) != 1 || be.adds[0].text != "ned c" {
		t.Fatalf("adds = %+v, want text %q", be.adds, "ned c")
	}
}

func TestLongInputWrapsInThePopupWidthWithTheCursorVisible(t *testing.T) {
	m, _ := newAcceptFake(t)
	press(t, m, "n")
	typeText(t, m, strings.Repeat("alpha beta gamma ", 8)+"omega")
	lines := strings.Split(screen(m), "\n")
	var got []string
	for _, l := range lines {
		if strings.Contains(l, "alpha") || strings.Contains(l, "omega") {
			got = append(got, l)
		}
	}
	if len(got) < 2 {
		t.Fatalf("input not wrapped over several lines:\n%s", screen(m))
	}
	if !strings.Contains(got[len(got)-1], "omega▌") {
		t.Fatalf("last line %q lacks the cursor after the text", got[len(got)-1])
	}
}

func TestAFailedAddShowsTheAlert(t *testing.T) {
	m, be := newAcceptFake(t)
	be.addErr = errors.New("disk full")
	press(t, m, "n")
	typeText(t, m, "x")
	press(t, m, "enter")
	if m.alert == nil {
		t.Fatal("no alert after a failed add")
	}
	s := screen(m)
	for _, want := range []string{"ADD FAILED", "disk full"} {
		if !strings.Contains(s, want) {
			t.Errorf("alert lacks %q", want)
		}
	}
}

func TestServiceBackendAddsACriterion(t *testing.T) {
	m, svc := newStoryModel(t)
	selectID(t, m, "KB-1")
	press(t, m, "c", "n")
	typeText(t, m, "new one")
	press(t, m, "enter")
	it, err := svc.Get(context.Background(), "KB-1")
	if err != nil {
		t.Fatal(err)
	}
	if n := len(it.Acceptance); n != 3 || it.Acceptance[2] != (core.Criterion{Text: "new one"}) {
		t.Fatalf("criteria = %+v, want the new one appended", it.Acceptance)
	}
	if !strings.Contains(screen(m), "[ ] new one") {
		t.Fatal("the popup does not show the new criterion")
	}
}
