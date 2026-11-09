package tui

import (
	"context"
	"image/color"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestThemeCycle(t *testing.T) {
	m := newModel(t, sampleBoard(), 160, 48)
	if m.th.p.Name != "tokyo-night" {
		t.Fatalf("default theme = %s", m.th.p.Name)
	}
	press(t, m, "t")
	if m.th.p.Name != Themes[1].Name || !strings.Contains(lastLines(screen(m), 3), "theme: "+Themes[1].Name) {
		t.Fatalf("after t: theme %s, status:\n%s", m.th.p.Name, lastLines(screen(m), 3))
	}
	press(t, m, "right")
	if strings.Contains(screen(m), "theme: ") {
		t.Fatal("the theme notice must go away at the next key")
	}
	for range len(Themes) - 1 {
		press(t, m, "t")
	}
	if m.th.p.Name != "tokyo-night" {
		t.Fatalf("cycling through every theme must come back to the first, got %s", m.th.p.Name)
	}
}

func TestConfiguredTheme(t *testing.T) {
	m := New(context.Background(), sampleBoard(), Options{Theme: "dracula"})
	if m.th.p.Name != "dracula" || m.notice != "" {
		t.Fatalf("theme = %s, notice = %q", m.th.p.Name, m.notice)
	}
}

func TestUnknownThemeFallsBack(t *testing.T) {
	m := New(context.Background(), sampleBoard(), Options{Theme: "solarized-neon"})
	if m.th.p.Name != "tokyo-night" || !strings.Contains(m.notice, `unknown theme "solarized-neon"`) ||
		!strings.Contains(m.notice, "catppuccin-mocha") {
		t.Fatalf("theme = %s, notice = %q", m.th.p.Name, m.notice)
	}
}

// TestAllThemesRenderSameLayout checks that themes change colors only.
func TestAllThemesRenderSameLayout(t *testing.T) {
	want := screen(newModel(t, sampleBoard(), 160, 48))
	for _, p := range Themes {
		m := New(context.Background(), sampleBoard(), Options{Project: "api-gateway", Theme: p.Name,
			Now: func() time.Time { return testNow }, Loc: time.UTC})
		m.tickEvery = 0
		_, _ = m.Update(windowSize(160, 48))
		runCmd(t, m, m.Init(), 0)
		if got := screen(m); got != want {
			t.Errorf("theme %s changes the layout", p.Name)
		}
	}
}

func TestThemesAreComplete(t *testing.T) {
	seen := map[string]bool{}
	for _, p := range Themes {
		if seen[p.Name] {
			t.Errorf("duplicate theme %s", p.Name)
		}
		seen[p.Name] = true
		v := reflect.ValueOf(p)
		for i := range v.NumField() {
			if f := v.Type().Field(i); f.Type.Kind() != reflect.Bool && v.Field(i).IsZero() {
				t.Errorf("theme %s: %s is not set", p.Name, f.Name)
			}
		}
	}
	for _, want := range []string{"catppuccin-mocha", "catppuccin-latte", "gruvbox-dark", "gruvbox-light", "dracula", "nord"} {
		if !seen[want] {
			t.Errorf("theme %s missing", want)
		}
	}
}

// TestChipsReadableOnDarkAndLightThemes checks the contrast of every pair
// of colors the UI draws on top of each other.
func TestChipsReadableOnDarkAndLightThemes(t *testing.T) {
	for _, p := range Themes {
		pairs := []struct {
			name   string
			fg, bg color.Color
			min    float64
		}{
			{"text on background", p.Text, p.Base, 4.5},
			{"text on cards", p.Text, p.Surface, 4.5},
			{"titles on cards", p.Bright, p.Surface, 4.5},
			{"muted on background", p.Muted, p.Base, 3},
			{"muted on cards", p.Muted, p.Surface, 3},
			{"key hints", p.Text, p.Overlay, 3},
			{"badges", readableOn(p, p.Warn), p.Warn, 3},
			{"logo", readableOn(p, p.Accent), p.Accent, 3},
			{"agents on cards", p.Agent, p.Surface, 3},
			{"people on cards", p.Human, p.Surface, 3},
			{"in progress on background", p.Cyan, p.Base, 3},
		}
		for _, c := range pairs {
			if r := contrastRatio(c.fg, c.bg); r < c.min {
				t.Errorf("%s: %s contrast %.2f, want at least %.1f", p.Name, c.name, r, c.min)
			}
		}
	}
}
