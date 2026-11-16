package tui

import (
	"strings"
	"testing"

	"github.com/CptPhenomeno/kb/internal/core"
)

func TestEnterOnATagSuggestionInsertsTheTagFilter(t *testing.T) {
	for _, view := range []string{"", "tab"} {
		m := newModel(t, suggestBoard(), 160, 48)
		if view != "" {
			press(t, m, view)
		}
		press(t, m, "/")
		typeText(t, m, "alpha")
		press(t, m, "down", "enter")
		if !m.board.searching || m.board.query != "tag:alpha-t " {
			t.Errorf("view %q: searching=%v query=%q, want the search open on %q", view, m.board.searching, m.board.query, "tag:alpha-t ")
		}
		if len(m.suggestions()) != 0 {
			t.Errorf("view %q: the list must close after the insert", view)
		}
	}
}

func TestEnterOnATypeSuggestionInsertsTheTypeFilter(t *testing.T) {
	m := newModel(t, suggestBoard(), 160, 48)
	press(t, m, "/")
	typeText(t, m, "type:sto")
	press(t, m, "down", "enter")
	if !m.board.searching || m.board.query != "type:story " {
		t.Errorf("searching=%v query=%q, want the search open on %q", m.board.searching, m.board.query, "type:story ")
	}
}

func TestInsertedFilterReplacesOnlyTheCurrentWord(t *testing.T) {
	m := newModel(t, suggestBoard(), 160, 48)
	press(t, m, "/")
	typeText(t, m, "zzz  type:bug alpha")
	press(t, m, "down", "enter")
	if want := "zzz  type:bug tag:alpha-t "; m.board.query != want {
		t.Errorf("query = %q, want %q", m.board.query, want)
	}
}

func TestEnterOnACardSelectsItOnTheBoardAndClearsTheSearch(t *testing.T) {
	m := newModel(t, suggestBoard(), 160, 48)
	press(t, m, "/")
	typeText(t, m, "refresh")
	press(t, m, "down", "enter")
	if m.board.searching || m.board.query != "" {
		t.Errorf("searching=%v query=%q, want the search closed and cleared", m.board.searching, m.board.query)
	}
	if got := m.board.selectedID(m); got != "KB-4" {
		t.Errorf("selected %q, want KB-4", got)
	}
}

func TestEnterOnACardInTheTreeOpensItsBranches(t *testing.T) {
	m := newModel(t, suggestBoard(), 160, 48)
	press(t, m, "tab", "x") // Tree, every branch closed
	if m.tree.selectedID(m) == "KB-13" {
		t.Fatal("KB-13 must start hidden")
	}
	press(t, m, "/")
	typeText(t, m, "bounded")
	press(t, m, "down", "enter")
	if m.board.searching || m.board.query != "" {
		t.Errorf("searching=%v query=%q, want the search closed and cleared", m.board.searching, m.board.query)
	}
	if got := m.tree.selectedID(m); got != "KB-13" {
		t.Errorf("selected %q, want KB-13", got)
	}
	if !strings.Contains(screen(m), "Bounded route label") {
		t.Error("the branches above KB-13 are not open")
	}
}

func TestEnterAcceptsTheHighlightedSuggestionAfterAReload(t *testing.T) {
	be := suggestBoard()
	m := newModel(t, be, 160, 48)
	press(t, m, "/", "t", "down", "down") // beta-t
	if s := m.selectedSuggestion(); s == nil || s.value != "beta-t" {
		t.Fatalf("selected %v, want beta-t", s)
	}
	// A new tag sorts in front and shifts the list under the highlight.
	be.tags = append([]core.Tag{{Name: "aaa-t", Color: "blue"}}, be.tags...)
	be.version++
	reload(t, m)
	press(t, m, "enter")
	if m.board.query != "tag:beta-t " {
		t.Errorf("query = %q, want the highlighted tag:beta-t", m.board.query)
	}
}

func TestEnterAfterTheHighlightedSuggestionVanishesDoesNothingHarmful(t *testing.T) {
	be := suggestBoard()
	m := newModel(t, be, 160, 48)
	press(t, m, "/", "t", "down") // alpha-t
	if s := m.selectedSuggestion(); s == nil || s.value != "alpha-t" {
		t.Fatalf("selected %v, want alpha-t", s)
	}
	be.tags = be.tags[1:]
	be.version++
	reload(t, m)
	if m.selectedSuggestion() != nil {
		t.Fatal("a vanished suggestion must not stay selected")
	}
	press(t, m, "enter")
	if m.board.query != "t" || m.board.searching {
		t.Errorf("query=%q searching=%v, want the search kept as with no selection", m.board.query, m.board.searching)
	}
}

func TestHelpDescribesTheSearchSuggestions(t *testing.T) {
	for _, v := range []string{"", "tab"} {
		m := newModel(t, sampleBoard(), 160, 48)
		if v != "" {
			press(t, m, v)
		}
		_, _, hints := m.helpHints(0)
		found := false
		for _, h := range hints {
			if h.key == "/" && strings.Contains(h.help, "suggest") {
				found = true
			}
		}
		if !found {
			t.Errorf("view %q: the / hint of the help does not mention suggestions", v)
		}
	}
	var enter string
	for _, h := range searchKeys {
		if h.key == "enter" {
			enter = h.help
		}
	}
	for _, w := range []string{"tag", "type", "card"} {
		if !strings.Contains(enter, w) {
			t.Errorf("the search enter hint %q does not mention %q", enter, w)
		}
	}
}

// spacedTagBoard has a tag with a space, used by KB-5 only.
func spacedTagBoard() *fakeBackend {
	be := sampleBoard()
	be.tags = append(be.tags, core.Tag{Name: "good first", Color: "blue"})
	be.snap.Get("KB-5").Tags = []string{"good first"}
	return be
}

func TestEnterOnATagWithASpaceInsertsAQuotedFilterThatMatches(t *testing.T) {
	m := newModel(t, spacedTagBoard(), 160, 48)
	press(t, m, "/")
	typeText(t, m, "zzz tag:\"good fi") // typing inside an open quote still suggests
	if len(m.suggestions()) != 1 {
		t.Fatalf("suggestions = %v, want the spaced tag", m.suggestions())
	}
	press(t, m, "down", "enter")
	if want := `zzz tag:"good first" `; m.board.query != want {
		t.Fatalf("query = %q, want %q", m.board.query, want)
	}
	m.board.query = `tag:"good first" `
	m.board.restore(m, "")
	if got := m.board.shownCount(m); got != 1 {
		t.Errorf("shown = %d, want only the card with the tag", got)
	}
}

func TestTypedQuotedTagFiltersExactly(t *testing.T) {
	it := &core.Item{ID: "KB-1", Type: core.TypeTask, Title: "x", Tags: []string{"good first"}}
	other := &core.Item{ID: "KB-2", Type: core.TypeTask, Title: "x", Tags: []string{"good", "first"}}
	for q, want := range map[string]bool{`tag:"good first"`: true, `TAG:"GOOD FIRST" x`: true, `tag:"good fi"`: false, `tag:"good first`: true, `"good first"`: true} {
		if got := matchSearch(q, it); got != want {
			t.Errorf("%s on spaced tag = %v, want %v", q, got, want)
		}
	}
	if matchSearch(`tag:"good first"`, other) {
		t.Error("two separate tags must not match the spaced one")
	}
}
