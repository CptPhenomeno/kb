package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/CptPhenomeno/kb/internal/core"
)

func withTags(names ...string) func(*core.Item) {
	return func(it *core.Item) { it.Tags = names }
}

var testTags = []core.Tag{{Name: "api", Color: "blue"}, {Name: "ui", Color: "green"}, {Name: "infrastructure", Color: "red"}}

// taggedBoard has cards with 0, 1 and 3 tags in the To do column, the last
// one with a name too long for a narrow column.
func taggedBoard() *fakeBackend {
	items := []*core.Item{
		item("KB-1", core.TypeTask, core.StatusTodo, "", "No tags here", pos(0), points(2)),
		item("KB-2", core.TypeTask, core.StatusTodo, "", "One tag", pos(2), prio(core.PriorityHigh), points(3), withTags("api"),
			phase(core.PhaseRed), tests(core.TestFailing, core.TestPending), claimedBy("@gabriele", testNow.Add(10*60e9))),
		item("KB-3", core.TypeTask, core.StatusTodo, "", "Three tags", pos(1), withTags("infrastructure", "api", "ui")),
		item("KB-4", core.TypeTask, core.StatusTodo, "", "Tag and assignee", pos(3), withTags("ui"), claimedBy("@dev", testNow.Add(10*60e9))),
	}
	return &fakeBackend{snap: core.NewSnapshot(items, nil), version: 1, tags: testTags}
}

func cardPlain(m *Model, it *core.Item, w int) []string {
	return strings.Split(ansi.Strip(renderCard(m, it, w, false)), "\n")
}

func TestCardWithoutTagsKeepsItsLines(t *testing.T) {
	m := newModel(t, taggedBoard(), 160, 48)
	got := cardPlain(m, m.snap.Get("KB-1"), 28)
	if len(got) != 4 { // border, header, title, border
		t.Fatalf("untagged card has %d rows:\n%s", len(got), strings.Join(got, "\n"))
	}
	for _, l := range got {
		if strings.Contains(l, "╭─") && strings.Contains(l, "api") {
			t.Errorf("untagged card shows a tag: %q", l)
		}
	}
}

func TestCardWithOneTagShowsItOnTheStatusRowAndNoPlusN(t *testing.T) {
	m := newModel(t, taggedBoard(), 160, 48)
	it := &core.Item{ID: "KB-9", Type: core.TypeTask, Title: "T", Tags: []string{"api"}, Assignee: "@dev"}
	got := cardPlain(m, it, 28)
	// border, header, title, tag + assignee row, border
	if len(got) != 5 || !strings.Contains(got[3], "⟨api⟩") || !strings.Contains(got[3], "@dev") || strings.Contains(strings.Join(got, "\n"), "+") {
		t.Errorf("one tag card:\n%s", strings.Join(got, "\n"))
	}
}

func TestCardWithThreeTagsShowsTheFirstAndMutedPlusTwo(t *testing.T) {
	m := newModel(t, taggedBoard(), 160, 48)
	it := m.snap.Get("KB-3")
	raw := renderCard(m, it, 28, false)
	s := ansi.Strip(raw)
	if !strings.Contains(s, "⟨infrastructure⟩ +2") {
		t.Errorf("three tags card:\n%s", s)
	}
	if strings.Contains(s, "⟨api⟩") {
		t.Errorf("second tag shown:\n%s", s)
	}
	for _, l := range strings.Split(raw, "\n") {
		if strings.Contains(l, "+2") && !strings.Contains(l, "38;2;") {
			t.Errorf("+2 not colored: %q", l)
		}
	}
}

func TestTagSitsBeforeTheStatusPartsWhenTheRowFits(t *testing.T) {
	m := newModel(t, taggedBoard(), 160, 48)
	got := cardPlain(m, m.snap.Get("KB-2"), 38)
	// border, header, title, status row, border
	if len(got) != 5 {
		t.Fatalf("rows = %d:\n%s", len(got), strings.Join(got, "\n"))
	}
	row := got[3]
	i, j, k := strings.Index(row, "⟨api⟩"), strings.Index(row, "RED"), strings.Index(row, "@gabriele")
	if i < 0 || j < i || k < j || !strings.Contains(row, "0/2") {
		t.Errorf("status row %q", row)
	}
}

func TestTagMovesToItsOwnRowWhenTheStatusRowIsFull(t *testing.T) {
	m := newModel(t, taggedBoard(), 160, 48)
	got := cardPlain(m, m.snap.Get("KB-2"), 26)
	s := strings.Join(got, "\n")
	// border, header, title, tag row, status row, border
	if len(got) != 6 || strings.TrimSpace(strings.Trim(got[3], "│")) != "⟨api⟩" ||
		!strings.Contains(got[4], "RED") || !strings.Contains(got[4], "@gabriele") {
		t.Errorf("tag row:\n%s", s)
	}
}

