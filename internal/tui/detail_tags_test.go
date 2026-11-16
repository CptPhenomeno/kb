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

func TestDetailShowsTagsAfterTheTddPhaseInOrder(t *testing.T) {
	m := newModel(t, manyTagsBoard(), 160, 48)
	press(t, m, "enter")
	rows := detailRows(m)
	ph := rowIndex(rows, "tdd phase")
	api := rowIndex(rows, "│ api │")
	if ph < 0 || api <= ph {
		t.Fatalf("tags not after tdd phase (phase %d, api %d):\n%s", ph, api, screen(m))
	}
	line := rows[api]
	i1, i2 := strings.Index(line, "│ api │"), strings.Index(line, "│ ui │")
	if i2 < 0 || i2-(i1+len("│ api │")) != 1 {
		t.Errorf("api and ui not one space apart in order: %q", line)
	}
	if strings.Index(line, "│ infrastructure │") < i2 {
		t.Errorf("order wrong: %q", line)
	}
}

func TestDetailWrapsTagBoxesOntoASecondRow(t *testing.T) {
	m := newModel(t, manyTagsBoard(), 80, 40)
	press(t, m, "enter")
	rows := detailRows(m)
	first := rowIndex(rows, "│ api │")
	last := rowIndex(rows, "│ docs │")
	if first < 0 || last < 0 || last-first < 3 {
		t.Fatalf("tags did not wrap onto further rows (%d, %d):\n%s", first, last, screen(m))
	}
	for _, r := range rows {
		if strings.Contains(r, "│ docs │") && strings.Contains(r, "│ api │") {
			t.Errorf("first and last tag on one row: %q", r)
		}
	}
	golden(t, "detail_tags_wrap_80x40", screen(m))
}

func TestDetailWithoutTagsHasNoTagSection(t *testing.T) {
	m := newModel(t, manyTagsBoard(), 160, 48)
	press(t, m, "down", "enter")
	s := screen(m)
	if !strings.Contains(s, "No tags") {
		t.Fatalf("wrong item:\n%s", s)
	}
	lines := strings.Split(s, "\n")
	in := false
	for _, l := range lines {
		if strings.Contains(l, "status") && strings.Contains(l, "To do") {
			in = true
		}
		if in && strings.Contains(l, "╭─") && strings.Count(l, "╭")+strings.Count(l, "╮") > 2 {
			t.Errorf("tag boxes in an untagged detail: %q", l)
		}
	}
}

func TestDetailTagWiderThanTheRowIsTruncated(t *testing.T) {
	m := newModel(t, manyTagsBoard(), 80, 40)
	long := strings.Repeat("x", 100)
	for _, r := range m.tagRows([]string{long, "api"}, 30) {
		if w := displayWidth(r); w > 30 {
			t.Errorf("row %q is %d cells wide", r, w)
		}
	}
}
