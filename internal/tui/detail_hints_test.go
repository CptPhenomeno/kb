package tui

import (
	"strings"
	"testing"
)

func TestCommandBarShowsTheDetailKeysWhileTheDetailIsOpen(t *testing.T) {
	m := newModel(t, sampleBoard(), 160, 48)
	selectID(t, m, "KB-4")
	press(t, m, "enter")
	bar := lastLines(screen(m), 2)
	for _, want := range []string{"↑ ↓ scroll", "pgup pgdn page", "e edit", "ctrl+e describe", "c criteria", "a assign", "l link", "esc/enter close"} {
		if !strings.Contains(bar, want) {
			t.Errorf("command bar lacks %q:\n%s", want, bar)
		}
	}
	if strings.Contains(bar, "shift") {
		t.Errorf("command bar still shows the board keys:\n%s", bar)
	}
}

func TestCommandBarShowsTheInnerPopupKeysOverTheDetail(t *testing.T) {
	m := newModel(t, sampleBoard(), 160, 48)
	selectID(t, m, "KB-4")
	press(t, m, "enter", "c")
	bar := lastLines(screen(m), 2)
	if !strings.Contains(bar, "space check") {
		t.Errorf("command bar lacks the criteria keys:\n%s", bar)
	}
	if strings.Contains(bar, "pgup pgdn page") {
		t.Errorf("command bar shows the detail keys under the popup:\n%s", bar)
	}
}

func TestHelpListsTheDetailPopupKeys(t *testing.T) {
	m := newModel(t, sampleBoard(), 160, 48)
	press(t, m, "?", "tab")
	help := screen(m)
	for _, want := range []string{"Detail popup", "scroll the detail by one line", "scroll the detail by one page", "close the detail popup"} {
		if !strings.Contains(help, want) {
			t.Errorf("help lacks %q:\n%s", want, help)
		}
	}
	if strings.Contains(strings.ToLower(help), "detail panel") {
		t.Errorf("help still speaks of a detail panel:\n%s", help)
	}
}

func TestEnterHintsOpenTheDetailPopup(t *testing.T) {
	for _, v := range []viewKind{viewBoard, viewTree} {
		found := false
		for _, h := range viewKeys[v] {
			if h.key == "enter" {
				found = true
				if h.help != "open the detail popup" {
					t.Errorf("%s enter hint: %q", v, h.help)
				}
			}
		}
		if !found {
			t.Errorf("%s has no enter hint", v)
		}
	}
	for _, h := range viewKeys[viewActivity] {
		if h.key == "o" && !strings.Contains(h.help, "detail popup") {
			t.Errorf("activity o hint: %q", h.help)
		}
	}
}

func TestBoardDetailGoldenShowsTheDetailKeys(t *testing.T) {
	m := newModel(t, sampleBoard(), 160, 48)
	press(t, m, "right", "right", "down", "down", "enter") // KB-4
	if !strings.Contains(lastLines(screen(m), 2), "pgup pgdn page") {
		t.Fatal("the command bar does not show the detail keys")
	}
	golden(t, "board_detail_160x48", screen(m))
}