func TestNarrowColumnDropsPlusNBeforeTruncatingTheTagName(t *testing.T) {
	m := newModel(t, taggedBoard(), 80, 24)
	it := &core.Item{ID: "KB-9", Type: core.TypeTask, Title: "T", Tags: []string{"api", "ui", "x"}}
	// inner width 11: "⟨api⟩ +2" needs 8, fits; inner 6: only "⟨api⟩" fits.
	if s := strings.Join(cardPlain(m, it, 15), "\n"); !strings.Contains(s, "⟨api⟩ +2") {
		t.Errorf("wide enough:\n%s", s)
	}
	s := strings.Join(cardPlain(m, it, 9), "\n")
	if !strings.Contains(s, "⟨api⟩") || strings.Contains(s, "+2") {
		t.Errorf("plus N not dropped first:\n%s", s)
	}
	s = strings.Join(cardPlain(m, m.snap.Get("KB-3"), 15), "\n")
	if !strings.Contains(s, "⟨infrastr…⟩") || strings.Contains(s, "+2") {
		t.Errorf("name not truncated after dropping plus N:\n%s", s)
	}
	for _, l := range strings.Split(s, "\n") {
		if displayWidth(l) > 15 {
			t.Errorf("row too wide: %q", l)
		}
	}
}

func TestBoardGoldenWithTagsAt160By48(t *testing.T) {
	golden(t, "board_tags_160x48", screen(newModel(t, taggedBoard(), 160, 48)))
}

func TestBoardGoldenWithTagsAt80By24(t *testing.T) {
	golden(t, "board_tags_80x24", screen(newModel(t, taggedBoard(), 80, 24)))
}

func taggedColumn(n int) *fakeBackend {
	var items []*core.Item
	for i := range n {
		opts := []func(*core.Item){pos(i)}
		if i%2 == 0 {
			opts = append(opts, withTags("api", "ui"))
		}
		items = append(items, item(fmt.Sprintf("KB-%d", i+1), core.TypeTask, core.StatusTodo, "", fmt.Sprintf("Task %d", i+1), opts...))
	}
	return &fakeBackend{snap: core.NewSnapshot(items, nil), version: 1, tags: testTags}
}

func TestColumnScrollingKeepsTheSelectedTaggedCardVisible(t *testing.T) {
	m := newModel(t, taggedColumn(20), 120, 30)
	for range 12 {
		press(t, m, "down")
		g := m.board.geometry(m)
		items := m.board.columns(m)[core.StatusTodo]
		end, _, _, _ := m.board.visible(m, items, m.board.scroll[core.StatusTodo], g.colW[core.StatusTodo], cardsArea(g))
		if m.board.row >= end || m.board.row < m.board.scroll[core.StatusTodo] {
			t.Fatalf("row %d outside shown cards [%d,%d)", m.board.row, m.board.scroll[core.StatusTodo], end)
		}
	}
}

func TestMoreIndicatorCountsCardsWithAnExtraTagRow(t *testing.T) {
	m := newModel(t, taggedColumn(20), 120, 30)
	g := m.board.geometry(m)
	items := m.board.columns(m)[core.StatusTodo]
	w := g.colW[core.StatusTodo]
	end, _, _, below := m.board.visible(m, items, 0, w, cardsArea(g))
	if !below {
		t.Fatal("expected hidden cards")
	}
	used := 0
	for _, it := range items[:end] {
		used += cardHeight(m, it, w)
	}
	// A tagged card without status parts or assignee has one row more.
	plain := newModel(t, longColumn(20), 120, 30)
	pend, _, _, _ := plain.board.visible(plain, plain.board.columns(plain)[core.StatusTodo], 0, w, cardsArea(g))
	if end > pend {
		t.Errorf("tagged column fits %d cards, untagged %d", end, pend)
	}
	if h, p := cardHeight(m, items[0], w), cardHeight(m, items[1], w); h != p+1 {
		t.Errorf("tagged card is %d rows, untagged %d", h, p)
	}
	want := fmt.Sprintf("↓ %d more", len(items)-end)
	if !strings.Contains(strings.Join(todoColumn(m), "\n"), want) {
		t.Errorf("column lacks %q", want)
	}
	if used > cardsArea(g) {
		t.Errorf("shown cards need %d rows of %d", used, cardsArea(g))
	}
}
