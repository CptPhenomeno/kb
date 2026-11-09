package tui

import (
	"regexp"
	"strings"
	"testing"
)

var sgrRe = regexp.MustCompile(`\x1b\[([0-9;:]*)m`)

// TestEveryCellHasThemeBackground: every line starts with the theme
// background and is padded to the full width, and the screen fills the
// full height, so a translucent terminal shows no gaps.
func TestEveryCellHasThemeBackground(t *testing.T) {
	m := newModel(t, sampleBoard(), 120, 40)
	bg := bgSeq(m.th.p.Base)
	lines := strings.Split(m.View().Content, "\n")
	if len(lines) != 40 {
		t.Fatalf("%d lines, want 40", len(lines))
	}
	for i, l := range lines {
		if !strings.HasPrefix(l, bg) {
			t.Fatalf("line %d does not start with the background: %q", i, l[:min(30, len(l))])
		}
		if w := displayWidth(l); w != 120 {
			t.Fatalf("line %d is %d cells wide, want 120", i, w)
		}
	}
}

// TestBackgroundSurvivesInnerResets: a style reset inside a line would
// fall back to the terminal's default (translucent) background, so every
// reset must be followed by a background color.
func TestBackgroundSurvivesInnerResets(t *testing.T) {
	for _, name := range ThemeNames() {
		m := newModel(t, sampleBoard(), 120, 40)
		m.setTheme(themeIndex(name))
		press(t, m, "right", "enter") // detail panel too
		content := m.View().Content
		for _, l := range strings.Split(content, "\n") {
			locs := sgrRe.FindAllStringSubmatchIndex(l, -1)
			for _, loc := range locs {
				params := l[loc[2]:loc[3]]
				if !resetsBackground(params) || loc[1] == len(l) {
					continue
				}
				rest := l[loc[1]:]
				if !strings.HasPrefix(rest, "\x1b[48;2;") && !strings.HasPrefix(rest, "\x1b[48:2:") {
					t.Fatalf("%s: reset %q not followed by a background: %q", name, l[loc[0]:loc[1]], rest[:min(20, len(rest))])
				}
			}
		}
	}
}

func TestCardsUseSurfaceColor(t *testing.T) {
	m := newModel(t, sampleBoard(), 120, 40)
	if !strings.Contains(m.View().Content, bgSeq(m.th.p.Surface)) {
		t.Fatal("cards are not painted with the surface color")
	}
}

func TestViewSetsTerminalBackground(t *testing.T) {
	m := newModel(t, sampleBoard(), 120, 40)
	if got := m.View().BackgroundColor; got != m.th.p.Base {
		t.Fatalf("view background = %v, want the theme base %v", got, m.th.p.Base)
	}
}
