package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/CptPhenomeno/kb/internal/core"
)

// suggestBoard has five tags and several cards containing a "t".
func suggestBoard() *fakeBackend {
	be := sampleBoard()
	for _, n := range []string{"alpha-t", "beta-t", "gamma-t", "delta-t", "eps-t"} {
		be.tags = append(be.tags, core.Tag{Name: n, Color: "blue"})
	}
	return be
}

func kinds(sg []suggestion, k sugKind) []string {
	var out []string
	for _, s := range sg {
		if s.kind == k {
			out = append(out, s.value)
		}
	}
	return out
}

func TestSuggestionsShowTheThreeGroupsWithHeadings(t *testing.T) {
	for _, view := range []string{"", "tab"} { // Board, then Tree
		m := newModel(t, suggestBoard(), 160, 48)
		if view != "" {
			press(t, m, view)
		}
		press(t, m, "/")
		typeText(t, m, "t")
		box := ansi.Strip(m.renderSuggestions())
		for _, want := range []string{"tags", "types", "cards", "KB-4", "Token refresh", "story"} {
			if !strings.Contains(box, want) {
				t.Errorf("view %q: suggestions lack %q:\n%s", view, want, box)
			}
		}
		if !strings.Contains(screen(m), "cards") {
			t.Errorf("view %q: suggestions not drawn on the screen", view)
		}
	}
}

func TestSuggestionsHideEmptyGroups(t *testing.T) {
	m := newModel(t, suggestBoard(), 160, 48)
	press(t, m, "/")
	typeText(t, m, "kb-4") // only a card
	box := ansi.Strip(m.renderSuggestions())
	if !strings.Contains(box, "cards") || strings.Contains(box, "tags") || strings.Contains(box, "types") {
		t.Errorf("only the cards group must show:\n%s", box)
	}
	m = newModel(t, suggestBoard(), 160, 48)
	press(t, m, "/")
	typeText(t, m, "zzz")
	if got := m.renderSuggestions(); got != "" {
		t.Errorf("no match must draw nothing, got:\n%s", ansi.Strip(got))
	}
}

func TestSuggestionsAreLimitedToThreeTagsTwoTypesAndFiveCards(t *testing.T) {
	m := newModel(t, suggestBoard(), 160, 48)
	press(t, m, "/", "t")
	sg := m.suggestions()
	if n := len(kinds(sg, sugTag)); n != 3 {
		t.Errorf("tags = %d, want 3", n)
	}
	if n := len(kinds(sg, sugType)); n != 2 {
		t.Errorf("types = %d, want 2", n)
	}
	if n := len(kinds(sg, sugCard)); n != 5 {
		t.Errorf("cards = %d, want 5", n)
	}
}

func TestEmptySearchTextShowsNoSuggestions(t *testing.T) {
	m := newModel(t, suggestBoard(), 160, 48)
	press(t, m, "/")
	if len(m.suggestions()) != 0 || m.renderSuggestions() != "" {
		t.Error("empty text must show no suggestions")
	}
	typeText(t, m, "to ") // a trailing space ends the word: nothing being typed
	if len(m.suggestions()) != 0 {
		t.Error("text ending with a space must show no suggestions")
	}
}

func TestTagSuggestionIsACompactOneRowTagInItsColor(t *testing.T) {
	m := newModel(t, suggestBoard(), 160, 48)
	press(t, m, "/")
	typeText(t, m, "alpha")
	raw := m.renderSuggestions()
	label := m.th.tagLabel(m.tagColor("alpha-t"), "alpha-t", 40)
	if !strings.Contains(raw, label) {
		t.Errorf("tag is not the compact colored label %q in:\n%s", label, raw)
	}
	if !strings.Contains(ansi.Strip(raw), "⟨alpha-t⟩") {
		t.Errorf("tag is not drawn as ⟨name⟩:\n%s", ansi.Strip(raw))
	}
}

func TestSuggestionsUseTheCurrentWordAndPrefixes(t *testing.T) {
	m := newModel(t, suggestBoard(), 160, 48)
	press(t, m, "/")
	typeText(t, m, "zzz alpha") // only the word being typed drives the list
	sg := m.suggestions()
	if got := kinds(sg, sugTag); len(got) != 1 || got[0] != "alpha-t" {
		t.Errorf("tags = %v, want [alpha-t]", got)
	}
	press(t, m, "backspace", "backspace", "backspace", "backspace", "backspace")
	typeText(t, m, "tag:BETA")
	sg = m.suggestions()
	if len(sg) != 1 || sg[0].kind != sugTag || sg[0].value != "beta-t" {
		t.Errorf("tag: narrows to the tag group, got %v", sg)
	}
	for range 8 {
		press(t, m, "backspace")
	}
	typeText(t, m, "type:sto")
	sg = m.suggestions()
	if len(sg) != 1 || sg[0].kind != sugType || sg[0].value != "story" {
		t.Errorf("type: narrows to the type group, got %v", sg)
	}
}

func TestCardSuggestionsFollowTheViewFilters(t *testing.T) {
	m := newModel(t, suggestBoard(), 160, 48)
	press(t, m, "/")
	typeText(t, m, "bounded") // KB-13 is a subtask, hidden by default on the Board
	if got := kinds(m.suggestions(), sugCard); len(got) != 0 {
		t.Errorf("hidden subtask suggested: %v", got)
	}
	press(t, m, "esc", "tab", "/")
	typeText(t, m, "bounded") // the Tree shows every type
	if got := kinds(m.suggestions(), sugCard); len(got) != 1 || got[0] != "KB-13" {
		t.Errorf("Tree cards = %v, want [KB-13]", got)
	}
}

func TestArrowsMoveAmongTheSuggestions(t *testing.T) {
	m := newModel(t, suggestBoard(), 160, 48)
	press(t, m, "/", "t")
	total := len(m.suggestions())
	if m.selectedSuggestion() != nil {
		t.Fatal("nothing is selected at first")
	}
	press(t, m, "down")
	first := m.selectedSuggestion()
	if first == nil || first.kind != sugTag {
		t.Fatalf("down selects the first suggestion, got %v", first)
	}
	press(t, m, "down", "down", "down")
	if s := m.selectedSuggestion(); s == nil || s.kind != sugType {
		t.Errorf("fourth suggestion should be a type, got %v", s)
	}
	for range total + 3 {
		press(t, m, "down")
	}
	if s := m.selectedSuggestion(); s == nil || s.kind != sugCard {
		t.Errorf("down stops at the last suggestion (a card), got %v", s)
	}
	for range total {
		press(t, m, "up")
	}
	if m.selectedSuggestion() != nil {
		t.Error("up from the first suggestion goes back to no selection")
	}
	press(t, m, "down")
	typeText(t, m, "o")
	if m.selectedSuggestion() != nil {
		t.Error("typing resets the selection")
	}
	press(t, m, "down")
	if !strings.Contains(ansi.Strip(m.renderSuggestions()), "› ") {
		t.Error("the selected suggestion is not marked")
	}
}

func TestEnterWithoutASelectionKeepsTheSearch(t *testing.T) {
	m := newModel(t, suggestBoard(), 160, 48)
	press(t, m, "/", "t", "o", "k", "enter")
	if m.board.searching || m.board.query != "tok" {
		t.Errorf("searching=%v query=%q, want the search kept", m.board.searching, m.board.query)
	}
	for _, l := range strings.Split(screen(m), "\n") {
		if strings.Trim(l, " │╭╮╰╯─") == "cards" {
			t.Error("suggestions must go away when the search closes")
		}
	}
}
