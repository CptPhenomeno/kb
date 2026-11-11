package tui

import (
	"context"
	"strings"
	"testing"
)

// The external editor cannot run inside a test: these tests check that
// ctrl+e targets the right item and what happens when the editor returns.

func TestCtrlEOnTheBoardSavesTheEditedDescription(t *testing.T) {
	m, svc := newLiveModel(t)
	selectID(t, m, "KB-2")
	_, cmd := m.Update(keyMsg("ctrl+e"))
	if cmd == nil || m.editing != "KB-2" {
		t.Fatalf("ctrl+e on the board did not start editing KB-2 (editing %q)", m.editing)
	}
	it, _ := svc.Get(context.Background(), "KB-2")
	send(t, m, editorDoneMsg{target: "KB-2", version: it.Version, text: "Written in the editor.\n\nSecond paragraph."})
	if got, _ := svc.Get(context.Background(), "KB-2"); got.Description != "Written in the editor.\n\nSecond paragraph." {
		t.Fatalf("description = %q", got.Description)
	}
	if !strings.Contains(lastLines(screen(m), 3), "KB-2 description saved") {
		t.Fatalf("no confirmation in the status line:\n%s", lastLines(screen(m), 3))
	}
}

func TestFormConfirmsTheDescriptionCameBackFromTheEditor(t *testing.T) {
	m, _ := newLiveModel(t)
	press(t, m, "n")
	_, cmd := m.Update(keyMsg("ctrl+e"))
	if cmd == nil {
		t.Fatal("ctrl+e in the form did not start the editor")
	}
	send(t, m, editorDoneMsg{text: "first line\nsecond line\nthird line"})
	s := screen(m)
	if !strings.Contains(s, "description updated (3 lines)") || !strings.Contains(s, "ctrl+s saves") {
		t.Fatalf("the form does not confirm the edited description:\n%s", s)
	}
}
