package tui

import (
	"image/color"
	"testing"
)

func TestNordLightIsRegisteredAsLight(t *testing.T) {
	p := kbPalette(t, "nord-light")
	if !p.Light {
		t.Error("nord-light must be light")
	}
	if themeIndex("nord-light") <= themeIndex("dracula-alucard") {
		t.Error("nord-light must come after dracula-alucard")
	}
}

func TestNordLightPassesTheContrastTest(t *testing.T) {
	p := kbPalette(t, "nord-light")
	roles := map[string]color.Color{"Muted": p.Muted, "Accent": p.Accent, "Human": p.Human,
		"Agent": p.Agent, "Warn": p.Warn, "Red": p.Red, "Green": p.Green, "Purple": p.Purple, "Cyan": p.Cyan}
	for r, c := range roles {
		for _, bg := range []color.Color{p.Base, p.Surface} {
			if got := contrastRatio(c, bg); got < 3 {
				t.Errorf("%s contrast %.2f, want at least 3", r, got)
			}
		}
	}
	for _, bg := range []color.Color{p.Base, p.Surface} {
		if got := contrastRatio(p.Text, bg); got < 4.5 {
			t.Errorf("Text contrast %.2f, want at least 4.5", got)
		}
	}
}

func TestNordLightPassesTheDistanceTest(t *testing.T) {
	p := kbPalette(t, "nord-light")
	for _, pair := range [][3]any{{"Human/Accent", p.Human, p.Accent}, {"Agent/Cyan", p.Agent, p.Cyan}, {"Warn/Red", p.Warn, p.Red}} {
		if d := rgbDistance(pair[1].(color.Color), pair[2].(color.Color)); d < 30 {
			t.Errorf("%s %.1f apart, want at least 30", pair[0], d)
		}
	}
}
