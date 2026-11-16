package tui

import (
	"strconv"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/CptPhenomeno/kb/internal/core"
)

func testTagTheme() *theme { return newTheme(Themes[0]) }

func plainRows(rows []string) []string {
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = ansi.Strip(r)
	}
	return out
}

func TestTagBoxHasThreeRowsAndRoundedCorners(t *testing.T) {
	th := testTagTheme()
	got := plainRows(th.tagBox(th.p.TagColor("blue"), "api", 20))
	want := []string{"╭─────╮", "│ api │", "╰─────╯"}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("tag box = %q, want %q", got, want)
	}
}

func TestTagBoxBorderAndTextUseTheTagColor(t *testing.T) {
	th := testTagTheme()
	c := th.p.TagColor("green")
	rows := th.tagBox(c, "api", 20)
	r, g, b, _ := c.RGBA()
	seq := "38;2;" + strconv.Itoa(int(r>>8)) + ";" + strconv.Itoa(int(g>>8)) + ";" + strconv.Itoa(int(b>>8))
	for i, row := range rows {
		if !strings.Contains(row, seq) {
			t.Errorf("row %d has no foreground %s: %q", i, seq, row)
		}
	}
}

func TestTagBoxSetsNoBackground(t *testing.T) {
	th := testTagTheme()
	for _, row := range th.tagBox(th.p.TagColor("red"), "api", 20) {
		if strings.Contains(row, "48;") {
			t.Errorf("row sets a background: %q", row)
		}
	}
}

func TestLongTagNameIsTruncatedWithAnEllipsis(t *testing.T) {
	th := testTagTheme()
	got := plainRows(th.tagBox(th.p.TagColor("blue"), "infrastructure", 10))
	want := []string{"╭────────╮", "│ infra… │", "╰────────╯"}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("tag box = %q, want %q", got, want)
	}
}

func TestTagBoxNeverExceedsTheAvailableWidth(t *testing.T) {
	th := testTagTheme()
	for _, name := range []string{"a", "api", "infrastructure", "日本語のタグ"} {
		for w := 0; w <= 30; w++ {
			rows := th.tagBox(th.p.TagColor("blue"), name, w)
			if rows != nil && len(rows) != 3 {
				t.Errorf("%q at %d: %d rows", name, w, len(rows))
			}
			for _, r := range rows {
				if displayWidth(r) > w {
					t.Errorf("%q at %d: row %q is %d wide", name, w, ansi.Strip(r), displayWidth(r))
				}
			}
		}
	}
}

func TestUnknownTagFallsBackToMuted(t *testing.T) {
	m := newModel(t, sampleBoard(), 80, 24)
	m.tagColors = map[string]core.TagColor{"api": "blue"}
	if m.tagColor("API") != m.th.p.TagColor("blue") {
		t.Error("known tag, matched case-insensitively, should use its color")
	}
	if m.tagColor("nope") != m.th.p.Muted {
		t.Error("unknown tag should use Muted")
	}
}
