package core_test

import (
	"slices"
	"testing"

	"github.com/CptPhenomeno/kb/internal/core"
)

func withTags(names ...string) func(*core.Item) {
	return func(it *core.Item) { it.Tags = names }
}

func tagColors(t *testing.T, h *harness) map[string]core.TagColor {
	t.Helper()
	tags, err := h.svc.Tags(ctx)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]core.TagColor{}
	for _, tg := range tags {
		out[tg.Name] = tg.Color
	}
	return out
}

func TestCardKeepsTagsInInsertionOrder(t *testing.T) {
	h := newHarness(t)
	it := h.add("dev", core.TypeTask, "t", "", withTags("zeta", "alpha", "mid"))
	if !slices.Equal(it.Tags, []string{"zeta", "alpha", "mid"}) {
		t.Fatalf("tags = %v", it.Tags)
	}
	got, err := h.svc.Get(ctx, it.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got.Tags, []string{"zeta", "alpha", "mid"}) {
		t.Fatalf("stored tags = %v", got.Tags)
	}
}

func TestDuplicateTagDifferingOnlyInCaseIsKeptOnce(t *testing.T) {
	h := newHarness(t)
	it := h.add("dev", core.TypeTask, "t", "", withTags("api", "API", "ui", "Api"))
	if !slices.Equal(it.Tags, []string{"api", "ui"}) {
		t.Fatalf("tags = %v, want [api ui]", it.Tags)
	}
}

func TestAssigningMissingTagCreatesItWithNextRotationColor(t *testing.T) {
	h := newHarness(t)
	if _, err := h.svc.AddTag(ctx, "dev", "first", "blue"); err != nil {
		t.Fatal(err)
	}
	h.add("dev", core.TypeTask, "t", "", withTags("a", "b"))
	c := tagColors(t, h)
	if c["first"] != "blue" || c["a"] != "purple" || c["b"] != "pink" {
		t.Fatalf("colors = %v", c)
	}
	// An edit creates tags the same way, wrapping around the palette.
	if _, err := h.svc.Edit(ctx, "dev", "KB-1", core.Patch{Tags: ptr([]string{"a", "c"})}, 0); err != nil {
		t.Fatal(err)
	}
	if c := tagColors(t, h); c["c"] != "red" {
		t.Fatalf("colors = %v, want c red", c)
	}
}

func TestAssigningExistingTagWithOtherCaseReusesIt(t *testing.T) {
	h := newHarness(t)
	if _, err := h.svc.AddTag(ctx, "dev", "API", ""); err != nil {
		t.Fatal(err)
	}
	it := h.add("dev", core.TypeTask, "t", "", withTags("api"))
	if !slices.Equal(it.Tags, []string{"API"}) {
		t.Fatalf("tags = %v, want the spelling of the tag [API]", it.Tags)
	}
	if len(tagColors(t, h)) != 1 {
		t.Fatal("a second tag was created")
	}
	// Re-sending the same tag in another case changes nothing.
	before := it.Version
	got, err := h.svc.Edit(ctx, "dev", it.ID, core.Patch{Tags: ptr([]string{"Api"})}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != before {
		t.Fatalf("version %d -> %d: no change expected", before, got.Version)
	}
}

func TestChangingTagsOfACardWritesAnEvent(t *testing.T) {
	h := newHarness(t)
	it := h.add("dev", core.TypeTask, "t", "", withTags("api"))
	if _, err := h.svc.Edit(ctx, "dev", it.ID, core.Patch{Tags: ptr([]string{"api", "ui"})}, 0); err != nil {
		t.Fatal(err)
	}
	evs, err := h.svc.History(ctx, it.ID)
	if err != nil {
		t.Fatal(err)
	}
	last := evs[len(evs)-1]
	if last.Kind != core.EventUpdated {
		t.Fatalf("last event = %v", last.Kind)
	}
	if got := core.DescribeEvent(last); got != "updated tags" {
		t.Fatalf("description = %q", got)
	}
}

func TestInvalidTagNameOnACardIsARuleError(t *testing.T) {
	h := newHarness(t)
	_, err := h.svc.Create(ctx, "dev", core.Item{Type: core.TypeTask, Title: "t", Tags: []string{"a,b"}})
	if err == nil {
		t.Fatal("comma accepted")
	}
}
