package tui

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/CptPhenomeno/kb/internal/core"
)

func TestEOpensTheInputPrefilledWithTheSelectedCriterion(t *testing.T) {
	m, _ := newAcceptFake(t)
	press(t, m, "down", "e")
	if !strings.Contains(screen(m), "reused refresh token → 401▌") {
		t.Fatalf("input not prefilled with the cursor at the end:\n%s", screen(m))
	}
}

func TestEnterSavesTheEditWithTheItemVersion(t *testing.T) {
	m, be := newAcceptFake(t)
	press(t, m, "down", "e")
	typeText(t, m, " twice")
	press(t, m, "enter")
	want := editCall{"KB-4", 2, "reused refresh token → 401 twice", 7}
	if len(be.edits) != 1 || be.edits[0] != want {
		t.Fatalf("edits = %+v, want %+v", be.edits, want)
	}
	if len(be.adds) != 0 {
		t.Fatalf("adds = %+v, want none", be.adds)
	}
	if m.accept == nil || strings.Contains(screen(m), "▌") {
		t.Fatal("the input must close and the popup stay open")
	}
}

func TestEditingACheckedCriterionReopensTheDoneStory(t *testing.T) {
	m, svc := newStoryModel(t)
	selectID(t, m, "KB-1")
	press(t, m, "c", "space", "down", "space", "esc")
	if got := status(t, svc, "KB-1"); got != core.StatusDone {
		t.Fatalf("story is %s, want done before the edit", got)
	}
	press(t, m, "c", "e")
	typeText(t, m, "!")
	press(t, m, "enter")
	if got := status(t, svc, "KB-1"); got == core.StatusDone {
		t.Fatal("story still done after editing a checked criterion")
	}
	it, err := svc.Get(context.Background(), "KB-1")
	if err != nil {
		t.Fatal(err)
	}
	if it.Acceptance[0] != (core.Criterion{Text: "opens the board!"}) {
		t.Fatalf("criterion = %+v, want edited and unchecked", it.Acceptance[0])
	}
}

func TestEscCancelsTheEditWithoutWriting(t *testing.T) {
	m, be := newAcceptFake(t)
	press(t, m, "e")
	typeText(t, m, "zzz")
	press(t, m, "esc")
	s := screen(m)
	if m.accept == nil || !strings.Contains(s, "Acceptance") {
		t.Fatal("the popup must stay open")
	}
	if strings.Contains(s, "▌") || strings.Contains(s, "zzz") || len(be.edits) != 0 {
		t.Fatal("esc must drop the input without writing")
	}
}

func TestEOnAnItemWithoutCriteriaDoesNothing(t *testing.T) {
	m, _ := newStoryModel(t)
	selectID(t, m, "KB-3")
	press(t, m, "c", "e")
	if strings.Contains(screen(m), "▌") {
		t.Fatal("e must not open the input without criteria")
	}
	press(t, m, "enter")
	if m.accept == nil {
		t.Fatal("the popup must stay open")
	}
}

func TestAVersionConflictOnEditShowsTheAlert(t *testing.T) {
	m, be := newAcceptFake(t)
	be.editErr = core.ErrConflict
	press(t, m, "e")
	typeText(t, m, "x")
	press(t, m, "enter")
	if m.alert == nil {
		t.Fatal("no alert after a conflicting edit")
	}
	if !strings.Contains(screen(m), "CHANGED MEANWHILE") {
		t.Fatalf("conflict alert missing:\n%s", screen(m))
	}
	be.editErr = errors.New("disk full")
	m.alert = nil
	press(t, m, "e")
	press(t, m, "enter")
	if m.alert == nil || !strings.Contains(screen(m), "EDIT FAILED") {
		t.Fatalf("generic error alert missing:\n%s", screen(m))
	}
}

func TestServiceBackendEditsACriterion(t *testing.T) {
	m, svc := newStoryModel(t)
	selectID(t, m, "KB-1")
	press(t, m, "c", "down", "e")
	typeText(t, m, " fast")
	press(t, m, "enter")
	it, err := svc.Get(context.Background(), "KB-1")
	if err != nil {
		t.Fatal(err)
	}
	if it.Acceptance[1].Text != "shows writes within a second fast" || it.Acceptance[0].Text != "opens the board" {
		t.Fatalf("criteria = %+v, want only the second edited", it.Acceptance)
	}
	if !strings.Contains(screen(m), "[ ] shows writes within a second fast") {
		t.Fatal("the popup does not show the edited criterion")
	}
}
