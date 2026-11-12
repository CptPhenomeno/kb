package tui

import (
	"image/color"
	"testing"
)

var rosePineThemes = []string{"rose-pine", "rose-pine-dawn"}

func TestRosePineThemesAreRegisteredAsOneDarkAndOneLight(t *testing.T) {
	if p := kbPalette(t, "rose-pine"); p.Light {
		t.Error("rose-pine must be dark")
	}
	if p := kbPalette(t, "rose-pine-dawn"); !p.Light {
		t.Error("rose-pine-dawn must be light")
	}
	for _, n := range []string{"kb-neon-rain", "kb-synth-sunset", "kb-chrome-haze", "kb-holo-dawn"} {
		if themeIndex(n) >= themeIndex("rose-pine") {
			t.Errorf("%s must come before the external themes", n)
		}
	}
	if themeIndex("kb-neon-rain") != 0 {
		t.Error("kb-neon-rain must stay the first theme")
	}
}

func TestRosePineThemesPassTheContrastTest(t *testing.T) {
	for _, n := range rosePineThemes {
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

func TestRosePineThemesPassTheDistanceTest(t *testing.T) {
	for _, n := range rosePineThemes {
		p := kbPalette(t, n)
		for _, pair := range [][3]any{{"Human/Accent", p.Human, p.Accent}, {"Agent/Cyan", p.Agent, p.Cyan}, {"Warn/Red", p.Warn, p.Red}} {
			if d := rgbDistance(pair[1].(color.Color), pair[2].(color.Color)); d < 30 {
				t.Errorf("%s: %s %.1f apart, want at least 30", n, pair[0], d)
			}
		}
	}
}
