package tui

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func barPrefModel(t *testing.T, opts Options) *Model {
	t.Helper()
	opts.Project, opts.Actor, opts.Now, opts.Loc = "api-gateway", "@dev", func() time.Time { return testNow }, time.UTC
	m := New(context.Background(), sampleBoard(), opts)
	m.tickEvery = 0
	send(t, m, windowSize(160, 48))
	runCmd(t, m, m.Init(), 0)
	return m
}

func TestPaletteShowsFullCommandBarInCompactMode(t *testing.T) {
	m := newModel(t, sampleBoard(), 160, 48)
	press(t, m, "ctrl+p")
	got := names(m.palette.matches(m))
	if !contains(got, "Show full command bar") || contains(got, "Show compact command bar") {
		t.Fatalf("compact mode palette: %v", got)
	}
}

func TestPaletteShowsCompactCommandBarInFullMode(t *testing.T) {
	m := newModel(t, sampleBoard(), 160, 48)
	m.barMode = barFull
	press(t, m, "ctrl+p")
	got := names(m.palette.matches(m))
	if !contains(got, "Show compact command bar") || contains(got, "Show full command bar") {
		t.Fatalf("full mode palette: %v", got)
	}
}

func TestPaletteActionSwitchesTheBarMode(t *testing.T) {
	m := barPrefModel(t, Options{})
	press(t, m, "ctrl+p", "f", "u", "l", "l", " ", "c", "o", "m", "m", "enter")
	if m.palette != nil || m.barMode != barFull {
		t.Fatalf("palette %v, mode %v; want closed and full", m.palette, m.barMode)
	}
	press(t, m, "ctrl+p", "c", "o", "m", "p", "a", "c", "t", "enter")
	if m.barMode != barCompact {
		t.Fatalf("mode = %v, want compact again", m.barMode)
	}
}

func TestCommandBarModeIsSaved(t *testing.T) {
	var saved []string
	m := barPrefModel(t, Options{SaveCommandBar: func(s string) error { saved = append(saved, s); return nil }})
	press(t, m, "ctrl+p", "f", "u", "l", "l", " ", "c", "o", "m", "m", "enter")
	press(t, m, "ctrl+p", "c", "o", "m", "p", "a", "c", "t", "enter")
	if strings.Join(saved, ",") != "full,compact" {
		t.Fatalf("saved = %v, want full then compact", saved)
	}
	m2 := barPrefModel(t, Options{SaveCommandBar: func(string) error { return errors.New("disk full") }})
	press(t, m2, "ctrl+p", "f", "u", "l", "l", " ", "c", "o", "m", "m", "enter")
	if m2.barMode != barFull || !strings.Contains(m2.notice, "could not save") {
		t.Fatalf("mode %v, notice %q; want full and a save warning", m2.barMode, m2.notice)
	}
}

func TestSavedCommandBarModeIsReadAtStartup(t *testing.T) {
	if m := barPrefModel(t, Options{CommandBar: "full"}); m.barMode != barFull {
		t.Fatalf("mode = %v, want full", m.barMode)
	}
}

func TestUnknownCommandBarValueFallsBackToCompact(t *testing.T) {
	for _, v := range []string{"", "bogus", "FULL "} {
		if m := barPrefModel(t, Options{CommandBar: v}); m.barMode != barCompact {
			t.Errorf("command_bar %q gave mode %v, want compact", v, m.barMode)
		}
	}
}
