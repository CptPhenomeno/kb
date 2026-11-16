package tui

import (
	tea "charm.land/bubbletea/v2"
	"context"
	"strings"
	"testing"

	"github.com/CptPhenomeno/kb/internal/core"
)

// manageModel is a live model with the tags API (blue, two cards), Docs (red,
// no card) and ui (green, one card), and the Manage tags popup open.
func manageModel(t *testing.T) (*Model, *core.Service) {
	t.Helper()
	m, svc := newLiveModel(t)
	ctx := context.Background()
	for _, tg := range [][2]string{{"API", "blue"}, {"ui", "green"}, {"Docs", "red"}} {
		if _, err := svc.AddTag(ctx, "@dev", tg[0], tg[1]); err != nil {
			t.Fatal(err)
		}
	}
	for id, tags := range map[string][]string{"KB-1": {"API"}, "KB-2": {"API", "ui"}} {
		if _, err := svc.Edit(ctx, "@dev", id, core.Patch{Tags: &tags}, 0); err != nil {
			t.Fatal(err)
		}
	}
	reload(t, m)
	press(t, m, "ctrl+p")
	typeText(t, m, "manage tags")
	press(t, m, "enter")
	return m, svc
}

// manageRow is the screen line of the popup that holds s.
func manageRow(m *Model, s string) string {
	for _, l := range strings.Split(screen(m), "\n") {
		if strings.Contains(l, s) && strings.Contains(l, " card") {
			return l
		}
	}
	return ""
}

func TestPaletteListsManageTags(t *testing.T) {
	m := newModel(t, sampleBoard(), 160, 48)
	press(t, m, "ctrl+p")
	if !contains(names(m.palette.matches(m)), "Manage tags") {
		t.Fatalf("palette lacks Manage tags: %v", names(m.palette.matches(m)))
	}
	m, _ = manageModel(t)
	if m.palette != nil || m.tagsPopup == nil || !strings.Contains(screen(m), "Manage tags") {
		t.Fatalf("Manage tags did not open the popup:\n%s", screen(m))
	}
}

func TestManageTagsPopupListsTagsWithColorAndCardCount(t *testing.T) {
	m, _ := manageModel(t)
	for _, c := range []struct{ name, color, cards string }{
		{"⟨API⟩", "blue", "2 cards"}, {"⟨Docs⟩", "red", "0 cards"}, {"⟨ui⟩", "green", "1 card"},
	} {
		row := manageRow(m, c.name)
		if row == "" || !strings.Contains(row, c.color) || !strings.Contains(row, c.cards) {
			t.Errorf("row of %s = %q, want color %q and %q\n%s", c.name, row, c.color, c.cards, screen(m))
		}
	}
	rendered := m.View().Content
	if !strings.Contains(rendered, "⟨API⟩") {
		t.Fatal("tag label missing from the styled screen")
	}
}

func TestManageTagsArrowsChangeAndSaveTheColor(t *testing.T) {
	m, svc := manageModel(t)
	press(t, m, "right") // API: blue → purple
	if tg, _ := storedTag(t, svc, "API"); tg.Color != "purple" {
		t.Fatalf("→ stored %q, want purple", tg.Color)
	}
	if row := manageRow(m, "⟨API⟩"); !strings.Contains(row, "purple") {
		t.Fatalf("popup not repainted: %q", row)
	}
	if m.tagColors[core.TagKey("API")] != "purple" {
		t.Fatalf("board colors not refreshed: %v", m.tagColors)
	}
	press(t, m, "left", "left")
	if tg, _ := storedTag(t, svc, "API"); tg.Color != "cyan" {
		t.Fatalf("← ← stored %q, want cyan", tg.Color)
	}
	press(t, m, "down")
	press(t, m, "left") // Docs: red wraps to pink
	if tg, _ := storedTag(t, svc, "Docs"); tg.Color != "pink" {
		t.Fatalf("← on red stored %q, want pink", tg.Color)
	}
	if tg, _ := storedTag(t, svc, "API"); tg.Color != "cyan" {
		t.Fatalf("recoloring Docs changed API to %q", tg.Color)
	}
}

