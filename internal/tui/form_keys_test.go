package tui

import (
	"context"
	"strings"
	"testing"
)

// The editor cannot run in a test: a command returned by the key means the
// editor was started.

func TestFormEnterOnDescriptionOpensEditor(t *testing.T) {
	m, _ := newLiveModel(t)
	press(t, m, "n")
	typeText(t, m, "Delta task")
	m.form.focus = fDescription
	_, cmd := m.Update(keyMsg("enter"))
	if cmd == nil {
		t.Fatal("enter on the description did not start the editor")
	}
	if m.form == nil || m.form.saving {
		t.Fatal("enter on the description saved the form")
	}
}

func TestFormCtrlEOpensEditorFromAnyField(t *testing.T) {
	m, _ := newLiveModel(t)
	press(t, m, "n")
	for i := range m.form.fields {
		m.form.focus = i
		if _, cmd := m.Update(keyMsg("ctrl+e")); cmd == nil {
			t.Fatalf("ctrl+e on field %q did not start the editor", m.form.fields[i].label)
		}
	}
}

func TestFormCtrlSSavesFromAnyField(t *testing.T) {
	for _, focus := range []int{fTitle, fType, fPriority, fDescription} {
		m, svc := newLiveModel(t)
		press(t, m, "n")
		typeText(t, m, "Delta task")
		m.form.focus = focus
		press(t, m, "ctrl+s")
		if _, err := svc.Get(context.Background(), "KB-4"); err != nil {
			t.Fatalf("ctrl+s on field %d did not create the item: %v", focus, err)
		}
	}
}

func TestFormEnterOnOtherFieldsSavesTheForm(t *testing.T) {
	for _, focus := range []int{fTitle, fType, fParent, fPriority, fPoints, fLabels, fSpec} {
		m, svc := newLiveModel(t)
		press(t, m, "n")
		typeText(t, m, "Delta task")
		m.form.focus = focus
		press(t, m, "enter")
		if _, err := svc.Get(context.Background(), "KB-4"); err != nil {
			t.Fatalf("enter on field %d did not create the item: %v", focus, err)
		}
	}
}

func TestFormStatusLineShowsTheContextualKeys(t *testing.T) {
	m := openForm(t)
	m.form.focus = fDescription
	s := screen(m)
	if !strings.Contains(s, "enter $EDITOR · ctrl+s saves · tab or ↑ ↓ move") {
		t.Fatalf("description status line missing:\n%s", s)
	}
	m.form.focus = fTitle
	s = screen(m)
	if !strings.Contains(s, "enter or ctrl+s saves · tab or ↑ ↓ move · ← → change a choice") {
		t.Fatalf("status line missing:\n%s", s)
	}
}

// While the form is open it takes every key, including ?, so the keys show
// in the command bar; the help screen lists them for when it is closed.
func TestHelpListsCtrlSAndTheContextualEnter(t *testing.T) {
	m := openForm(t)
	bar := lastLines(screen(m), 3)
	if !strings.Contains(bar, "ctrl+s") || !strings.Contains(bar, "enter") || !strings.Contains(bar, "$EDITOR") {
		t.Fatalf("command bar does not list the form keys:\n%s", bar)
	}
	send(t, m, windowSize(160, 64)) // tall enough for every section of the help
	press(t, m, "esc", "?")
	s := screen(m)
	if !strings.Contains(s, "ctrl+s") || !strings.Contains(s, "or open $EDITOR on the description") {
		t.Fatalf("help does not list ctrl+s and the contextual enter:\n%s", s)
	}
}
