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

func TestDetailListsChildrenWithTheirStatus(t *testing.T) {
	m := newModel(t, sampleBoard(), 160, 60)
	press(t, m, "right", "right", "down", "enter") // KB-2, story with three children
	if m.selectedID() != "KB-2" {
		t.Fatalf("selected %s, want KB-2", m.selectedID())
	}
	s := screen(m)
	if !strings.Contains(s, "children 0/3 done") {
		t.Fatalf("detail lacks the children header:\n%s", s)
	}
	for _, want := range []string{"KB-3 Validate token in middleware", "KB-4 Token refresh", "KB-5 Expired token"} {
		if !strings.Contains(s, want) {
			t.Errorf("children list lacks %q", want)
		}
	}
	for _, label := range []string{"Review", "In progress", "To do"} {
		if !strings.Contains(s, label) {
			t.Errorf("children list does not show the status %q", label)
		}
	}
}

func TestItemWithoutChildrenHasNoChildrenSection(t *testing.T) {
	m := newModel(t, sampleBoard(), 160, 60)
	press(t, m, "enter") // KB-15, a task without subtasks
	if strings.Contains(screen(m), "children ") {
		t.Fatal("a leaf item shows a children section")
	}
}

func TestDetailListsEveryLinkWithTheLinkedItemsStatus(t *testing.T) {
	be := sampleBoard()
	be.snap.Relations = append(be.snap.Relations, core.Relation{Kind: core.RelBlocks, From: "KB-17", To: "KB-15"})
	m := newModel(t, be, 160, 60)
	press(t, m, "enter") // KB-15, blocked by KB-17 which is done
	var row string
	for _, line := range strings.Split(screen(m), "\n") {
		if strings.Contains(line, "blocked by") && strings.Contains(line, "KB-17") {
			row = line
		}
	}
	if !strings.Contains(row, "Health check endpoint") || !strings.Contains(row, "Done") {
		t.Fatalf("link to a finished blocker missing or incomplete: %q\n%s", row, screen(m))
	}
}

// longDetail opens the detail of KB-4 with a description longer than the
// panel, on a short terminal.
func longDetail(t *testing.T) *Model {
	t.Helper()
	be := sampleBoard()
	var b strings.Builder
	for i := 1; i <= 30; i++ {
		b.WriteString("Paragraph line number " + string(rune('A'+i%26)) + ".\n\n")
	}
	be.snap.Get("KB-4").Description = b.String()
	m := newModel(t, be, 160, 30)
	press(t, m, "right", "right", "down", "down", "enter")
	return m
}

func TestPgdownScrollsTheDetailPanel(t *testing.T) {
	m := longDetail(t)
	if strings.Contains(screen(m), "history") {
		t.Fatal("fixture too short: the history is already visible")
	}
	for range 10 {
		press(t, m, "pgdown")
	}
	s := screen(m)
	if !strings.Contains(s, "history") || !strings.Contains(s, "refresh rotates the token") {
		t.Fatalf("pgdown did not reach the end of the detail:\n%s", s)
	}
	press(t, m, "pgup", "pgup", "pgup", "pgup", "pgup", "pgup", "pgup", "pgup", "pgup", "pgup")
	if !strings.Contains(screen(m), "Token refresh") {
		t.Fatal("pgup did not come back to the top")
	}
	press(t, m, "pgdown", "down", "up") // another item: back to the top
	if !strings.Contains(screen(m), "KB-4 · task") {
		t.Fatal("changing the selection must scroll the detail back to the top")
	}
}

func TestAMarkerSaysMoreLinesAreBelow(t *testing.T) {
	m := longDetail(t)
	if s := screen(m); !strings.Contains(s, "↓") || !strings.Contains(s, "more lines") {
		t.Fatalf("no marker for the cut content:\n%s", s)
	}
	press(t, m, "pgdown")
	if s := screen(m); !strings.Contains(s, "↑") {
		t.Fatal("no marker for the content above after scrolling")
	}
}
