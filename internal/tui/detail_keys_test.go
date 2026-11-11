package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// detailPage is the number of lines a page scroll moves in a popup of the
// model's size.
func detailPage(m *Model) int {
	_, h := m.detailSize()
	return detailWindow(h - 2)
}

func TestDownScrollsTheDetailByOneLine(t *testing.T) {
	m := longDetail(t)
	press(t, m, "down")
	if m.detailScroll != 1 || m.selectedID() != "KB-4" {
		t.Fatalf("scroll %d, selected %s: want 1, KB-4", m.detailScroll, m.selectedID())
	}
	press(t, m, "down")
	if m.detailScroll != 2 {
		t.Fatalf("scroll %d after two downs, want 2", m.detailScroll)
	}
}

func TestUpScrollsTheDetailBackByOneLine(t *testing.T) {
	m := longDetail(t)
	press(t, m, "up")
	if m.detailScroll != 0 {
		t.Fatalf("up at the top scrolled to %d", m.detailScroll)
	}
	press(t, m, "down", "down", "up")
	if m.detailScroll != 1 || m.selectedID() != "KB-4" {
		t.Fatalf("scroll %d, selected %s: want 1, KB-4", m.detailScroll, m.selectedID())
	}
}

func TestPgdownScrollsTheDetailByOnePopupPage(t *testing.T) {
	m := longDetail(t)
	press(t, m, "pgdown")
	if want := detailPage(m); m.detailScroll != want {
		t.Fatalf("scroll %d after pgdown, want %d", m.detailScroll, want)
	}
	for range 10 {
		press(t, m, "pgdown")
	}
	max := m.detailScroll
	press(t, m, "pgdown")
	if m.detailScroll != max || !strings.Contains(screen(m), "refresh rotates the token") {
		t.Fatalf("pgdown went past the last page (%d then %d) or hides the end:\n%s", max, m.detailScroll, screen(m))
	}
}

func TestPgupScrollsTheDetailBackByOnePopupPage(t *testing.T) {
	m := longDetail(t)
	press(t, m, "pgdown", "pgdown", "pgup")
	if want := detailPage(m); m.detailScroll != want {
		t.Fatalf("scroll %d, want %d", m.detailScroll, want)
	}
	press(t, m, "pgup", "pgup")
	if m.detailScroll != 0 {
		t.Fatalf("pgup went above the top: %d", m.detailScroll)
	}
}

func TestMouseWheelScrollsTheDetail(t *testing.T) {
	m := longDetail(t)
	send(t, m, tea.MouseWheelMsg{X: 1, Y: 1, Button: tea.MouseWheelDown}) // outside the popup
	send(t, m, tea.MouseWheelMsg{X: 80, Y: 15, Button: tea.MouseWheelDown})
	if m.detailScroll != 2 || m.selectedID() != "KB-4" {
		t.Fatalf("scroll %d, selected %s after two wheel downs: want 2, KB-4", m.detailScroll, m.selectedID())
	}
	send(t, m, tea.MouseWheelMsg{X: 80, Y: 15, Button: tea.MouseWheelUp})
	if m.detailScroll != 1 {
		t.Fatalf("scroll %d after a wheel up, want 1", m.detailScroll)
	}
}

func TestMarkersShowTheLinesHiddenAboveAndBelow(t *testing.T) {
	m := longDetail(t)
	s := screen(m)
	if strings.Contains(s, "· pgup") || !strings.Contains(s, "· pgdown") {
		t.Fatalf("at the top only the marker below is expected:\n%s", s)
	}
	press(t, m, "down", "down", "down")
	s = screen(m)
	if !strings.Contains(s, "↑ 3 more lines") {
		t.Fatalf("marker above does not count 3 hidden lines:\n%s", s)
	}
	w, h := m.detailSize()
	below := len(m.detailLines(w-4)) - 3 - detailWindow(h-2)
	if !strings.Contains(s, fmt.Sprintf("↓ %d more lines", below)) {
		t.Fatalf("marker below does not count %d hidden lines:\n%s", below, s)
	}
	for range 10 {
		press(t, m, "pgdown")
	}
	if s = screen(m); strings.Contains(s, "· pgdown") || !strings.Contains(s, "· pgup") {
		t.Fatalf("at the end only the marker above is expected:\n%s", s)
	}
}

func TestOtherKeysLeaveTheSelectionUnchangedWhileTheDetailIsOpen(t *testing.T) {
	m := longDetail(t)
	view := m.view
	for _, k := range []string{"left", "right", "shift+left", "shift+right", "shift+up", "shift+down", "n", "d", "s", "f", "/", "t", "tab", "shift+tab", "?", "ctrl+p", "x", "1"} {
		press(t, m, k)
		if !m.detailOpen || m.selectedID() != "KB-4" || m.view != view {
			t.Fatalf("%q changed the state: open %v, selected %s, view %v", k, m.detailOpen, m.selectedID(), m.view)
		}
		if m.form != nil || m.confirm != nil || m.filterOpen || m.picker != nil || m.palette != nil || m.sort != nil || m.help || m.board.searching {
			t.Fatalf("%q opened something under the detail", k)
		}
	}
}

func TestQDoesNotQuitWhileTheDetailIsOpen(t *testing.T) {
	m := longDetail(t)
	if press(t, m, "q") || !m.detailOpen {
		t.Fatal("q quit or closed the detail")
	}
}

func TestCtrlCQuitsWithTheDetailOpen(t *testing.T) {
	m := longDetail(t)
	if !press(t, m, "ctrl+c") {
		t.Fatal("ctrl+c did not quit")
	}
}

func TestClickOutsideTheDetailClosesIt(t *testing.T) {
	m := longDetail(t)
	send(t, m, tea.MouseClickMsg{X: 2, Y: 2, Button: tea.MouseLeft})
	if m.detailOpen || m.selectedID() != "KB-4" {
		t.Fatalf("open %v, selected %s after a click outside: want closed, KB-4", m.detailOpen, m.selectedID())
	}
}

func TestClickInsideTheDetailKeepsItOpen(t *testing.T) {
	m := longDetail(t)
	for _, p := range [][2]int{{80, 15}, {20, 4}, {139, 25}} { // inside, top left and bottom right corners
		send(t, m, tea.MouseClickMsg{X: p[0], Y: p[1], Button: tea.MouseLeft})
		if !m.detailOpen || m.selectedID() != "KB-4" {
			t.Fatalf("click at %v: open %v, selected %s", p, m.detailOpen, m.selectedID())
		}
	}
}
