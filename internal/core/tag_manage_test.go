package core_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/CptPhenomeno/kb/internal/core"
)

func tagChange(t *testing.T, e core.Event) (before, after []any) {
	t.Helper()
	if e.Kind != core.EventUpdated {
		t.Fatalf("event kind = %s, want updated", e.Kind)
	}
	ch, _ := e.Data["changes"].(map[string]any)
	pair, _ := ch["tags"].([]any)
	if len(pair) != 2 {
		t.Fatalf("event data = %v, want a tags change", e.Data)
	}
	b, _ := pair[0].([]any)
	a, _ := pair[1].([]any)
	return b, a
}

func TestTagColorChangeKeepsCardsAndWritesNoCardEvent(t *testing.T) {
	h := newHarness(t)
	c := h.add("dev", core.TypeTask, "t", "", withTags("api"))
	before := len(h.history(c.ID))
	tag, err := h.svc.SetTagColor(ctx, "dev", "API", "blue")
	if err != nil || tag.Color != "blue" || tag.Name != "api" || tag.Cards != 1 {
		t.Fatalf("tag = %+v, err = %v", tag, err)
	}
	got := h.get(c.ID)
	if !slices.Equal(got.Tags, []string{"api"}) || got.Version != c.Version {
		t.Fatalf("card = %v v%d", got.Tags, got.Version)
	}
	if n := len(h.history(c.ID)); n != before {
		t.Fatalf("events %d -> %d, want none", before, n)
	}
	if _, err := h.svc.SetTagColor(ctx, "dev", "api", "magenta"); !errors.Is(err, core.ErrRule) {
		t.Fatalf("err = %v, want rule", err)
	}
}

func TestTagRenameChangesOnlyTheCaseOfATag(t *testing.T) {
	h := newHarness(t)
	c := h.add("dev", core.TypeTask, "t", "", withTags("api"))
	tag, err := h.svc.RenameTag(ctx, "dev", "api", "API")
	if err != nil || tag.Name != "API" {
		t.Fatalf("tag = %+v, err = %v", tag, err)
	}
	if got := h.get(c.ID).Tags; !slices.Equal(got, []string{"API"}) {
		t.Fatalf("card tags = %v", got)
	}
	if _, found, _ := tagByName(t, h, "api"); !found {
		t.Fatal("tag api (any case) must still exist")
	}
}

func tagByName(t *testing.T, h *harness, name string) (core.Tag, bool, error) {
	t.Helper()
	tags, err := h.svc.Tags(ctx)
	for _, tg := range tags {
		if core.TagKey(tg.Name) == core.TagKey(name) {
			return tg, true, err
		}
	}
	return core.Tag{}, false, err
}

func TestTagRenameToAnotherExistingTagIsAConflict(t *testing.T) {
	h := newHarness(t)
	h.add("dev", core.TypeTask, "t", "", withTags("api", "ui"))
	_, err := h.svc.RenameTag(ctx, "dev", "api", "UI")
	if !errors.Is(err, core.ErrConflict) {
		t.Fatalf("err = %v, want conflict", err)
	}
	if tg, _, _ := tagByName(t, h, "api"); tg.Name != "api" {
		t.Fatalf("api changed: %+v", tg)
	}
	if _, err := h.svc.RenameTag(ctx, "dev", "api", "a,b"); !errors.Is(err, core.ErrRule) {
		t.Fatalf("invalid new name err = %v, want rule", err)
	}
}

func TestTagRenameWritesAnEventOnEachCardUsingTheTag(t *testing.T) {
	h := newHarness(t)
	a := h.add("dev", core.TypeTask, "a", "", withTags("x", "api"))
	b := h.add("dev", core.TypeTask, "b", "", withTags("api"))
	c := h.add("dev", core.TypeTask, "c", "", withTags("x"))
	nc := len(h.history(c.ID))
	if _, err := h.svc.RenameTag(ctx, "dev", "Api", "backend"); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{a.ID, b.ID} {
		bf, af := tagChange(t, lastEvent(t, h, id))
		if len(bf) == 0 || len(bf) != len(af) || slices.Equal(bf, af) {
			t.Fatalf("%s change %v -> %v", id, bf, af)
		}
		if got := h.get(id); got.Version != 2 {
			t.Fatalf("%s version = %d, want bumped", id, got.Version)
		}
	}
	if got := h.get(a.ID).Tags; !slices.Equal(got, []string{"x", "backend"}) {
		t.Fatalf("order kept? %v", got)
	}
	if len(h.history(c.ID)) != nc || h.get(c.ID).Version != 1 {
		t.Fatal("card without the tag must not change")
	}
}

