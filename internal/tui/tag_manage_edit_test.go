package tui

import (
	"context"
	"strings"
	"testing"

	"github.com/CptPhenomeno/kb/internal/core"
)

func TestManageTagsRenameInLineUpdatesTheTag(t *testing.T) {
	m, svc := manageModel(t)
	press(t, m, "r")
	if !strings.Contains(screen(m), "API") {
		t.Fatalf("rename input should start from the current name:\n%s", screen(m))
	}
	press(t, m, "backspace", "backspace", "backspace")
	// r, n and d are letters while typing
	typeText(t, m, "rend")
	press(t, m, "enter")
	if _, ok := storedTag(t, svc, "API"); ok {
		t.Fatal("old name still stored")
	}
	tag, ok := storedTag(t, svc, "rend")
	if !ok || tag.Color != "blue" {
		t.Fatalf("renamed tag = %+v, %v", tag, ok)
	}
	it, err := svc.Get(context.Background(), "KB-1")
	if err != nil || len(it.Tags) != 1 || it.Tags[0] != "rend" {
		t.Fatalf("card tags %v, %v", it.Tags, err)
	}
	if m.tagsPopup == nil || m.tagsPopup.mode != tagsList || manageRow(m, "⟨rend⟩") == "" {
		t.Fatalf("popup should be back in the list with the new name:\n%s", screen(m))
	}
}

func TestManageTagsEscCancelsTheRename(t *testing.T) {
	m, svc := manageModel(t)
	press(t, m, "r")
	typeText(t, m, "zzz")
	press(t, m, "esc")
	if m.tagsPopup == nil || m.tagsPopup.mode != tagsList {
		t.Fatal("esc should return to the list, popup open")
	}
	if _, ok := storedTag(t, svc, "API"); !ok {
		t.Fatal("tag was renamed")
	}
	if _, ok := storedTag(t, svc, "APIzzz"); ok {
		t.Fatal("rename leaked")
	}
}

func TestManageTagsRenameToAnExistingTagShowsAnErrorAndKeepsTheName(t *testing.T) {
	m, svc := manageModel(t)
	press(t, m, "r", "backspace", "backspace", "backspace")
	typeText(t, m, "DOCS")
	press(t, m, "enter")
	s := screen(m)
	if !strings.Contains(s, "already exists") || m.alert != nil {
		t.Fatalf("want an in-popup conflict message, alert=%v:\n%s", m.alert, s)
	}
	if m.tagsPopup.mode != tagsRename || m.tagsPopup.input != "DOCS" {
		t.Fatalf("mode %v input %q: the edited name must be kept", m.tagsPopup.mode, m.tagsPopup.input)
	}
	for _, n := range []string{"API", "Docs"} {
		if _, ok := storedTag(t, svc, n); !ok {
			t.Fatalf("tag %s lost", n)
		}
	}
	// the user can fix the name
	press(t, m, "backspace", "backspace", "backspace", "backspace")
	typeText(t, m, "Guide")
	press(t, m, "enter")
	if _, ok := storedTag(t, svc, "Guide"); !ok || m.tagsPopup.mode != tagsList {
		t.Fatalf("fixed name not saved:\n%s", screen(m))
	}
}

func TestManageTagsRenameRejectsAnInvalidName(t *testing.T) {
	m, svc := manageModel(t)
	press(t, m, "r")
	typeText(t, m, ",x")
	press(t, m, "enter")
	if !strings.Contains(screen(m), "comma") || m.tagsPopup.mode != tagsRename {
		t.Fatalf("want a comma message:\n%s", screen(m))
	}
	if _, ok := storedTag(t, svc, "API"); !ok {
		t.Fatal("tag changed")
	}
}

func TestManageTagsRenameChangingOnlyTheCaseSucceeds(t *testing.T) {
	m, svc := manageModel(t)
	press(t, m, "r", "backspace", "backspace", "backspace")
	typeText(t, m, "api")
	press(t, m, "enter")
	tag, ok := storedTag(t, svc, "api")
	if !ok || tag.Name != "api" || m.tagsPopup.mode != tagsList {
		t.Fatalf("case change failed: %+v\n%s", tag, screen(m))
	}
}

func TestManageTagsRenameIsRefusedWhileAColorSaveIsPending(t *testing.T) {
	m, svc := manageModel(t)
	m.wantedColors = map[string]wantedColor{core.TagKey("API"): {"API", core.TagColor("pink")}}
	press(t, m, "r")
	if m.tagsPopup.mode != tagsList || !strings.Contains(screen(m), "color is still being saved") {
		t.Fatalf("rename should be refused with a message:\n%s", screen(m))
	}
	if _, ok := storedTag(t, svc, "API"); !ok {
		t.Fatal("tag changed")
	}
}

func TestManageTagsNCreatesATagWithNameAndColor(t *testing.T) {
	m, svc := manageModel(t)
	press(t, m, "n")
	typeText(t, m, "new, ")
	press(t, m, "backspace", "backspace") // drop ", "
	typeText(t, m, "nd")
	press(t, m, "enter")
	if m.tagsPopup.mode != tagsCreateColor {
		t.Fatalf("name should be followed by the color step:\n%s", screen(m))
	}
	want := core.NextTagColor(m.tagsPopup.last)
	if m.tagsPopup.color != want {
		t.Fatalf("proposed %q want %q", m.tagsPopup.color, want)
	}
	press(t, m, "right")
	chosen := core.TagColors[(indexOfColor(want)+1)%len(core.TagColors)]
	if m.tagsPopup.color != chosen {
		t.Fatalf("right: %q want %q", m.tagsPopup.color, chosen)
	}
	press(t, m, "enter")
	tag, ok := storedTag(t, svc, "newnd")
	if !ok || tag.Color != chosen {
		t.Fatalf("created %+v ok=%v want color %s", tag, ok, chosen)
	}
	if m.tagsPopup.mode != tagsList || manageRow(m, "⟨newnd⟩") == "" {
		t.Fatalf("popup should list the new tag:\n%s", screen(m))
	}
}

func TestManageTagsCreateStartsAtTheNextColorInRotation(t *testing.T) {
	m, svc := manageModel(t)
	last, _ := svc.LastTagColor(context.Background())
	press(t, m, "n")
	typeText(t, m, "x")
	press(t, m, "enter")
	if m.tagsPopup.color != core.NextTagColor(last) {
		t.Fatalf("color %q, last %q", m.tagsPopup.color, last)
	}
	press(t, m, "esc")
	if m.tagsPopup == nil || m.tagsPopup.mode != tagsList {
		t.Fatal("esc should cancel the creation, popup open")
	}
	if _, ok := storedTag(t, svc, "x"); ok {
		t.Fatal("cancelled tag was stored")
	}
}

func TestManageTagsCreateExistingNameShowsAnErrorAndKeepsTheName(t *testing.T) {
	m, _ := manageModel(t)
	press(t, m, "n")
	typeText(t, m, "ui")
	press(t, m, "enter", "enter")
	if !strings.Contains(screen(m), "already exists") || m.tagsPopup.mode != tagsCreateName || m.tagsPopup.input != "ui" {
		t.Fatalf("mode %v input %q:\n%s", m.tagsPopup.mode, m.tagsPopup.input, screen(m))
	}
}

func indexOfColor(c core.TagColor) int {
	for i, x := range core.TagColors {
		if x == c {
			return i
		}
	}
	return -1
}
