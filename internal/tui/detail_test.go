package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/CptPhenomeno/kb/internal/core"
)

// popupCorners reports whether the screen shows a popup frame with its top
// left corner at (x, y) and the given outer size.
func popupCorners(s string, x, y, w, h int) bool {
	lines := strings.Split(s, "\n")
	at := func(row, col int) rune {
		if row >= len(lines) {
			return 0
		}
		r := []rune(lines[row])
		if col >= len(r) {
			return 0
		}
		return r[col]
	}
	return at(y, x) == '╭' && at(y, x+w-1) == '╮' && at(y+h-1, x) == '╰' && at(y+h-1, x+w-1) == '╯'
}

func TestEnterOpensDetailPopupCenteredAt75Percent(t *testing.T) {
	m := newModel(t, sampleBoard(), 160, 48)
	press(t, m, "right", "right", "down", "down", "enter") // KB-4
	if !m.detailOpen {
		t.Fatal("enter did not open the detail")
	}
	if !popupCorners(screen(m), 20, 6, 120, 36) {
		t.Fatalf("no 120x36 popup centered at (20,6):\n%s", screen(m))
	}
}

func TestDetailPopupIsAtLeast60By16OnASmallTerminal(t *testing.T) {
	m := newModel(t, sampleBoard(), 70, 20)
	press(t, m, "enter")
	if !popupCorners(screen(m), 5, 2, 60, 16) {
		t.Fatalf("no 60x16 popup centered at (5,2):\n%s", screen(m))
	}
}

func TestDetailPopupNeverExceedsTheScreenMinusOneCellPerSide(t *testing.T) {
	m := newModel(t, sampleBoard(), 100, 15)
	press(t, m, "enter")
	if !popupCorners(screen(m), 12, 1, 75, 13) {
		t.Fatalf("no 75x13 popup at (12,1):\n%s", screen(m))
	}
}

func TestBoardColumnsUseTheFullWidthWithTheDetailOpen(t *testing.T) {
	m := newModel(t, sampleBoard(), 160, 48)
	closed := m.board.geometry(m)
	press(t, m, "enter")
	g := m.board.geometry(m)
	if g != closed || g.colX[4]+g.colW[4] != 160 {
		t.Fatalf("columns with the detail open %v, closed %v: want the same, ending at 160", g, closed)
	}
}

func TestTreeRowsUseTheFullWidthWithTheDetailOpen(t *testing.T) {
	m := treeModel(t)
	press(t, m, "enter")
	if _, ok := m.tree.rowAt(m, 150, 3); !ok {
		t.Fatal("the tree list does not reach the right edge with the detail open")
	}
	first := strings.Split(ansi.Strip(m.tree.render(m, m.bodyHeight())), "\n")[2]
	if displayWidth(strings.TrimRight(first, " ")) < 130 {
		t.Fatalf("tree row is not drawn at full width: %q", first)
	}
}

func TestEnterOnATreeRowOpensTheDetailPopup(t *testing.T) {
	m := treeModel(t)
	press(t, m, "enter")
	if !m.detailOpen || !popupCorners(screen(m), 20, 6, 120, 36) {
		t.Fatalf("enter on a tree row did not open the centered popup:\n%s", screen(m))
	}
}

func TestEscClosesTheDetailPopup(t *testing.T) {
	m := newModel(t, sampleBoard(), 160, 48)
	press(t, m, "enter", "esc")
	if m.detailOpen || popupCorners(screen(m), 20, 6, 120, 36) {
		t.Fatal("esc did not close the detail popup")
	}
}

func TestEnterClosesTheDetailPopup(t *testing.T) {
	m := newModel(t, sampleBoard(), 160, 48)
	press(t, m, "enter", "enter")
	if m.detailOpen || popupCorners(screen(m), 20, 6, 120, 36) {
		t.Fatal("enter did not close the detail popup")
	}
}

func TestDetailTextWrapsInThePopupWidth(t *testing.T) {
	be := sampleBoard()
	line := strings.Repeat("alpha beta ", 9) + "gamma" // 104 cells, fits the popup but not the old panel
	be.snap.Get("KB-4").Description = line + " " + strings.Repeat("delta ", 25) + "ZEBRA"
	m := newModel(t, be, 160, 48)
	press(t, m, "right", "right", "down", "down", "enter") // KB-4
	s := screen(m)
	if !strings.Contains(s, line) {
		t.Fatalf("a 104-cell line is wrapped before the popup width:\n%s", s)
	}
	if !strings.Contains(s, "ZEBRA") {
		t.Fatalf("the end of the wrapped description is missing:\n%s", s)
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
	m := newModel(t, sampleBoard(), 100, 48) // narrow columns
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
