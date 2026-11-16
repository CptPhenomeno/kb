package tui

import (
	"testing"

	"github.com/CptPhenomeno/kb/internal/core"
)

func TestEveryThemeDefinesAColorForEachTagColorName(t *testing.T) {
	for _, p := range Themes {
		for _, name := range core.TagColors {
			if p.TagColor(name) == nil {
				t.Errorf("%s: no color for tag color %q", p.Name, name)
			}
		}
	}
}

func TestTagColorsReachContrastOnBaseAndSurfaceInEveryTheme(t *testing.T) {
	for _, p := range Themes {
		for _, name := range core.TagColors {
			c := p.TagColor(name)
			if c == nil {
				continue
			}
			if r := contrastRatio(c, p.Base); r < 3 {
				t.Errorf("%s: tag %s has %.2f:1 on Base, want 3:1", p.Name, name, r)
			}
			if r := contrastRatio(c, p.Surface); r < 3 {
				t.Errorf("%s: tag %s has %.2f:1 on Surface, want 3:1", p.Name, name, r)
			}
		}
	}
}

func TestTagColorsOfAThemeAreAtLeast30Apart(t *testing.T) {
	for _, p := range Themes {
		for i, a := range core.TagColors {
			for _, b := range core.TagColors[i+1:] {
				ca, cb := p.TagColor(a), p.TagColor(b)
				if ca == nil || cb == nil {
					continue
				}
				if d := rgbDistance(ca, cb); d < 30 {
					t.Errorf("%s: tags %s and %s are %.1f apart, want at least 30", p.Name, a, b, d)
				}
			}
		}
	}
}
