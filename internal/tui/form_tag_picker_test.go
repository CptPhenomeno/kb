package tui

import (
	"context"
	"strings"
	"testing"

	"github.com/CptPhenomeno/kb/internal/core"
)

// tagFormModel is a live board with the tags API, ui and Docs, where KB-2
// carries ui, with the edit form of KB-2 open and the tags field focused.
func tagFormModel(t *testing.T) (*Model, *core.Service) {
	t.Helper()
	m, svc := newLiveModel(t)
	ctx := context.Background()
	for _, tg := range [][2]string{{"API", "blue"}, {"ui", "green"}, {"Docs", "red"}} {
		if _, err := svc.AddTag(ctx, "@dev", tg[0], tg[1]); err != nil {
			t.Fatal(err)
		}
	}
	tags := []string{"ui"}
	if _, err := svc.Edit(ctx, "@dev", "KB-2", core.Patch{Tags: &tags}, 0); err != nil {
		t.Fatal(err)
	}
	reload(t, m)
	selectID(t, m, "KB-2")
	press(t, m, "e")
	for m.form.focus != fTags {
		press(t, m, "down")
	}
	return m, svc
}

func TestFormShowsTheTagsFieldAsACommaList(t *testing.T) {
	m, svc := tagFormModel(t)
	tags := []string{"ui", "API"}
	if _, err := svc.Edit(context.Background(), "@dev", "KB-2", core.Patch{Tags: &tags}, 0); err != nil {
		t.Fatal(err)
	}
	reload(t, m)
	press(t, m, "esc", "e")
	if s := screen(m); !strings.Contains(s, "tags") || !strings.Contains(s, "ui, API") || strings.Contains(s, "labels") {
		t.Fatalf("tags field not a comma list:\n%s", s)
	}
}

func TestEnterOnTheTagsFieldOpensTheSelector(t *testing.T) {
	m, _ := tagFormModel(t)
	press(t, m, "enter")
	if m.form == nil || m.form.picker == nil {
		t.Fatalf("selector not open:\n%s", screen(m))
	}
	s := screen(m)
	for _, want := range []string{"⟨API⟩", "⟨Docs⟩", "⟨ui⟩"} {
		if !strings.Contains(s, want) {
			t.Fatalf("selector misses %s:\n%s", want, s)
		}
	}
}

func TestSelectorMarksTheTagsOfTheCard(t *testing.T) {
	m, _ := tagFormModel(t)
	press(t, m, "enter")
	var marked, plain int
	for _, l := range strings.Split(screen(m), "\n") {
		switch {
		case strings.Contains(l, "[x]") && strings.Contains(l, "⟨ui⟩"):
			marked++
		case strings.Contains(l, "[ ]") && (strings.Contains(l, "⟨API⟩") || strings.Contains(l, "⟨Docs⟩")):
			plain++
		}
	}
	if marked != 1 || plain != 2 {
		t.Fatalf("marks wrong (marked %d, plain %d):\n%s", marked, plain, screen(m))
	}
}

func TestTypingFiltersTheSelectorWithoutCase(t *testing.T) {
	m, _ := tagFormModel(t)
	press(t, m, "enter")
	typeText(t, m, "oC")
	if got := selectorRows(m); len(got) != 1 || !strings.Contains(got[0], "⟨Docs⟩") {
		t.Fatalf("filter 'oC' wrong: %q\n%s", got, screen(m))
	}
	press(t, m, "backspace", "backspace")
	if got := selectorRows(m); len(got) != 3 {
		t.Fatalf("backspace did not widen the filter:\n%s", screen(m))
	}
}

// selectorRows are the screen rows of the selector: the ones with a box.
func selectorRows(m *Model) []string {
	var rows []string
	for _, l := range strings.Split(screen(m), "\n") {
		if strings.Contains(l, "[ ]") || strings.Contains(l, "[x]") {
			rows = append(rows, l)
		}
	}
	return rows
}

func TestSpaceTogglesTheSelectedTag(t *testing.T) {
	m, _ := tagFormModel(t)
	press(t, m, "enter", "space") // first row: API
	if got := strings.Join(m.form.tags, ","); got != "ui,API" {
		t.Fatalf("after adding: %q", got)
	}
	press(t, m, "down", "down", "space") // Docs, then ui: removes ui
	if got := strings.Join(m.form.tags, ","); got != "API" {
		t.Fatalf("after removing ui: %q", got)
	}
	press(t, m, "up", "space") // Docs added
	if got := strings.Join(m.form.tags, ","); got != "API,Docs" {
		t.Fatalf("after adding Docs: %q", got)
	}
}

func TestEscClosesTheSelectorAndKeepsTheForm(t *testing.T) {
	m, _ := tagFormModel(t)
	press(t, m, "enter", "space", "esc")
	if m.form == nil || m.form.picker != nil {
		t.Fatalf("esc should close only the selector:\n%s", screen(m))
	}
	if s := screen(m); !strings.Contains(s, "Edit KB-2") || !strings.Contains(s, "ui, API") {
		t.Fatalf("form does not show the chosen tags:\n%s", s)
	}
}

func TestSavingTheFormStoresTheChosenTags(t *testing.T) {
	m, svc := tagFormModel(t)
	press(t, m, "enter", "space")                         // add API
	press(t, m, "down", "down", "space", "esc", "ctrl+s") // remove ui
	it, err := svc.Get(context.Background(), "KB-2")
	if err != nil {
		t.Fatal(err)
	}
	if len(it.Tags) != 1 || it.Tags[0] != "API" {
		t.Fatalf("stored tags %v, want [API]", it.Tags)
	}
}