func TestTagDeleteRemovesTheTagFromEveryCard(t *testing.T) {
	h := newHarness(t)
	a := h.add("dev", core.TypeTask, "a", "", withTags("x", "api", "y"))
	b := h.add("dev", core.TypeTask, "b", "", withTags("api"))
	n, err := h.svc.DeleteTag(ctx, "dev", "API")
	if err != nil {
		t.Fatal(err)
	}
	if n.Name != "api" || n.Cards != 2 {
		t.Fatalf("deleted = %+v", n)
	}
	if got := h.get(a.ID).Tags; !slices.Equal(got, []string{"x", "y"}) {
		t.Fatalf("a tags = %v", got)
	}
	if got := h.get(b.ID).Tags; len(got) != 0 {
		t.Fatalf("b tags = %v", got)
	}
	if _, found, _ := tagByName(t, h, "api"); found {
		t.Fatal("tag still listed")
	}
}

func TestTagDeleteWritesAnEventOnEachCardThatHadTheTag(t *testing.T) {
	h := newHarness(t)
	a := h.add("dev", core.TypeTask, "a", "", withTags("x", "api"))
	c := h.add("dev", core.TypeTask, "c", "", withTags("x"))
	if _, err := h.svc.DeleteTag(ctx, "dev", "api"); err != nil {
		t.Fatal(err)
	}
	bf, af := tagChange(t, lastEvent(t, h, a.ID))
	if len(bf) != 2 || len(af) != 1 || af[0] != "x" {
		t.Fatalf("change %v -> %v", bf, af)
	}
	if h.get(a.ID).Version != 2 || h.get(c.ID).Version != 1 || len(h.history(c.ID)) != 1 {
		t.Fatal("only the card with the tag changes")
	}
}

func TestMissingTagIsNotFound(t *testing.T) {
	h := newHarness(t)
	if _, err := h.svc.SetTagColor(ctx, "dev", "nope", "red"); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("color err = %v", err)
	}
	if _, err := h.svc.RenameTag(ctx, "dev", "nope", "x"); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("rename err = %v", err)
	}
	if _, err := h.svc.DeleteTag(ctx, "dev", "nope"); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("delete err = %v", err)
	}
}

func TestTagNameArgumentMatchesWithoutCase(t *testing.T) {
	h := newHarness(t)
	h.add("dev", core.TypeTask, "a", "", withTags("École"))
	if _, err := h.svc.SetTagColor(ctx, "dev", "éCOLE", "pink"); err != nil {
		t.Fatal(err)
	}
	if tg, _, _ := tagByName(t, h, "école"); tg.Color != "pink" {
		t.Fatalf("tag = %+v", tg)
	}
	if _, err := h.svc.RenameTag(ctx, "dev", " ÉCOLE ", "Lycée"); err != nil {
		t.Fatal(err)
	}
}

func TestTagNameConflictMessageHasNoVersionText(t *testing.T) {
	h := newHarness(t)
	h.add("dev", core.TypeTask, "t", "", withTags("api", "ui"))
	_, err := h.svc.AddTag(ctx, "dev", "UI", "")
	if !errors.Is(err, core.ErrConflict) || err.Error() != "tag ui already exists" {
		t.Fatalf("add err = %v", err)
	}
	_, err = h.svc.RenameTag(ctx, "dev", "api", "ui")
	if !errors.Is(err, core.ErrConflict) || err.Error() != "tag ui already exists" {
		t.Fatalf("rename err = %v", err)
	}
}
