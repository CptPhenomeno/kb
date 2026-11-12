package tui

import (
	"context"
	"image/color"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestTOpensThemePicker(t *testing.T) {
	m := newModel(t, sampleBoard(), 160, 48)
	press(t, m, "t")
	s := screen(m)
	if m.picker == nil || !strings.Contains(s, "Theme") {
		t.Fatalf("t did not open the picker:\n%s", s)
	}
	for _, name := range ThemeNames() {
		if !strings.Contains(s, name) {
			t.Errorf("picker does not list %s", name)
		}
	}
	if bar := lastLines(s, 2); !strings.Contains(bar, "preview") || !strings.Contains(bar, "apply") {
		t.Errorf("command bar does not explain the picker keys:\n%s", bar)
	}
}

func TestPickerPreviewsWhileMoving(t *testing.T) {
	m := newModel(t, sampleBoard(), 160, 48)
	press(t, m, "t", "down")
	if m.th.p.Name != Themes[1].Name {
		t.Fatalf("moving down previews %s, want %s", m.th.p.Name, Themes[1].Name)
	}
	// The picker starts on the current theme: climb to the top of the list
	// and one more step to wrap around to the last theme.
	order := pickerOrder()
	pos := 0
	for i, idx := range order {
		if idx == 1 {
			pos = i
		}
	}
	for i := 0; i <= pos; i++ {
		press(t, m, "up")
	}
	if last := order[len(Themes)-1]; m.th.p.Name != Themes[last].Name {
		t.Fatalf("moving up from the first theme previews %s, want the last one", m.th.p.Name)
	}
}

func TestPickerEscRestoresPreviousTheme(t *testing.T) {
	m := newModel(t, sampleBoard(), 160, 48)
	press(t, m, "t", "down", "down", "esc")
	if m.picker != nil || m.th.p.Name != "tokyo-night" {
		t.Fatalf("after esc: picker %v, theme %s", m.picker, m.th.p.Name)
	}
}

func TestPickerEnterAppliesAndSavesTheme(t *testing.T) {
	m := newModel(t, sampleBoard(), 160, 48)
	var saved string
	m.opts.SaveTheme = func(name string) error { saved = name; return nil }
	press(t, m, "t", "down", "enter")
	if m.picker != nil || m.th.p.Name != Themes[1].Name || saved != Themes[1].Name {
		t.Fatalf("after enter: picker %v, theme %s, saved %q", m.picker, m.th.p.Name, saved)
	}
	if !strings.Contains(lastLines(screen(m), 3), "saved") {
		t.Fatalf("status line does not confirm the saved theme:\n%s", lastLines(screen(m), 3))
	}
	// Board keys work again once the picker is closed.
	press(t, m, "right")
	if m.selectedID() != "KB-5" {
		t.Fatalf("board navigation after the picker selected %s", m.selectedID())
	}
}

func TestThemePrecedenceEnvUserProject(t *testing.T) {
	cases := []struct{ env, user, project, want string }{
		{"dracula", "nord", "gruvbox-dark", "dracula"},
		{"", "nord", "gruvbox-dark", "nord"},
		{"", "", "gruvbox-dark", "gruvbox-dark"},
		{"", "", "", ""},
	}
	for _, c := range cases {
		if got := ResolveTheme(c.env, c.user, c.project); got != c.want {
			t.Errorf("ResolveTheme(%q, %q, %q) = %q, want %q", c.env, c.user, c.project, got, c.want)
		}
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

func TestThemePickerPopupCentered(t *testing.T) {
	m := newModel(t, sampleBoard(), 160, 48)
	press(t, m, "t")
	x, y := find(t, screen(m), "Theme")
	if x < 40 || x > 100 || y < 5 || y > 30 {
		t.Fatalf("picker title at (%d,%d), want it near the center of a 160×48 screen", x, y)
	}
}

func TestBoardStaysVisibleAroundThePopup(t *testing.T) {
	m := newModel(t, sampleBoard(), 160, 48)
	press(t, m, "t")
	if g := m.board.geometry(m); g.colX[4]+g.colW[4] != m.width {
		t.Fatalf("the picker narrows the board; it must float over the full width")
	}
	s := screen(m)
	for _, col := range []string{"Backlog", "Done"} {
		if !strings.Contains(s, col) {
			t.Errorf("column %s hidden by the picker", col)
		}
	}
}

func TestPickerSnapshot(t *testing.T) {
	m := newModel(t, sampleBoard(), 160, 48)
	press(t, m, "t", "down")
	golden(t, "picker_160x48", screen(m))
}

// TestEveryRoleReadableOnBaseAndSurface requires, on every theme, Text at
// 4.5:1 and every other text or indicator role at 3:1 against both the
// screen background and the cards.
func TestEveryRoleReadableOnBaseAndSurface(t *testing.T) {
	for _, p := range Themes {
		roles := []struct {
			name string
			c    color.Color
			min  float64
		}{
			{"Text", p.Text, 4.5},
			{"Muted", p.Muted, 3}, {"Accent", p.Accent, 3}, {"Human", p.Human, 3},
			{"Agent", p.Agent, 3}, {"Warn", p.Warn, 3}, {"Red", p.Red, 3},
			{"Green", p.Green, 3}, {"Purple", p.Purple, 3}, {"Cyan", p.Cyan, 3},
		}
		for _, r := range roles {
			for _, bg := range []struct {
				name string
				c    color.Color
			}{{"Base", p.Base}, {"Surface", p.Surface}} {
				if got := contrastRatio(r.c, bg.c); got < r.min {
					t.Errorf("%s: %s on %s contrast %.2f, want at least %.1f", p.Name, r.name, bg.name, got, r.min)
				}
			}
		}
	}
}

func TestPickerGroupsDarkBeforeLight(t *testing.T) {
	m := newModel(t, sampleBoard(), 160, 48)
	press(t, m, "t")
	s := screen(m)
	d, l := strings.Index(s, "Dark"), strings.Index(s, "Light")
	if d < 0 || l < 0 || d > l {
		t.Fatalf("headings missing or out of order (Dark at %d, Light at %d):\n%s", d, l, s)
	}
	for _, p := range Themes {
		i := strings.Index(s, p.Name)
		if i < 0 {
			t.Fatalf("%s not listed", p.Name)
		}
		if p.Light && i < l || !p.Light && (i < d || i > l) {
			t.Errorf("%s (light=%v) is in the wrong group", p.Name, p.Light)
		}
	}
}

func TestPickerKbThemesFirstInEachGroup(t *testing.T) {
	saved := Themes
	defer func() { Themes = saved }()
	var others []Palette
	for _, p := range saved {
		if !strings.HasPrefix(p.Name, "kb-") {
			others = append(others, p)
		}
	}
	Themes = append(others, withName(saved[0], "kb-test-dark"), withName(saved[2], "kb-test-light"))
	order := pickerOrder()
	var dark, light []string
	for _, i := range order {
		if Themes[i].Light {
			light = append(light, Themes[i].Name)
		} else {
			dark = append(dark, Themes[i].Name)
		}
	}
	if dark[0] != "kb-test-dark" || dark[1] != "tokyo-night" {
		t.Errorf("dark group order: %v", dark)
	}
	if light[0] != "kb-test-light" || light[1] != "catppuccin-latte" {
		t.Errorf("light group order: %v", light)
	}
}

func withName(p Palette, name string) Palette { p.Name = name; return p }

func TestPickerDownSkipsLightHeading(t *testing.T) {
	m := newModel(t, sampleBoard(), 160, 48)
	press(t, m, "t")
	order := pickerOrder()
	m.picker.cursor = order[lastDark(order)]
	m.setTheme(m.picker.cursor)
	press(t, m, "down")
	if !Themes[m.picker.cursor].Light || m.picker.cursor != order[lastDark(order)+1] {
		t.Fatalf("down from the last dark theme landed on %s", Themes[m.picker.cursor].Name)
	}
	if m.th.p.Name != Themes[m.picker.cursor].Name {
		t.Errorf("preview is %s, cursor on %s", m.th.p.Name, Themes[m.picker.cursor].Name)
	}
}

func lastDark(order []int) int {
	n := -1
	for k, i := range order {
		if !Themes[i].Light {
			n = k
		}
	}
	return n
}

func TestPickerUpSkipsDarkHeadingAtTop(t *testing.T) {
	m := newModel(t, sampleBoard(), 160, 48)
	press(t, m, "t")
	order := pickerOrder()
	m.picker.cursor = order[0]
	press(t, m, "up")
	if m.picker.cursor != order[len(order)-1] || !Themes[m.picker.cursor].Light {
		t.Fatalf("up from the first theme landed on %s, want the last light theme", Themes[m.picker.cursor].Name)
	}
}

func TestPickerRowsHaveNoDarkLightColumn(t *testing.T) {
	m := newModel(t, sampleBoard(), 160, 48)
	press(t, m, "t")
	for _, line := range strings.Split(screen(m), "\n") {
		for _, p := range Themes {
			if strings.Contains(line, p.Name) && (strings.Contains(line, " dark ") || strings.Contains(line, " light ")) {
				t.Errorf("row still has a dark/light column: %q", line)
			}
		}
	}
}

func TestPickerKeepsSamplesAndCurrent(t *testing.T) {
	m := newModel(t, sampleBoard(), 160, 48)
	press(t, m, "t")
	s := screen(m)
	if strings.Count(s, "■■■■■■") < len(Themes) {
		t.Errorf("color samples missing:\n%s", s)
	}
	if strings.Count(s, "(current)") != 1 {
		t.Errorf("want exactly one (current) marker")
	}
}

// rgbDistance is the euclidean distance between two colors over 0-255
// channels.
func rgbDistance(a, b color.Color) float64 {
	ar, ag, ab, _ := a.RGBA()
	br, bg, bb, _ := b.RGBA()
	d := func(x, y uint32) float64 { return (float64(x) - float64(y)) / 257 }
	return math.Sqrt(d(ar, br)*d(ar, br) + d(ag, bg)*d(ag, bg) + d(ab, bb)*d(ab, bb))
}

// requireDistinctRoles fails for every theme where the two roles are closer
// than 30 apart.
func requireDistinctRoles(t *testing.T, nameA, nameB string, pick func(Palette) (color.Color, color.Color)) {
	t.Helper()
	for _, p := range Themes {
		a, b := pick(p)
		if d := rgbDistance(a, b); d < 30 {
			t.Errorf("%s: %s and %s are %.1f apart, want at least 30", p.Name, nameA, nameB, d)
		}
	}
}

func TestHumanAndAccentAreDistinctOnEveryTheme(t *testing.T) {
	requireDistinctRoles(t, "Human", "Accent", func(p Palette) (color.Color, color.Color) { return p.Human, p.Accent })
}

func TestAgentAndCyanAreDistinctOnEveryTheme(t *testing.T) {
	requireDistinctRoles(t, "Agent", "Cyan", func(p Palette) (color.Color, color.Color) { return p.Agent, p.Cyan })
}

func TestWarnAndRedAreDistinctOnEveryTheme(t *testing.T) {
	requireDistinctRoles(t, "Warn", "Red", func(p Palette) (color.Color, color.Color) { return p.Warn, p.Red })
}
