package core_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/CptPhenomeno/kb/internal/core"
)

func TestTagNameIsTrimmedBeforeValidation(t *testing.T) {
	h := newHarness(t)
	tag, err := h.svc.AddTag(ctx, "dev", "  api \t", "")
	if err != nil {
		t.Fatal(err)
	}
	if tag.Name != "api" {
		t.Fatalf("name = %q, want trimmed", tag.Name)
	}
}

func TestTagNameRules(t *testing.T) {
	cases := map[string]string{
		"empty":    "   ",
		"comma":    "a,b",
		"too long": strings.Repeat("x", 25),
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)
			_, err := h.svc.AddTag(ctx, "dev", in, "")
			if !errors.Is(err, core.ErrRule) {
				t.Fatalf("AddTag(%q) err = %v, want rule error", in, err)
			}
		})
	}
	h := newHarness(t)
	if _, err := h.svc.AddTag(ctx, "dev", strings.Repeat("é", 24), ""); err != nil {
		t.Fatalf("24 characters must be accepted: %v", err)
	}
}

func TestTagNamesDifferingOnlyInCaseConflict(t *testing.T) {
	h := newHarness(t)
	if _, err := h.svc.AddTag(ctx, "dev", "API", ""); err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"api", "Api", "ÉCOLE"} {
		if n == "ÉCOLE" {
			if _, err := h.svc.AddTag(ctx, "dev", "école", ""); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := h.svc.AddTag(ctx, "dev", n, ""); !errors.Is(err, core.ErrConflict) {
			t.Fatalf("AddTag(%q) err = %v, want conflict", n, err)
		}
	}
}

func TestTagKeepsItsCreationSpelling(t *testing.T) {
	h := newHarness(t)
	if _, err := h.svc.AddTag(ctx, "dev", "GraphQL", ""); err != nil {
		t.Fatal(err)
	}
	tags, err := h.svc.Tags(ctx)
	if err != nil || len(tags) != 1 || tags[0].Name != "GraphQL" {
		t.Fatalf("tags = %+v, err = %v", tags, err)
	}
}

func TestNewTagsTakeColorsInRotation(t *testing.T) {
	h := newHarness(t)
	want := []string{"red", "orange", "yellow", "green", "cyan", "blue", "purple", "pink"}
	for i, c := range want {
		tag, err := h.svc.AddTag(ctx, "dev", "t"+string(rune('a'+i)), "")
		if err != nil || string(tag.Color) != c {
			t.Fatalf("tag %d: color %q err %v, want %s", i, tag.Color, err, c)
		}
	}
	// An explicit color moves the rotation: the next one follows it.
	if _, err := h.svc.AddTag(ctx, "dev", "x", "blue"); err != nil {
		t.Fatal(err)
	}
	tag, err := h.svc.AddTag(ctx, "dev", "y", "")
	if err != nil || tag.Color != "purple" {
		t.Fatalf("after blue got %q (%v), want purple", tag.Color, err)
	}
}

func TestTagRotationWrapsFromPinkToRed(t *testing.T) {
	h := newHarness(t)
	if _, err := h.svc.AddTag(ctx, "dev", "a", "pink"); err != nil {
		t.Fatal(err)
	}
	tag, err := h.svc.AddTag(ctx, "dev", "b", "")
	if err != nil || tag.Color != "red" {
		t.Fatalf("after pink got %q (%v), want red", tag.Color, err)
	}
}

func TestUnknownTagColorIsARuleErrorListingTheColors(t *testing.T) {
	h := newHarness(t)
	_, err := h.svc.AddTag(ctx, "dev", "a", "magenta")
	if !errors.Is(err, core.ErrRule) {
		t.Fatalf("err = %v, want rule error", err)
	}
	for _, c := range core.TagColors {
		if !strings.Contains(err.Error(), string(c)) {
			t.Errorf("error %q does not list %s", err, c)
		}
	}
}
