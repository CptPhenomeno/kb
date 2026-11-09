package tui

import (
	"strings"
	"testing"
)

// Terminals and multiplexers bind many ctrl+shift combinations for
// themselves (WezTerm: ctrl+shift+p is its command palette), so kb must
// never advertise one.
func TestNoAdvertisedCtrlShiftShortcuts(t *testing.T) {
	all := append(append([]keyHint{}, globalKeys...), pickerKeys...)
	for _, hs := range viewKeys {
		all = append(all, hs...)
	}
	for _, h := range all {
		if strings.Contains(h.key, "ctrl+shift") || strings.Contains(h.key, "ctrl shift") {
			t.Errorf("advertised shortcut %q uses ctrl+shift", h.key)
		}
	}
}

func TestPaletteAdvertisedOnCtrlP(t *testing.T) {
	m := newModel(t, sampleBoard(), 160, 48)
	if bar := lastLines(screen(m), 2); !strings.Contains(bar, "ctrl+p commands") {
		t.Fatalf("command bar does not advertise ctrl+p:\n%s", bar)
	}
}
