package tui

import (
	"regexp"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/CptPhenomeno/kb/internal/core"
)

// derivedBoard has a story and an epic with points below, and a story and an
// epic with none.
func derivedBoard() *fakeBackend {
	items := []*core.Item{
		item("KB-1", core.TypeEpic, core.StatusDoing, "", "Epic with work"),
		item("KB-2", core.TypeStory, core.StatusDoing, "KB-1", "Story with work"),
		item("KB-3", core.TypeTask, core.StatusDoing, "KB-2", "Three", points(3)),
		item("KB-4", core.TypeTask, core.StatusTodo, "KB-2", "Five", points(5)),
		item("KB-5", core.TypeStory, core.StatusTodo, "", "Empty story"),
		item("KB-6", core.TypeEpic, core.StatusTodo, "", "Empty epic"),
		item("KB-7", core.TypeTask, core.StatusTodo, "", "Own points", points(2)),
	}
	return &fakeBackend{snap: core.NewSnapshot(items, nil), version: 1}
}

func TestStoryCardShowsDerivedPointsInTheStoryColor(t *testing.T) {
	m := newModel(t, derivedBoard(), 160, 48)
	raw := renderCard(m, m.snap.Get("KB-2"), 28, false)
	if !strings.Contains(raw, fgSeq(m, core.TypeStory)+"m 8pt") {
		t.Fatalf("8pt is not in the story color %s: %q", fgSeq(m, core.TypeStory), raw)
	}
	if strings.Contains(ansi.Strip(raw), "Σ") {
		t.Fatalf("card has a symbol: %q", ansi.Strip(raw))
	}
}

func TestEpicCardShowsDerivedPointsInTheEpicColor(t *testing.T) {
	m := newModel(t, derivedBoard(), 160, 48)
	raw := renderCard(m, m.snap.Get("KB-1"), 28, false)
	if !strings.Contains(raw, fgSeq(m, core.TypeEpic)+"m 8pt") {
		t.Fatalf("8pt is not in the epic color %s: %q", fgSeq(m, core.TypeEpic), raw)
	}
}

func TestCardWithoutDerivedPointsShowsNoPoints(t *testing.T) {
	m := newModel(t, derivedBoard(), 160, 48)
	for _, id := range []string{"KB-5", "KB-6"} {
		if got := ansi.Strip(renderCard(m, m.snap.Get(id), 28, false)); ptRe.MatchString(got) {
			t.Errorf("%s shows points:\n%s", id, got)
		}
	}
	// a task keeps its own muted points
	raw := renderCard(m, m.snap.Get("KB-7"), 28, false)
	if !strings.Contains(ansi.Strip(raw), "2pt") || strings.Contains(raw, fgSeq(m, core.TypeTask)+"m 2pt") {
		t.Errorf("task points changed: %q", raw)
	}
}

func TestDetailShowsDerivedPointsFromChildren(t *testing.T) {
	m := newModel(t, derivedBoard(), 160, 48)
	selectID(t, m, "KB-2")
	press(t, m, "enter")
	rows := detailRows(m)
	i := rowIndex(rows, "points")
	if i < 0 || !strings.Contains(rows[i], "points    8 (from children)") {
		t.Fatalf("detail points row %d:\n%s", i, screen(m))
	}
	var raw string
	for _, l := range strings.Split(m.View().Content, "\n") {
		if strings.Contains(ansi.Strip(l), "(from children)") {
			raw = l
		}
	}
	if !strings.Contains(raw, fgSeq(m, core.TypeStory)+"m8") {
		t.Errorf("derived points not in the story color: %q", raw)
	}
	selectID(t, m, "KB-5")
	press(t, m, "esc", "enter")
	if strings.Contains(screen(m), "from children") {
		t.Errorf("story without points shows the hint")
	}
}

func TestSortByPointsUsesDerivedPointsForStories(t *testing.T) {
	m := newModel(t, derivedBoard(), 160, 48)
	items := []*core.Item{m.snap.Get("KB-5"), m.snap.Get("KB-2"), m.snap.Get("KB-7")}
	boardSort{field: sortPoints}.apply(m.snap, items)
	var ids []string
	for _, it := range items {
		ids = append(ids, it.ID)
	}
	if got := strings.Join(ids, " "); got != "KB-2 KB-7 KB-5" {
		t.Fatalf("order = %s, want KB-2 KB-7 KB-5", got)
	}
}

var ptRe = regexp.MustCompile(`\d+pt`)
