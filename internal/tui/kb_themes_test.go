package tui

import (
	"context"
	"image/color"
	"testing"
	"time"
)

var kbDark = []string{"kb-neon-noir", "kb-synth-dusk"}
var kbLight = []string{"kb-chrome-haze", "kb-holo-dawn"}

func kbPalette(t *testing.T, name string) Palette {
	t.Helper()
	i := themeIndex(name)
	if i < 0 {
		t.Fatalf("theme %s is not registered", name)
	}
	return Themes[i]
}

func TestKbThemesRegisteredTwoDarkTwoLight(t *testing.T) {
	for _, n := range kbDark {
		if p := kbPalette(t, n); p.Light {
			t.Errorf("%s must be dark", n)
		}
	}
	for _, n := range kbLight {
		if p := kbPalette(t, n); !p.Light {
			t.Errorf("%s must be light", n)
		}
	}
}

func TestKbThemesPassContrast(t *testing.T) {
	for _, n := range append(append([]string{}, kbDark...), kbLight...) {
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

func TestKbThemesPassDistance(t *testing.T) {
	for _, n := range append(append([]string{}, kbDark...), kbLight...) {
		p := kbPalette(t, n)
		for _, pair := range [][3]any{{"Human/Accent", p.Human, p.Accent}, {"Agent/Cyan", p.Agent, p.Cyan}, {"Warn/Red", p.Warn, p.Red}} {
			if d := rgbDistance(pair[1].(color.Color), pair[2].(color.Color)); d < 30 {
				t.Errorf("%s: %s %.1f apart, want at least 30", n, pair[0], d)
			}
		}
	}
}

func TestPickerListsKbThemesFirstInEachGroup(t *testing.T) {
	var dark, light []string
	for _, i := range pickerOrder() {
		if Themes[i].Light {
			light = append(light, Themes[i].Name)
		} else {
			dark = append(dark, Themes[i].Name)
		}
	}
	if len(dark) < 2 || dark[0] != "kb-neon-noir" || dark[1] != "kb-synth-dusk" {
		t.Errorf("dark group: %v", dark)
	}
	if len(light) < 2 || light[0] != "kb-chrome-haze" || light[1] != "kb-holo-dawn" {
		t.Errorf("light group: %v", light)
	}
}

func TestKbThemesRenderSameLayout(t *testing.T) {
	want := screen(newModel(t, sampleBoard(), 160, 48))
	for _, n := range append(append([]string{}, kbDark...), kbLight...) {
		kbPalette(t, n)
		m := New(context.Background(), sampleBoard(), Options{Project: "api-gateway", Theme: n,
			Now: func() time.Time { return testNow }, Loc: time.UTC})
		m.tickEvery = 0
		_, _ = m.Update(windowSize(160, 48))
		runCmd(t, m, m.Init(), 0)
		if m.th.p.Name != n {
			t.Errorf("theme %s not applied", n)
		}
		if got := screen(m); got != want {
			t.Errorf("theme %s changes the layout", n)
		}
	}
}
