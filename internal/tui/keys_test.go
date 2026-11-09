package tui

import (
	"regexp"
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

// The palette is bound to ctrl+p; it is advertised once it lands (KB-12).
func TestPaletteAdvertisedOnCtrlP(t *testing.T) {
	for _, h := range globalKeys {
		if h.label == "commands" && h.key != "ctrl+p" {
			t.Fatalf("command palette hint uses %q, want ctrl+p", h.key)
		}
	}
}

// missingFeatures are keys whose features have not landed yet. Remove a
// key from here when its feature is implemented.
var missingFeatures = map[viewKind][]string{
	viewBoard:    {"n", "e", "d", "a", "l", "ctrl+p"},
	viewTree:     {"↑ ↓", "← →", "enter", "n", "e", "/", "f"},
	viewActivity: {"↑ ↓", "enter", "esc", "o", "a"},
}

func TestCommandBarHidesKeysNotImplementedYet(t *testing.T) {
	m := newModel(t, sampleBoard(), 160, 48)
	for v, keys := range missingFeatures {
		m.view = v
		bar := lastLines(screen(m), 2)
		for _, k := range keys {
			chip := regexp.MustCompile(`(^|\n|   )` + regexp.QuoteMeta(k) + ` `)
			if chip.MatchString(bar) && !globalWorking(k) {
				t.Errorf("%v view advertises %q, which does nothing yet:\n%s", v, k, bar)
			}
		}
	}
}

func TestHelpHidesKeysNotImplementedYet(t *testing.T) {
	m := newModel(t, sampleBoard(), 160, 48)
	press(t, m, "?")
	help := screen(m)
	for _, want := range []string{"shift ← →", "filter", "theme"} {
		if !strings.Contains(help, want) {
			t.Errorf("help lacks the working key %q", want)
		}
	}
	for _, missing := range []string{"create an item", "open the command palette", "delete the selected item"} {
		if strings.Contains(help, missing) {
			t.Errorf("help advertises %q, which does nothing yet", missing)
		}
	}
}

// globalWorking reports keys that work in every view (tab, ?, q, t).
func globalWorking(k string) bool { return k == "tab" || k == "?" || k == "q" || k == "t" }
