package tui

import (
	"strings"
	"testing"

	"github.com/CptPhenomeno/kb/internal/core"
)

func TestDetailPanelGrowsWithTerminal(t *testing.T) {
	widths := map[int]int{}
	for _, w := range []int{100, 160, 240} {
		m := newModel(t, sampleBoard(), w, 48)
		press(t, m, "enter")
		widths[w] = m.board.geometry(m).detailW
	}
	if !(widths[100] < widths[160] && widths[160] < widths[240]) {
		t.Fatalf("detail widths %v must grow with the terminal", widths)
	}
	if widths[240] <= 56 {
		t.Fatalf("detail width at 240 columns = %d, want more than the old 56 cap", widths[240])
	}
}

const longNote = "moved token validation into the middleware so that every handler behind it receives an already verified principal and the final word is ZEBRA"

func TestHistoryLinesWrapInsteadOfTruncating(t *testing.T) {
	be := sampleBoard()
	be.events = append(be.events, core.Event{ID: 9, At: testNow, Actor: "agent-auth", Item: "KB-4",
		Kind: core.EventNote, Data: map[string]any{"text": longNote}})
	m := newModel(t, be, 140, 60)
	press(t, m, "right", "right", "down", "down", "enter") // KB-4
	s := screen(m)
	if !strings.Contains(s, "ZEBRA") {
		t.Fatalf("the end of a long history entry is cut:\n%s", s)
	}
	if strings.Contains(s, "…") && strings.Contains(s, "receives an…") {
		t.Fatal("history entry truncated with an ellipsis")
	}
}

func TestTestsAndCriteriaWrap(t *testing.T) {
	be := sampleBoard()
	it := be.snap.Get("KB-4")
	it.Tests = append(it.Tests, core.Test{Name: "refresh keeps working when the clock of the client is skewed by more than five minutes QUAIL"})
	it.Acceptance = append(it.Acceptance, core.Criterion{Text: "a reused refresh token revokes the whole token family and logs a security event OTTER"})
	m := newModel(t, be, 140, 60)
	press(t, m, "right", "right", "down", "down", "enter") // KB-4
	s := screen(m)
	for _, w := range []string{"QUAIL", "OTTER"} {
		if !strings.Contains(s, w) {
			t.Errorf("long test or criterion cut before %q", w)
		}
	}
}

func TestCardIDNeverTruncated(t *testing.T) {
	m := newModel(t, sampleBoard(), 160, 48)
	press(t, m, "enter") // the detail panel makes the columns narrow
	s := screen(m)
	for _, id := range []string{"KB-15", "KB-17", "KB-12"} {
		if !strings.Contains(s, id+" ") {
			t.Errorf("card ID %s is cut in narrow columns", id)
		}
	}
}
