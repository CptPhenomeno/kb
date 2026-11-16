package tui

import (
	"strings"
	"testing"

	"github.com/CptPhenomeno/kb/internal/core"
)

// manyTags is a board whose first card has enough tags to wrap in the detail.
func manyTagsBoard() *fakeBackend {
	names := []string{"api", "ui", "infrastructure", "backend", "frontend", "release", "security", "docs"}
	it := item("KB-1", core.TypeTask, core.StatusTodo, "", "Many tags", pos(0), phase(core.PhaseRed), withTags(names...))
	plain := item("KB-2", core.TypeTask, core.StatusTodo, "", "No tags", pos(1))
	tags := []core.Tag{{Name: "api", Color: "blue"}, {Name: "ui", Color: "green"}, {Name: "infrastructure", Color: "red"}}
	return &fakeBackend{snap: core.NewSnapshot([]*core.Item{it, plain}, nil), version: 1, tags: tags}
}

func detailRows(m *Model) []string { return strings.Split(screen(m), "\n") }

func rowIndex(rows []string, sub string) int {
	for i, r := range rows {
		if strings.Contains(r, sub) {
			return i
		}
	}
	return -1
}

func TestDetailShowsATagsFieldAfterTheTddPhaseInOrder(t *testing.T) {
	m := newModel(t, manyTagsBoard(), 160, 48)
	press(t, m, "enter")
	rows := detailRows(m)
	ph := rowIndex(rows, "tdd phase")
	tg := rowIndex(rows, "tags      ⟨")
	if ph < 0 || tg != ph+1 {
		t.Fatalf("tags field not right after tdd phase (phase %d, tags %d):\n%s", ph, tg, screen(m))
	}
	line := rows[tg]
	want := "tags      ⟨api⟩ ⟨ui⟩ ⟨infrastructure⟩ ⟨backend⟩ ⟨frontend⟩ ⟨release⟩ ⟨security⟩ ⟨docs⟩"
	if !strings.Contains(line, want) {
		t.Errorf("tags field = %q, want it to contain %q", line, want)
	}
}

func TestDetailWrapsTagsUnderTheValueColumn(t *testing.T) {
	m := newModel(t, manyTagsBoard(), 80, 40)
	press(t, m, "enter")
	rows := detailRows(m)
	first := rowIndex(rows, "tags      ⟨api⟩")
	last := rowIndex(rows, "⟨docs⟩")
	if first < 0 || last != first+1 {
		t.Fatalf("tags did not wrap onto a second line (%d, %d):\n%s", first, last, screen(m))
	}
	col := strings.Index(rows[first], "⟨api⟩")
	if got := strings.Index(rows[last], "⟨"); got != col {
		t.Errorf("continuation starts at col %d, value column is %d: %q", got, col, rows[last])
	}
	golden(t, "detail_tags_field_wrap_80x40", screen(m))
}

func TestDetailWithoutTagsHasNoTagsField(t *testing.T) {
	m := newModel(t, manyTagsBoard(), 160, 48)
	press(t, m, "down", "enter")
	s := screen(m)
	if !strings.Contains(s, "No tags") {
		t.Fatalf("wrong item:\n%s", s)
	}
	if strings.Contains(s, "│ tags      ") {
		t.Errorf("tags field in an untagged detail:\n%s", s)
	}
}

func TestDetailTagWiderThanTheValueColumnIsTruncated(t *testing.T) {
	m := newModel(t, manyTagsBoard(), 80, 40)
	long := strings.Repeat("x", 100)
	for _, r := range m.tagLines([]string{long, "api"}, 30) {
		if w := displayWidth(r); w > 30 {
			t.Errorf("line %q is %d cells wide", r, w)
		}
	}
}
