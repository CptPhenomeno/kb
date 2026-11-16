package tui

import (
	"context"
	"strings"
	"testing"

	"github.com/CptPhenomeno/kb/internal/core"
)

func firstSelectorRow(m *Model) string {
	for _, l := range strings.Split(screen(m), "\n") {
		if strings.Contains(l, "create \"") {
			return l
		}
	}
	return ""
}

func storedTag(t *testing.T, svc *core.Service, name string) (core.Tag, bool) {
	t.Helper()
	tags, err := svc.Tags(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, tg := range tags {
		if core.TagKey(tg.Name) == core.TagKey(name) {
			return tg, true
		}
	}
	return core.Tag{}, false
}

func TestUnmatchedTextShowsACreateRowFirst(t *testing.T) {
	m, _ := tagFormModel(t)
	press(t, m, "enter")
	typeText(t, m, "newtag")
	rows := m.pickerRows()
	if len(rows) != 1 || !rows[0].create || rows[0].tag.Name != "newtag" {
		t.Fatalf("rows %+v, want a single create row", rows)
	}
	if r := firstSelectorRow(m); !strings.Contains(r, `create "newtag"`) {
		t.Fatalf("create row not shown:\n%s", screen(m))
	}
	press(t, m, "backspace", "backspace", "backspace", "backspace", "backspace", "backspace")
	typeText(t, m, "a")
	rows = m.pickerRows()
	if len(rows) < 2 || !rows[0].create || rows[1].create {
		t.Fatalf("create row should come before matching tags: %+v", rows)
	}
}

func TestExactMatchWithoutCaseShowsNoCreateRow(t *testing.T) {
	m, _ := tagFormModel(t)
	press(t, m, "enter")
	typeText(t, m, "docs")
	for _, r := range m.pickerRows() {
		if r.create {
			t.Fatalf("create row for an existing tag: %+v", m.pickerRows())
		}
	}
	if strings.Contains(screen(m), "create \"") {
		t.Fatalf("create row on screen:\n%s", screen(m))
	}
}

func TestArrowsOnTheCreateRowChangeTheProposedColor(t *testing.T) {
	m, _ := tagFormModel(t)
	press(t, m, "enter")
	typeText(t, m, "newtag")
	if got := m.pickerRows()[0].color; got != "orange" {
		t.Fatalf("first proposal %q", got)
	}
	press(t, m, "right")
	if got := m.pickerRows()[0].color; got != "yellow" {
		t.Fatalf("after right %q", got)
	}
	press(t, m, "left", "left")
	if got := m.pickerRows()[0].color; got != "red" {
		t.Fatalf("after two lefts %q", got)
	}
	press(t, m, "left", "left", "left", "left", "left", "left", "left", "left", "left")
	if got := m.pickerRows()[0].color; got != "pink" {
		t.Fatalf("cycling wrong: %q", got)
	}
	if !strings.Contains(firstSelectorRow(m), "pink") {
		t.Fatalf("row does not name the color:\n%s", screen(m))
	}
}

func TestCreateRowProposesTheNextRotationColor(t *testing.T) {
	m, _ := tagFormModel(t) // last created tag is Docs, red
	press(t, m, "enter")
	typeText(t, m, "one")
	if got := m.pickerRows()[0].color; got != core.NextTagColor("red") {
		t.Fatalf("proposal %q", got)
	}
	press(t, m, "space") // creates "one" as orange
	typeText(t, m, "two")
	if got := m.pickerRows()[0].color; got != "yellow" {
		t.Fatalf("second proposal %q, want yellow (after the pending orange)", got)
	}
}

func TestSavedFormStoresTheNewTagWithTheChosenColor(t *testing.T) {
	m, svc := tagFormModel(t)
	press(t, m, "enter")
	typeText(t, m, "fresh")
	press(t, m, "right", "right", "space") // orange -> yellow -> green
	if _, ok := storedTag(t, svc, "fresh"); ok {
		t.Fatal("tag written before the form is saved")
	}
	if got := strings.Join(m.form.tags, ","); got != "ui,fresh" {
		t.Fatalf("form tags %q", got)
	}
	if !strings.Contains(screen(m), "⟨fresh⟩") {
		t.Fatalf("pending tag not listed:\n%s", screen(m))
	}
	press(t, m, "esc", "ctrl+s")
	tg, ok := storedTag(t, svc, "fresh")
	if !ok || tg.Color != "green" {
		t.Fatalf("stored tag %+v ok=%v, want green", tg, ok)
	}
	it, err := svc.Get(context.Background(), "KB-2")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(it.Tags, ",") != "ui,fresh" {
		t.Fatalf("card tags %v", it.Tags)
	}
}

func TestCancelledFormLeavesNoNewTag(t *testing.T) {
	m, svc := tagFormModel(t)
	press(t, m, "enter")
	typeText(t, m, "ghost")
	press(t, m, "space", "esc", "esc")
	if m.form != nil {
		t.Fatalf("form still open:\n%s", screen(m))
	}
	if _, ok := storedTag(t, svc, "ghost"); ok {
		t.Fatal("cancelled form left a tag")
	}
}

func TestFailedSaveLeavesNoNewTag(t *testing.T) {
	m, svc := tagFormModel(t)
	press(t, m, "enter")
	typeText(t, m, "orphan")
	press(t, m, "space", "esc")
	m.form.fields[fTitle].value = "" // card validation fails
	press(t, m, "ctrl+s")
	if m.form == nil {
		t.Fatal("form closed on a failed save")
	}
	if _, ok := storedTag(t, svc, "orphan"); ok {
		t.Fatal("failed save left a stray tag")
	}
}
