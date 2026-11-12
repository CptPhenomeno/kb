package tui

import (
	"image/color"
	"testing"
)

var externalLightThemes = []string{"solarized-light", "tokyo-night-day", "dracula-alucard"}

func TestSolarizedTokyoDayAndAlucardAreRegisteredAsLight(t *testing.T) {
	prev := themeIndex("rose-pine-dawn")
	for _, n := range externalLightThemes {
		if p := kbPalette(t, n); !p.Light {
			t.Errorf("%s must be light", n)
		}
		if i := themeIndex(n); i <= prev {
			t.Errorf("%s must come after rose-pine-dawn and the previous external theme", n)
		} else {
			prev = i
		}
	}
}

func TestExternalLightThemesPassTheContrastTest(t *testing.T) {
	for _, n := range externalLightThemes {
		p := kbPalette(t, n)
		roles := map[string]color.Color{"Muted": p.Muted, "Accent": p.Accent, "Human": p.Human,
			"Agent": p.Agent, "Warn": p.Warn, "Red": p.Red, "Green": p.Green, "Purple": p.Purple, "Cyan": p.Cyan}
		for r, c := range roles {
			for _, bg := range []color.Color{p.Base, p.Surface} {
				if got := contrastRatio(c, bg); got < 3 {
					t.Errorf("%s: %s contrast %.2f, want at least 3", n, r, got)
				}
			}
		}
		for _, bg := range []color.Color{p.Base, p.Surface} {
			if got := contrastRatio(p.Text, bg); got < 4.5 {
				t.Errorf("%s: Text contrast %.2f, want at least 4.5", n, got)
			}
		}
	}
}

func TestExternalLightThemesPassTheDistanceTest(t *testing.T) {
	for _, n := range externalLightThemes {
		p := kbPalette(t, n)
		for _, pair := range [][3]any{{"Human/Accent", p.Human, p.Accent}, {"Agent/Cyan", p.Agent, p.Cyan}, {"Warn/Red", p.Warn, p.Red}} {
			if d := rgbDistance(pair[1].(color.Color), pair[2].(color.Color)); d < 30 {
				t.Errorf("%s: %s %.1f apart, want at least 30", n, pair[0], d)
			}
		}
	}
}
