package tui

import (
	"context"
	"strings"
	"testing"

	"github.com/CptPhenomeno/kb/internal/core"
)

func TestDAsksForConfirmation(t *testing.T) {
	m, be := newAcceptFake(t)
	press(t, m, "down", "d")
	if !strings.Contains(screen(m), `Delete criterion 2 "reused refresh token → 401"?`) {
		t.Fatalf("confirmation missing:\n%s", screen(m))
	}
	if len(be.removes) != 0 {
		t.Fatalf("removes = %+v, want none before confirming", be.removes)
	}
	press(t, m, "x", "up")
	if !strings.Contains(screen(m), "Delete criterion 2") {
		t.Fatal("other keys must leave the confirmation open")
	}
}

func TestYConfirmsAndRemovesTheCriterion(t *testing.T) {
	m, be := newAcceptFake(t)
	press(t, m, "down", "d", "y")
	want := removeCall{"KB-4", 2, 7}
	if len(be.removes) != 1 || be.removes[0] != want {
		t.Fatalf("removes = %+v, want %+v", be.removes, want)
	}
	if m.accept == nil || strings.Contains(screen(m), "Delete criterion") {
		t.Fatal("the confirmation must close and the popup stay open")
	}
}

func TestEnterConfirmsTheRemoval(t *testing.T) {
	m, be := newAcceptFake(t)
	press(t, m, "d", "enter")
	want := removeCall{"KB-4", 1, 7}
	if len(be.removes) != 1 || be.removes[0] != want {
		t.Fatalf("removes = %+v, want %+v", be.removes, want)
	}
}

func TestNCancelsTheRemoval(t *testing.T) {
	m, be := newAcceptFake(t)
	press(t, m, "d")
	if !strings.Contains(screen(m), "Delete criterion 1") {
		t.Fatal("d must open the confirmation")
	}
	press(t, m, "n")
	if len(be.removes) != 0 || m.accept == nil || strings.Contains(screen(m), "Delete criterion") {
		t.Fatal("n must close the confirmation without removing")
	}
}

func TestEscCancelsTheConfirmationAndKeepsThePopupOpen(t *testing.T) {
	m, be := newAcceptFake(t)
	press(t, m, "d", "esc")
	if len(be.removes) != 0 || m.accept == nil || strings.Contains(screen(m), "Delete criterion") {
		t.Fatal("esc must close only the confirmation")
	}
	press(t, m, "esc")
	if m.accept != nil {
		t.Fatal("a second esc must close the popup")
	}
}

func TestCursorStaysInRangeAfterRemovingTheLastRow(t *testing.T) {
	m, svc := newStoryModel(t)
	selectID(t, m, "KB-1")
	press(t, m, "c", "down", "d", "y")
	if !strings.Contains(screen(m), "› [ ] opens the board") {
		t.Fatalf("cursor not on the new last row:\n%s", screen(m))
	}
	press(t, m, "d", "y")
	it, err := svc.Get(context.Background(), "KB-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(it.Acceptance) != 0 {
		t.Fatalf("criteria = %+v, want none", it.Acceptance)
	}
	press(t, m, "d", "down", "up")
	if m.accept == nil || !strings.Contains(screen(m), "no acceptance criteria") {
		t.Fatal("the empty popup must stay usable")
	}
}

func TestRemovingTheLastUncheckedCriterionTakesTheStoryToDone(t *testing.T) {
	m, svc := newStoryModel(t)
	selectID(t, m, "KB-1")
	press(t, m, "c", "space", "down", "d", "y")
	if got := status(t, svc, "KB-1"); got != core.StatusDone {
		t.Fatalf("story is %s, want done", got)
	}
}

func TestServiceBackendRemovesACriterion(t *testing.T) {
	m, svc := newStoryModel(t)
	selectID(t, m, "KB-1")
	press(t, m, "c", "d", "y")
	it, err := svc.Get(context.Background(), "KB-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(it.Acceptance) != 1 || it.Acceptance[0].Text != "shows writes within a second" {
		t.Fatalf("criteria = %+v, want only the second", it.Acceptance)
	}
	if !strings.Contains(screen(m), "[ ] shows writes within a second") || strings.Contains(screen(m), "[ ] opens the board") {
		t.Fatalf("the popup does not show the removal:\n%s", screen(m))
	}
}
