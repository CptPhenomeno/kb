package tui

import (
	"strconv"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestTagRendersOnOneLineAsTheNameBetweenAngleBrackets(t *testing.T) {
	th := testTagTheme()
	got := ansi.Strip(th.tagLabel(th.p.TagColor("blue"), "api", 20))
	if got != "⟨api⟩" {
		t.Errorf("tag = %q, want %q", got, "⟨api⟩")
	}
	if strings.Contains(got, "\n") {
		t.Error("tag spans more than one line")
	}
	if displayWidth("⟨") != 1 || displayWidth("⟩") != 1 {
		t.Errorf("brackets are %d and %d cells wide, want 1", displayWidth("⟨"), displayWidth("⟩"))
	}
}

func TestBracketsAndNameUseTheTagColor(t *testing.T) {
	th := testTagTheme()
	c := th.p.TagColor("green")
	r, g, b, _ := c.RGBA()
	seq := "38;2;" + strconv.Itoa(int(r>>8)) + ";" + strconv.Itoa(int(g>>8)) + ";" + strconv.Itoa(int(b>>8))
	got := th.tagLabel(c, "api", 20)
	if !strings.Contains(got, seq) {
		t.Errorf("tag has no foreground %s: %q", seq, got)
	}
	// One style wraps the whole tag, so brackets and name share the color.
	if strings.Count(got, "38;") != 1 {
		t.Errorf("tag should use a single foreground: %q", got)
	}
}

func TestTagSetsNoBackground(t *testing.T) {
	th := testTagTheme()
	if got := th.tagLabel(th.p.TagColor("red"), "api", 20); strings.Contains(got, "48;") {
		t.Errorf("tag sets a background: %q", got)
	}
}

func TestLongTagNameIsTruncatedWithAnEllipsisInsideTheBrackets(t *testing.T) {
	th := testTagTheme()
	got := ansi.Strip(th.tagLabel(th.p.TagColor("blue"), "infrastructure", 8))
	if got != "⟨infra…⟩" {
		t.Errorf("tag = %q, want %q", got, "⟨infra…⟩")
	}
}

func TestTagNeverExceedsTheWidthAndIsEmptyBelowThreeCells(t *testing.T) {
	th := testTagTheme()
	for _, name := range []string{"a", "api", "infrastructure", "日本語のタグ"} {
		for w := 0; w <= 30; w++ {
			got := th.tagLabel(th.p.TagColor("blue"), name, w)
			if w < 3 {
				if got != "" {
					t.Errorf("%q at %d: want empty, got %q", name, w, ansi.Strip(got))
				}
				continue
			}
			plain := ansi.Strip(got)
			if displayWidth(got) > w {
				t.Errorf("%q at %d: %q is %d wide", name, w, plain, displayWidth(got))
			}
			if !strings.HasPrefix(plain, "⟨") || !strings.HasSuffix(plain, "⟩") {
				t.Errorf("%q at %d: %q lacks brackets", name, w, plain)
			}
		}
	}
}
