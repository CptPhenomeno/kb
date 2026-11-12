package tui

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
)

// withEighteenThemes swaps Themes for 9 dark and 9 light fake themes.
func withEighteenThemes(t *testing.T) {
	t.Helper()
	saved := Themes
	t.Cleanup(func() { Themes = saved })
	var dark, light Palette
	for _, p := range saved {
		if p.Light {
			light = p
		} else {
			dark = p
		}
	}
	Themes = nil
	for i := 0; i < 9; i++ {
		d, l := withName(dark, fmt.Sprintf("dark-%d", i+1)), withName(light, fmt.Sprintf("light-%d", i+1))
		Themes = append(Themes, d, l)
	}
}

func pickerModel(t *testing.T, w, h int) *Model {
	t.Helper()
	withEighteenThemes(t)
	m := newModel(t, sampleBoard(), w, h)
	press(t, m, "t")
	return m
}

func TestPickerWithEighteenThemesFits80By24(t *testing.T) {
	m := pickerModel(t, 80, 24)
	out := m.renderPicker()
	if n := strings.Count(out, "\n") + 1; n > 24 {
		t.Fatalf("picker is %d rows, want at most 24", n)
	}
	if !strings.Contains(screen(m), "dark-1") {
		t.Errorf("selected theme not on screen:\n%s", screen(m))
	}
}

func TestPickerScrollsToKeepSelectionVisibleGoingDown(t *testing.T) {
	m := pickerModel(t, 80, 24)
	for _, i := range pickerOrder() {
		m.picker.cursor = i
		m.setTheme(i)
		if s := screen(m); !strings.Contains(s, "› "+Themes[i].Name) {
			t.Fatalf("selection %s not visible:\n%s", Themes[i].Name, s)
		}
	}
	if s := screen(m); !regexp.MustCompile(`↑ \d+ more`).MatchString(s) || regexp.MustCompile(`↓ \d+ more`).MatchString(s) {
		t.Errorf("at the end want an up marker only:\n%s", s)
	}
}

func TestPickerScrollsBackGoingUp(t *testing.T) {
	m := pickerModel(t, 80, 24)
	order := pickerOrder()
	m.picker.cursor = order[len(order)-1]
	m.setTheme(m.picker.cursor)
	for k := len(order) - 1; k >= 0; k-- {
		if s := screen(m); !strings.Contains(s, "› "+Themes[m.picker.cursor].Name) {
			t.Fatalf("selection %s not visible:\n%s", Themes[m.picker.cursor].Name, s)
		}
		press(t, m, "up")
	}
	press(t, m, "down")
	press(t, m, "down")
	if s := screen(m); regexp.MustCompile(`↑ \d+ more`).MatchString(s) {
		t.Errorf("back at the top there is still an up marker:\n%s", s)
	}
}

func TestPickerThatFitsDoesNotScroll(t *testing.T) {
	m := pickerModel(t, 80, 40)
	for _, i := range pickerOrder() {
		m.picker.cursor = i
		m.setTheme(i)
		s := screen(m)
		if strings.Contains(s, "more") || !strings.Contains(s, "dark-1") || !strings.Contains(s, "light-9") {
			t.Fatalf("picker scrolled or cut rows at 80x40:\n%s", s)
		}
	}
}

func TestPickerGoldenWithEighteenThemesAt80By24(t *testing.T) {
	m := pickerModel(t, 80, 24)
	press(t, m, "down", "down", "down", "down", "down", "down", "down", "down", "down", "down")
	golden(t, "picker_scroll_80x24", screen(m))
}

func TestPickerLeavesHeaderAndCommandBarVisibleAt80By24(t *testing.T) {
	m := pickerModel(t, 80, 24)
	for _, i := range pickerOrder() {
		m.picker.cursor = i
		m.setTheme(i)
		rows := strings.Split(screen(m), "\n")
		if !strings.HasPrefix(rows[0], " kb  api") || !strings.Contains(rows[0], "1 stale") {
			t.Fatalf("header row covered: %q", rows[0])
		}
		bar := strings.Join(rows[len(rows)-2:], "\n")
		if !strings.Contains(bar, "↑ ↓ preview") || !strings.Contains(bar, "enter apply") {
			t.Fatalf("picker keys covered at %s:\n%s", Themes[i].Name, screen(m))
		}
	}
}

func TestPickerMarkerNeverStandsForAHeadingAlone(t *testing.T) {
	m := pickerModel(t, 80, 24)
	order := pickerOrder()
	m.picker.cursor = order[0]
	m.setTheme(order[0])
	// Scroll down until the first theme is the only hidden row above.
	for k := 1; k < len(order); k++ {
		m.picker.cursor = order[k]
		m.setTheme(order[k])
		s := screen(m)
		if strings.Contains(s, "↑ 0 more") || strings.Contains(s, "↓ 0 more") {
			t.Fatalf("empty marker:\n%s", s)
		}
		if strings.Contains(s, "dark-1") && !strings.Contains(s, "Dark") {
			t.Fatalf("heading hidden while its first theme shows:\n%s", s)
		}
		if strings.Contains(s, "↑ 1 more") && strings.Contains(s, "dark-2") && !strings.Contains(s, "dark-1") && strings.Contains(s, "Dark") {
			t.Fatalf("heading and marker both shown:\n%s", s)
		}
	}
}
