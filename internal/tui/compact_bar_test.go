package tui

import (
	"slices"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func barText(m *Model) string { return ansi.Strip(strings.Join(m.commandBar(), "\n")) }

// chips returns the "key label" pairs of a bar, whitespace-normalized.
func barHas(bar string, pairs ...string) []string {
	flat := strings.Join(strings.Fields(bar), " ")
	var missing []string
	for _, p := range pairs {
		if !strings.Contains(flat, p) {
			missing = append(missing, p)
		}
	}
	return missing
}

func barAbsent(bar string, labels ...string) []string {
	flat := strings.Join(strings.Fields(bar), " ")
	var found []string
	for _, l := range labels {
		if strings.Contains(flat, l) {
			found = append(found, l)
		}
	}
	return found
}

func TestBoardCompactBarShowsTheEssentialKeys(t *testing.T) {
	m := newModel(t, sampleBoard(), 160, 48)
	bar := barText(m)
	if miss := barHas(bar, "← → ↑ ↓ navigate", "enter details", "n new", "/ search", "tab next view", "ctrl+p commands", "? help", "q quit"); len(miss) > 0 {
		t.Fatalf("board compact bar lacks %v:\n%s", miss, bar)
	}
	if f := barAbsent(bar, "sort", "reorder", "filter", "delete", "theme", "assign", "link"); len(f) > 0 {
		t.Fatalf("board compact bar shows %v:\n%s", f, bar)
	}
}

func TestTreeCompactBarShowsTheEssentialKeysWithX(t *testing.T) {
	m := newModel(t, sampleBoard(), 160, 48)
	press(t, m, "tab")
	bar := barText(m)
	if miss := barHas(bar, "← → ↑ ↓ navigate", "x expand/collapse all", "enter details", "n new", "/ search", "tab next view", "ctrl+p commands", "? help", "q quit"); len(miss) > 0 {
		t.Fatalf("tree compact bar lacks %v:\n%s", miss, bar)
	}
	if f := barAbsent(bar, "filter", "delete", "theme", "criteria"); len(f) > 0 {
		t.Fatalf("tree compact bar shows %v:\n%s", f, bar)
	}
}

func TestActivityCompactBarShowsTheEssentialKeys(t *testing.T) {
	m := newModel(t, sampleBoard(), 160, 48)
	press(t, m, "tab", "tab")
	bar := barText(m)
	if miss := barHas(bar, "↑ ↓ select actor", "enter only this actor", "o open item", "tab next view", "ctrl+p commands", "? help", "q quit"); len(miss) > 0 {
		t.Fatalf("activity compact bar lacks %v:\n%s", miss, bar)
	}
	if f := barAbsent(bar, "release claim", "scroll events", "all actors", "theme"); len(f) > 0 {
		t.Fatalf("activity compact bar shows %v:\n%s", f, bar)
	}
}

func TestBoardCompactBarTakesAtMostTwoRowsAt80By24(t *testing.T) {
	m := newModel(t, sampleBoard(), 80, 24)
	if n := len(m.commandBar()); n > 2 {
		t.Fatalf("board compact bar takes %d rows:\n%s", n, barText(m))
	}
}

func TestFullModeShowsTheCompleteBar(t *testing.T) {
	m := newModel(t, sampleBoard(), 160, 48)
	m.barMode = barFull
	bar := barText(m)
	if miss := barHas(bar, "shift ↑ ↓ reorder", "s sort", "f filter", "d delete", "t theme", "p epics & stories"); len(miss) > 0 {
		t.Fatalf("full bar lacks %v:\n%s", miss, bar)
	}
	m.view = viewBoard
	m2 := newModel(t, sampleBoard(), 80, 24)
	m2.barMode = barFull
	if n := len(m2.commandBar()); n < 3 {
		t.Fatalf("full bar at 80x24 takes %d rows, want today's wrapped bar", n)
	}
}

func TestPopupsKeepTheirKeysInCompactMode(t *testing.T) {
	open := map[string][]string{
		"palette": {"ctrl+p"},
		"filter":  {"f"},
		"search":  {"/"},
		"sort":    {"s"},
		"form":    {"n"},
		"theme":   {"t"},
	}
	for name, keys := range open {
		c := newModel(t, sampleBoard(), 160, 48)
		f := newModel(t, sampleBoard(), 160, 48)
		f.barMode = barFull
		press(t, c, keys...)
		press(t, f, keys...)
		if got, want := c.commandBar(), f.commandBar(); !slices.Equal(got, want) {
			t.Errorf("%s: compact bar differs from full:\n%s\nvs\n%s", name, strings.Join(got, "\n"), strings.Join(want, "\n"))
		}
	}
}

func TestBarKeepsTheViewBarHeightWithTheHelpOrTheDetailOpen(t *testing.T) {
	for _, mode := range []barMode{barCompact, barFull} {
		for _, key := range []string{"?", "enter"} {
			m := newModel(t, sampleBoard(), 80, 24)
			m.barMode = mode
			want := len(m.commandBar())
			press(t, m, key)
			if got := len(m.commandBar()); got != want {
				t.Errorf("mode %d key %q: bar height %d, want the view bar's %d", mode, key, got, want)
			}
		}
	}
}

func TestHelpListsEveryKeyInCompactMode(t *testing.T) {
	m := newModel(t, sampleBoard(), 160, 48)
	press(t, m, "?")
	s := screen(m)
	for _, h := range append(append([]keyHint{}, viewKeys[viewBoard]...), globalKeys...) {
		if !strings.Contains(s, h.key) || !strings.Contains(s, strings.Fields(h.help)[0]) {
			t.Errorf("help lacks %q (%s)", h.key, h.help)
		}
	}
}

func TestBoardGoldenWithTheCompactBarAt80By24(t *testing.T) {
	m := newModel(t, sampleBoard(), 80, 24)
	golden(t, "board_compact_80x24", screen(m))
}
