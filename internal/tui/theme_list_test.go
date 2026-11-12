package tui

import (
	"os"
	"strings"
	"testing"
)

func TestThemesAreNineDarkAndNineLight(t *testing.T) {
	var dark, light int
	for _, p := range Themes {
		if p.Light {
			light++
		} else {
			dark++
		}
	}
	if dark != 9 || light != 9 {
		t.Errorf("got %d dark and %d light themes, want 9 and 9", dark, light)
	}
}

func TestThemeNamesAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, p := range Themes {
		if seen[p.Name] {
			t.Errorf("duplicate theme name %q", p.Name)
		}
		seen[p.Name] = true
	}
}

func TestReadmeListsEveryTheme(t *testing.T) {
	data, err := os.ReadFile("../../README.md")
	if err != nil {
		t.Fatal(err)
	}
	var row string
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "| `KB_THEME`") {
			row = line
		}
	}
	if row == "" {
		t.Fatal("README has no KB_THEME row")
	}
	for _, p := range Themes {
		if !strings.Contains(row, "`"+p.Name+"`") {
			t.Errorf("KB_THEME row does not list %s", p.Name)
		}
	}
	if !strings.Contains(row, "default is `"+Themes[0].Name+"`") {
		t.Errorf("KB_THEME row does not name %s as the default", Themes[0].Name)
	}
}

func TestPickerGoldenWithEveryRealThemeAt80By24(t *testing.T) {
	m := newModel(t, sampleBoard(), 80, 24)
	press(t, m, "t")
	order := pickerOrder()
	for _, i := range order {
		m.picker.cursor = i
		m.setTheme(i)
		if s := screen(m); !strings.Contains(s, "› "+Themes[i].Name) {
			t.Fatalf("selection %s not visible at 80x24:\n%s", Themes[i].Name, s)
		}
	}
	// Land on the first light theme, where the kb themes lead the group.
	press(t, m, "down", "down", "down", "down", "down", "down", "down", "down", "down", "down", "down")
	golden(t, "picker_real_80x24", screen(m))
}
