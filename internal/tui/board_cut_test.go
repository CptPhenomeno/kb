package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/CptPhenomeno/kb/internal/core"
)

// longColumn is a board whose To do column holds n one-line tasks.
func longColumn(n int) *fakeBackend {
	var items []*core.Item
	for i := range n {
		items = append(items, item(fmt.Sprintf("KB-%d", i+1), core.TypeTask, core.StatusTodo, "", fmt.Sprintf("Task number %d", i+1), pos(i)))
	}
	return &fakeBackend{snap: core.NewSnapshot(items, nil), version: 1}
}

// todoColumn renders the To do column as plain lines, borders included.
func todoColumn(m *Model) []string {
	g := m.board.geometry(m)
	items := m.board.columns(m)[core.StatusTodo]
	out := m.board.renderColumn(m, core.StatusTodo, items, m.board.scroll[core.StatusTodo], g.colW[core.StatusTodo], m.bodyHeight(), m.board.selectedID(m))
	return strings.Split(ansi.Strip(out), "\n")
}

func TestColumnWithHiddenCardsFillsItsRowsWithTheNextCardCut(t *testing.T) {
	for h := 20; h <= 40; h++ {
		m := newModel(t, longColumn(20), 120, h)
		lines := todoColumn(m)
		inner := lines[1 : len(lines)-1]
		for i, l := range inner {
			if strings.TrimSpace(strings.Trim(l, "│")) == "" {
				t.Errorf("height %d: empty row %d in the column:\n%s", h, i, strings.Join(lines, "\n"))
			}
		}
		if last := inner[len(inner)-1]; !strings.Contains(last, "↓") {
			t.Errorf("height %d: the last row is %q, want the more marker", h, last)
		}
	}
}

func TestMoreMarkerCountsTheCutCardAsHidden(t *testing.T) {
	m := newModel(t, longColumn(20), 120, 25) // leaves two rows for a cut card
	g := m.board.geometry(m)
	items := m.board.columns(m)[core.StatusTodo]
	end, _, _, _ := m.board.visible(m, items, 0, g.colW[core.StatusTodo], cardsArea(g))
	lines := todoColumn(m)
	cut := false
	for _, l := range lines {
		if strings.Contains(l, fmt.Sprintf("KB-%d", end+1)) {
			cut = true
		}
	}
	if !cut {
		t.Fatalf("the next card KB-%d is not shown cut:\n%s", end+1, strings.Join(lines, "\n"))
	}
	want := fmt.Sprintf("↓ %d more", len(items)-end)
	if last := lines[len(lines)-2]; !strings.Contains(last, want) {
		t.Fatalf("marker %q, want %q", last, want)
	}
}

func TestMovingDownOntoTheCutCardScrollsItFullyIntoView(t *testing.T) {
	m := newModel(t, longColumn(20), 120, 25)
	g := m.board.geometry(m)
	items := m.board.columns(m)[core.StatusTodo]
	end, _, _, _ := m.board.visible(m, items, 0, g.colW[core.StatusTodo], cardsArea(g))
	for range end {
		press(t, m, "down")
	}
	if m.board.row != end {
		t.Fatalf("selected row %d, want %d", m.board.row, end)
	}
	id := items[end].ID
	s := ansi.Strip(strings.Join(todoColumn(m), "\n"))
	// A whole card shows its ID and its closing border line.
	if !strings.Contains(s, id) || !strings.Contains(s, "╰") {
		t.Fatalf("selected card %s not shown whole:\n%s", id, s)
	}
	e2, _, _, _ := m.board.visible(m, items, m.board.scroll[core.StatusTodo], g.colW[core.StatusTodo], cardsArea(g))
	if m.board.row >= e2 {
		t.Fatalf("selected row %d is outside the whole cards shown (end %d)", m.board.row, e2)
	}
}

func TestBoardGoldenAt80By24WithACutCard(t *testing.T) {
	m := newModel(t, longColumn(12), 80, 24)
	golden(t, "board_cut_card_80x24", screen(m))
}
