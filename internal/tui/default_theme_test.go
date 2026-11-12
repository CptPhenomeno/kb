package tui

import (
	"context"
	"strings"
	"testing"
)

func TestDefaultThemeIsKbNeonRain(t *testing.T) {
	if Themes[0].Name != "kb-neon-rain" {
		t.Fatalf("Themes[0] = %s, want kb-neon-rain", Themes[0].Name)
	}
	m := New(context.Background(), sampleBoard(), Options{Theme: ResolveTheme("", "", "")})
	if m.th.p.Name != "kb-neon-rain" || m.notice != "" {
		t.Fatalf("theme = %s, notice = %q", m.th.p.Name, m.notice)
	}
}

func TestSavedThemeIsKeptOverTheDefault(t *testing.T) {
	name := ResolveTheme("", "nord", "gruvbox-dark")
	m := New(context.Background(), sampleBoard(), Options{Theme: name})
	if m.th.p.Name != "nord" {
		t.Fatalf("theme = %s, want nord", m.th.p.Name)
	}
}

func TestKbThemeEnvOverridesSavedTheme(t *testing.T) {
	name := ResolveTheme("tokyo-night", "nord", "gruvbox-dark")
	m := New(context.Background(), sampleBoard(), Options{Theme: name})
	if m.th.p.Name != "tokyo-night" {
		t.Fatalf("theme = %s, want tokyo-night", m.th.p.Name)
	}
}

func TestUnknownSavedThemeFallsBackToKbNeonRain(t *testing.T) {
	name := ResolveTheme("", "no-such-theme", "nord")
	m := New(context.Background(), sampleBoard(), Options{Theme: name})
	if m.th.p.Name != "kb-neon-rain" || !strings.Contains(m.notice, `unknown theme "no-such-theme"`) ||
		!strings.Contains(m.notice, "using kb-neon-rain") {
		t.Fatalf("theme = %s, notice = %q", m.th.p.Name, m.notice)
	}
}

func TestGoldensUseKbNeonRainAsTheDefaultTheme(t *testing.T) {
	m := newModel(t, sampleBoard(), 160, 48)
	if m.th.p.Name != "kb-neon-rain" {
		t.Fatalf("default test model uses %s, want kb-neon-rain", m.th.p.Name)
	}
	press(t, m, "t")
	if !strings.Contains(screen(m), "kb-neon-rain (current)") {
		t.Fatalf("picker does not mark kb-neon-rain as current:\n%s", screen(m))
	}
}
