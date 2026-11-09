package tui

import (
	"context"
	"strings"
	"testing"

	"github.com/CptPhenomeno/kb/internal/core"
)

func typeText(t *testing.T, m *Model, s string) {
	t.Helper()
	for _, r := range s {
		if r == ' ' {
			press(t, m, "space")
			continue
		}
		press(t, m, string(r))
	}
}

func boardSize(t *testing.T, svc *core.Service) int {
	t.Helper()
	snap, err := svc.Board(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return len(snap.Items())
}

func TestNewFormCreatesItem(t *testing.T) {
	m, svc := newLiveModel(t)
	press(t, m, "n")
	if !strings.Contains(screen(m), "New item") {
		t.Fatalf("n did not open the form:\n%s", screen(m))
	}
	typeText(t, m, "Delta task")
	press(t, m, "down", "right")         // type: task → subtask? no: the next type after task is bug
	press(t, m, "down", "down", "right") // priority medium → high
	press(t, m, "enter")
	it, err := svc.Get(context.Background(), "KB-4")
	if err != nil {
		t.Fatalf("no item created: %v\n%s", err, screen(m))
	}
	if it.Title != "Delta task" || it.Type != core.TypeBug || it.Priority != core.PriorityHigh || it.Status != core.StatusBacklog {
		t.Fatalf("created %+v", it)
	}
	if m.form != nil || m.selectedID() != "KB-4" {
		t.Fatalf("after creation: form %v, selected %s", m.form, m.selectedID())
	}
}

func TestEditFormPrefilled(t *testing.T) {
	m, svc := newLiveModel(t)
	selectID(t, m, "KB-2")
	press(t, m, "e")
	s := screen(m)
	if !strings.Contains(s, "Edit KB-2") || !strings.Contains(s, "Beta") {
		t.Fatalf("edit form not prefilled:\n%s", s)
	}
	typeText(t, m, " v2")
	press(t, m, "enter")
	if it, _ := svc.Get(context.Background(), "KB-2"); it.Title != "Beta v2" {
		t.Fatalf("title = %q, want 'Beta v2'", it.Title)
	}
}

func TestFormValidationError(t *testing.T) {
	m, svc := newLiveModel(t)
	before := boardSize(t, svc)
	press(t, m, "n", "enter") // empty title
	if m.form == nil || !strings.Contains(screen(m), "title is required") {
		t.Fatalf("empty title not reported next to the field:\n%s", screen(m))
	}
	typeText(t, m, "Orphan")
	press(t, m, "down", "down") // parent
	typeText(t, m, "KB-99")
	press(t, m, "enter")
	if m.form == nil || !strings.Contains(screen(m), "KB-99") || !strings.Contains(screen(m), "not found") {
		t.Fatalf("unknown parent not reported:\n%s", screen(m))
	}
	if boardSize(t, svc) != before {
		t.Fatal("an invalid form created an item")
	}
}

func TestEscCancelsForm(t *testing.T) {
	m, svc := newLiveModel(t)
	before := boardSize(t, svc)
	press(t, m, "n")
	typeText(t, m, "Never")
	press(t, m, "esc")
	if m.form != nil || boardSize(t, svc) != before {
		t.Fatal("esc must close the form without writing")
	}
}
