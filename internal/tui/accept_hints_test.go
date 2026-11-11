package tui

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/CptPhenomeno/kb/internal/core"
)

func TestEmptyPopupInvitesToPressN(t *testing.T) {
	m, _ := newStoryModel(t)
	selectID(t, m, "KB-3")
	press(t, m, "c")
	s := screen(m)
	if !strings.Contains(s, "no acceptance criteria") || !strings.Contains(s, "Press n") {
		t.Fatalf("empty popup does not invite to press n:\n%s", s)
	}
	if strings.Contains(s, "kb accept") {
		t.Fatalf("empty popup still points to the CLI:\n%s", s)
	}
}

func TestCommandBarShowsNEDInThePopup(t *testing.T) {
	m, _ := newAcceptFake(t)
	bar := lastLines(screen(m), 2)
	for _, want := range []string{"↑ ↓ choose", "space check", "n new", "e edit", "d delete", "esc close"} {
		if !strings.Contains(bar, want) {
			t.Errorf("command bar lacks %q:\n%s", want, bar)
		}
	}
	press(t, m, "n")
	bar = lastLines(screen(m), 2)
	for _, want := range []string{"enter save", "esc cancel"} {
		if !strings.Contains(bar, want) {
			t.Errorf("command bar of the input line lacks %q:\n%s", want, bar)
		}
	}
	if strings.Contains(bar, "n new") {
		t.Errorf("command bar of the input line still shows the normal keys:\n%s", bar)
	}
}

func TestCommandBarShowsTheConfirmationKeysWhileDeleting(t *testing.T) {
	m, _ := newAcceptFake(t)
	press(t, m, "d")
	bar := lastLines(screen(m), 2)
	for _, want := range []string{"y/enter delete", "n/esc keep"} {
		if !strings.Contains(bar, want) {
			t.Errorf("command bar lacks %q while deleting:\n%s", want, bar)
		}
	}
	if strings.Contains(bar, "e edit") {
		t.Errorf("command bar still shows the normal keys while deleting:\n%s", bar)
	}
}

func TestHelpListsThePopupKeys(t *testing.T) {
	m := newModel(t, sampleBoard(), 160, 48)
	press(t, m, "?")
	help := screen(m)
	for _, want := range []string{"add a criterion", "edit the selected criterion", "delete the selected criterion", "save the criterion", "cancel", "confirm"} {
		if !strings.Contains(help, want) {
			t.Errorf("help lacks %q:\n%s", want, help)
		}
	}
}

func TestPaletteOffersManageAcceptanceCriteria(t *testing.T) {
	m := newModel(t, sampleBoard(), 160, 48)
	press(t, m, "ctrl+p")
	got := names(m.palette.matches(m))
	if !contains(got, "Manage acceptance criteria") || contains(got, "Check acceptance criteria") {
		t.Fatalf("palette actions: %v", got)
	}
}

func TestAcceptancePopupGolden(t *testing.T) {
	be := sampleBoard()
	for _, it := range be.snap.Items() {
		if it.ID == "KB-4" {
			it.Acceptance = []core.Criterion{{Text: "returns a new access + refresh pair", Done: true},
				{Text: "reused refresh token → 401"}, {Text: "an expired refresh token → 401 with a clear message"}}
		}
	}
	m := New(context.Background(), be, Options{Project: "api-gateway", Actor: "@dev", Now: func() time.Time { return testNow }, Loc: time.UTC})
	m.tickEvery = 0
	send(t, m, windowSize(160, 48))
	runCmd(t, m, m.Init(), 0)
	selectID(t, m, "KB-4")
	press(t, m, "c", "down")
	golden(t, "accept_160x48", screen(m))
}
