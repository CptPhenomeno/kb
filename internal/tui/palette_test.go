package tui

import (
	"strings"
	"testing"

	"github.com/CptPhenomeno/kb/internal/core"
)

func TestPaletteOpensWithCtrlShiftP(t *testing.T) {
	m := newModel(t, sampleBoard(), 160, 48)
	press(t, m, "ctrl+shift+p")
	if m.palette == nil || !strings.Contains(screen(m), "Commands") {
		t.Fatal("ctrl+shift+p did not open the palette")
	}
}

func TestPaletteOpensWithCtrlP(t *testing.T) {
	m := newModel(t, sampleBoard(), 160, 48)
	press(t, m, "ctrl+p")
	if m.palette == nil || !strings.Contains(screen(m), "Commands") {
		t.Fatal("ctrl+p did not open the palette")
	}
	press(t, m, "esc")
	if m.palette != nil {
		t.Fatal("esc did not close the palette")
	}
}

func TestPaletteFuzzyMatch(t *testing.T) {
	m := newModel(t, sampleBoard(), 160, 48)
	press(t, m, "ctrl+p", "m", "v", "n", "x")
	got := m.palette.matches(m)
	if len(got) == 0 || got[0].name != "Move to next column" {
		t.Fatalf("'mvnx' matches %v, want 'Move to next column' first", names(got))
	}
	press(t, m, "backspace", "backspace", "backspace", "backspace", "t", "h", "e", "m")
	if got := m.palette.matches(m); len(got) != 1 || got[0].name != "Choose theme" {
		t.Fatalf("'them' matches %v", names(got))
	}
}

func TestPaletteRunsAction(t *testing.T) {
	m, svc := newLiveModel(t)
	press(t, m, "ctrl+p", "n", "e", "x", "t", " ", "c", "o", "l", "enter")
	if m.palette != nil {
		t.Fatal("the palette must close after running an action")
	}
	if got := status(t, svc, "KB-1"); got != core.StatusDoing {
		t.Fatalf("'Move to next column' from the palette left KB-1 in %s", got)
	}
}

func TestPaletteListsContextActions(t *testing.T) {
	m := newModel(t, sampleBoard(), 160, 48)
	press(t, m, "ctrl+p")
	board := names(m.palette.matches(m))
	for _, want := range []string{"Move to next column", "Reorder up", "Open details", "Search", "Filter", "Choose theme", "Next view", "Quit"} {
		if !contains(board, want) {
			t.Errorf("board palette lacks %q: %v", want, board)
		}
	}
	for _, want := range []string{"Create item", "Edit item", "Delete item", "Manage links", "Manage acceptance criteria"} {
		if !contains(board, want) {
			t.Errorf("board palette lacks %q", want)
		}
	}
	press(t, m, "esc", "tab", "ctrl+p")
	tree := names(m.palette.matches(m))
	if contains(tree, "Move to next column") {
		t.Errorf("the Tree view palette lists board actions: %v", tree)
	}
	for _, want := range []string{"Create child item", "Collapse branch", "Edit item", "Open details"} {
		if !contains(tree, want) {
			t.Errorf("tree palette lacks %q: %v", want, tree)
		}
	}
	if !strings.Contains(screen(m), "shift ← →") && !strings.Contains(lastLines(screen(m), 2), "enter") {
		t.Error("palette command bar missing")
	}
}

func names(as []action) []string {
	var out []string
	for _, a := range as {
		out = append(out, a.name)
	}
	return out
}

func contains(xs []string, x string) bool {
	for _, y := range xs {
		if y == x {
			return true
		}
	}
	return false
}
