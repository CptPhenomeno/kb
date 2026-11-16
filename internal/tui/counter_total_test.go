package tui

import (
	"strings"
	"testing"
)

// headerTotal returns the header line of the screen, where the total sits.
func headerTotal(m *Model) string { return strings.Split(screen(m), "\n")[0] }

func TestBoardHeaderCountsVisibleCards(t *testing.T) {
	m := counterModel(t, Options{})
	// epic, story, three tasks and the bug; the subtask is hidden
	if h := headerTotal(m); !strings.Contains(h, "6 cards") || strings.Contains(h, "items") {
		t.Fatalf("header = %q, want 6 cards", h)
	}
}

func TestBoardHeaderSumsVisibleTaskAndBugPointsInPointsMode(t *testing.T) {
	m := counterModel(t, Options{BoardCounter: "points"})
	if h := headerTotal(m); !strings.Contains(h, "16pt") || strings.Contains(h, "items") {
		t.Fatalf("header = %q, want 16pt", h)
	}
}

func TestBoardHeaderFollowsTheSearch(t *testing.T) {
	m := counterModel(t, Options{})
	press(t, m, "/", "E", "i", "g", "h", "t", "enter")
	if h := headerTotal(m); !strings.Contains(h, "1 card") {
		t.Fatalf("header = %q, want 1 card", h)
	}
	press(t, m, "#")
	if h := headerTotal(m); !strings.Contains(h, "8pt") {
		t.Fatalf("header = %q, want 8pt", h)
	}
}

func TestTreeHeaderKeepsTheItemsCount(t *testing.T) {
	m := counterModel(t, Options{BoardCounter: "points"})
	press(t, m, "tab")
	if h := headerTotal(m); !strings.Contains(h, "7 items") {
		t.Fatalf("tree header = %q, want 7 items", h)
	}
	press(t, m, "tab")
	if h := headerTotal(m); !strings.Contains(h, "7 items") {
		t.Fatalf("activity header = %q, want 7 items", h)
	}
}

func TestPaletteTogglesTheCounters(t *testing.T) {
	var saved []string
	m := counterModel(t, Options{SaveBoardCounter: func(s string) error { saved = append(saved, s); return nil }})
	press(t, m, "ctrl+p")
	if !contains(names(m.palette.matches(m)), "Toggle counters: cards / points") {
		t.Fatalf("palette lacks the toggle: %v", names(m.palette.matches(m)))
	}
	press(t, m, "t", "o", "g", "g", "l", "e", " ", "c", "o", "u", "n", "enter")
	if h := headerTotal(m); !strings.Contains(h, "16pt") {
		t.Fatalf("header = %q, want 16pt", h)
	}
	if strings.Join(saved, ",") != "points" {
		t.Fatalf("saved = %v", saved)
	}
}

func TestHelpAndFullBarListTheHashKey(t *testing.T) {
	m := counterModel(t, Options{})
	m.barMode = barFull
	if miss := barHas(barText(m), "# counters"); len(miss) > 0 {
		t.Fatalf("full bar lacks %v:\n%s", miss, barText(m))
	}
	press(t, m, "?")
	if s := screen(m); !strings.Contains(s, "counters") || !strings.Contains(s, "#") {
		t.Fatalf("help lacks #:\n%s", s)
	}
}

func TestBoardHeaderSaysOneCardInTheSingular(t *testing.T) {
	m := counterModel(t, Options{})
	press(t, m, "/", "E", "i", "g", "h", "t", "enter")
	if h := headerTotal(m); !strings.Contains(h, "1 card") || strings.Contains(h, "1 cards") {
		t.Fatalf("header = %q, want 1 card", h)
	}
}
