package tui

import (
	"context"
	"strings"
	"testing"

	"github.com/CptPhenomeno/kb/internal/core"
)

func TestSearchIncremental(t *testing.T) {
	m := newModel(t, sampleBoard(), 160, 48)
	press(t, m, "/", "t", "o", "k")
	s := screen(m)
	if strings.Contains(s, "Key storage schema") || !strings.Contains(s, "Token refresh") || !strings.Contains(s, "Expired token") {
		t.Fatalf("search 'tok' does not narrow the board:\n%s", s)
	}
	press(t, m, "backspace", "backspace", "backspace", "k", "b", "-", "1", "7")
	if s := screen(m); strings.Contains(s, "Token refresh") || !strings.Contains(s, "Health check endpoint") {
		t.Fatalf("search by ID kb-17 failed:\n%s", s)
	}
	press(t, m, "enter") // keep the search, back to navigation
	press(t, m, "q")     // q navigates nothing but must not be typed into the search
	if m.board.query != "kb-17" {
		t.Fatalf("query = %q after enter", m.board.query)
	}
	m2 := newModel(t, sampleBoard(), 160, 48)
	press(t, m2, "/", "x", "y", "z", "esc")
	if m2.board.query != "" || !strings.Contains(screen(m2), "Key storage schema") {
		t.Fatal("esc must clear the search")
	}
}

func TestFilterShowAllTypes(t *testing.T) {
	m := newModel(t, sampleBoard(), 160, 48)
	s := screen(m)
	for _, want := range []string{"Authentication", "API keys per tenant", "0/1 done"} {
		if !strings.Contains(s, want) {
			t.Errorf("default board lacks %q (epics and stories are shown by default)", want)
		}
	}
	if strings.Contains(s, "Bounded route label") {
		t.Error("subtasks must be hidden by default")
	}
	press(t, m, "f")
	if !strings.Contains(screen(m), "Filter") {
		t.Fatal("f did not open the filter popup")
	}
	press(t, m, "down", "down", "down", "down", "space", "esc") // subtask on
	if !strings.Contains(screen(m), "Bounded route label") {
		t.Fatal("turning subtasks on does not show them")
	}
	press(t, m, "f", "up", "up", "up", "up", "space", "esc") // epic off
	if strings.Contains(screen(m), "Authentication") {
		t.Fatal("turning epics off does not hide them")
	}
}

func TestFilterBlockedStale(t *testing.T) {
	m := newModel(t, sampleBoard(), 160, 48)
	press(t, m, "f", "down", "down", "down", "down", "down", "space", "esc") // only blocked
	s := screen(m)
	if !strings.Contains(s, "Key rotation endpoint") || strings.Contains(s, "Key storage schema") {
		t.Fatalf("only blocked shows the wrong cards:\n%s", s)
	}
	press(t, m, "f", "space", "down", "space", "esc") // the cursor is kept: blocked off, stale on
	s = screen(m)
	// KB-12 is stale; KB-3 has an expired claim too but is in review.
	if !strings.Contains(s, "Status code counters") || strings.Contains(s, "Validate token") || strings.Contains(s, "Token refresh") {
		t.Fatalf("only stale shows the wrong cards:\n%s", s)
	}
}

func TestFiltersShownInHeader(t *testing.T) {
	m := newModel(t, sampleBoard(), 160, 48)
	if header := strings.Split(screen(m), "\n")[0]; !strings.Contains(header, "epic · story · task · bug") {
		t.Fatalf("default header: %s", header)
	}
	press(t, m, "f", "down", "down", "down", "down", "space", "down", "space", "esc", "/", "k", "e", "y", "enter")
	header := strings.Split(screen(m), "\n")[0]
	for _, want := range []string{"all types", "blocked", `"key"`} {
		if !strings.Contains(header, want) {
			t.Errorf("header lacks %q: %s", want, header)
		}
	}
}

func TestPHidesAndShowsEpicsAndStories(t *testing.T) {
	m := newModel(t, sampleBoard(), 160, 48)
	press(t, m, "p")
	s := screen(m)
	if strings.Contains(s, "Authentication") || strings.Contains(s, "API keys per tenant") {
		t.Fatal("p did not hide epics and stories")
	}
	if header := strings.Split(s, "\n")[0]; !strings.Contains(header, "showing task · bug") {
		t.Fatalf("header after p: %s", header)
	}
	press(t, m, "p")
	if !strings.Contains(screen(m), "Authentication") {
		t.Fatal("p again did not show epics and stories")
	}
}

func TestTheParentsChoiceIsSavedForTheUser(t *testing.T) {
	var saved []bool
	m := newModel(t, sampleBoard(), 160, 48)
	m.opts.SaveShowParents = func(show bool) error { saved = append(saved, show); return nil }
	press(t, m, "p", "p")
	if len(saved) != 2 || saved[0] || !saved[1] {
		t.Fatalf("saved = %v, want [false true]", saved)
	}
	hidden := false
	m2 := New(context.Background(), sampleBoard(), Options{ShowParents: &hidden})
	if m2.board.types[core.TypeEpic] || m2.board.types[core.TypeStory] {
		t.Fatal("a saved 'hide' choice is not applied at start-up")
	}
}