func TestManageTagsDeleteAsksWithTheCardCount(t *testing.T) {
	m, svc := manageModel(t)
	press(t, m, "d")
	s := screen(m)
	if !strings.Contains(s, "⟨API⟩") || !strings.Contains(s, "2 cards lose it") {
		t.Fatalf("confirmation lacks the tag or card count:\n%s", s)
	}
	press(t, m, "esc")
	if _, ok := storedTag(t, svc, "API"); !ok || m.tagsPopup == nil {
		t.Fatal("esc on the confirmation deleted the tag or closed the popup")
	}
	press(t, m, "down", "d")
	if !strings.Contains(screen(m), "No card uses it") {
		t.Fatalf("an unused tag must say no card loses it:\n%s", screen(m))
	}
	press(t, m, "n")
	press(t, m, "down", "d")
	if !strings.Contains(screen(m), "1 card loses it") {
		t.Fatalf("singular wording:\n%s", screen(m))
	}
}

func TestManageTagsConfirmedDeleteRemovesTheTagFromTheCards(t *testing.T) {
	m, svc := manageModel(t)
	press(t, m, "d", "enter")
	if _, ok := storedTag(t, svc, "API"); ok {
		t.Fatal("tag still stored")
	}
	for _, id := range []string{"KB-1", "KB-2"} {
		it, err := svc.Get(context.Background(), id)
		if err != nil {
			t.Fatal(err)
		}
		for _, tg := range it.Tags {
			if tg == "API" {
				t.Fatalf("%s still has the tag: %v", id, it.Tags)
			}
		}
	}
	if m.tagsPopup == nil || manageRow(m, "⟨API⟩") != "" {
		t.Fatalf("popup should stay open without the tag:\n%s", screen(m))
	}
	if m.tagsPopup.cursor != 0 {
		t.Fatalf("cursor %d", m.tagsPopup.cursor)
	}
}

func TestManageTagsEscClosesThePopup(t *testing.T) {
	m, _ := manageModel(t)
	press(t, m, "esc")
	if m.tagsPopup != nil || strings.Contains(screen(m), "Manage tags") {
		t.Fatalf("esc did not close the popup:\n%s", screen(m))
	}
}

func TestManageTagsKeysAreInHintsAndHelp(t *testing.T) {
	m, _ := manageModel(t)
	bar := strings.Join(m.commandBar(), "\n")
	for _, k := range []string{"↑ ↓", "← →", "d", "esc"} {
		if !strings.Contains(bar, k) {
			t.Errorf("hints lack %q: %s", k, bar)
		}
	}
	press(t, m, "esc", "?")
	for m.helpTab != len(helpTabNames) && m.help {
		press(t, m, "tab")
	}
	if s := screen(m); !strings.Contains(s, "Manage tags") || !strings.Contains(s, "color") {
		t.Fatalf("help lacks the Manage tags tab:\n%s", s)
	}
}

func TestManageTagsRapidColorChangesSaveTheLastChoice(t *testing.T) {
	m, svc := manageModel(t)
	// three → before any command runs: API blue → purple → pink → red
	var cmds []tea.Cmd
	for i := 0; i < 3; i++ {
		_, cmd := m.Update(keyMsg("right"))
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	if row := manageRow(m, "⟨API⟩"); !strings.Contains(row, "red") {
		t.Fatalf("popup must show the last choice at once: %q", row)
	}
	// run the commands newest first, the worst order a scheduler may pick
	for i := len(cmds) - 1; i >= 0; i-- {
		runCmd(t, m, cmds[i], 0)
		if row := manageRow(m, "⟨API⟩"); !strings.Contains(row, "red") {
			t.Fatalf("display stepped back to %q", row)
		}
	}
	if tg, _ := storedTag(t, svc, "API"); tg.Color != "red" {
		t.Fatalf("stored %q, want red", tg.Color)
	}
	if row := manageRow(m, "⟨API⟩"); !strings.Contains(row, "red") {
		t.Fatalf("final display %q", row)
	}
}
