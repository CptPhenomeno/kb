package tui

import (
	"image/color"
	"math"

	"charm.land/lipgloss/v2"

	"github.com/CptPhenomeno/kb/internal/core"
)

// Palette lists the colors a theme assigns to each role of the UI. Every
// theme paints its own background, so the TUI stays readable on
// translucent terminals.
type Palette struct {
	Name    string
	Light   bool        // light background
	Base    color.Color // screen background
	Surface color.Color // cards and panels
	Overlay color.Color // borders and key hints
	Text    color.Color
	Bright  color.Color // titles
	Muted   color.Color // secondary text
	Accent  color.Color // selection and the kb logo
	Human   color.Color // people (@name)
	Agent   color.Color // coding agents
	Warn    color.Color // blocked, stale, high priority, bugs
	Red     color.Color // failing tests, TDD red
	Green   color.Color // passing tests, done
	Purple  color.Color // review, refactor
	Cyan    color.Color // in progress, tasks
}

func hex(s string) color.Color { return lipgloss.Color(s) }

// Themes are the built-in palettes, built from the official colors of each
// theme. The first one is the default.
//
// Derived (not official) colors, kept near the original hue so that a role
// reaches 3:1 contrast and stays 30 apart from its neighbor role:
//   - catppuccin-latte Warn #e05a00: Peach #fe640b darkened
//   - gruvbox-light Human #946011: Yellow #b57614 darkened
//   - dracula Agent #8ea0f0: periwinkle between Cyan and Purple, Dracula has no blue
//   - solarized-dark Human #dd3d8c: magenta #d33682 lightened
//   - nord Warn #dba06f: Aurora orange #d08770 lightened
var Themes = []Palette{
	{Name: "tokyo-night", Base: hex("#1a1b26"), Surface: hex("#24283b"), Overlay: hex("#414868"),
		Text: hex("#c0caf5"), Bright: hex("#ffffff"), Muted: hex("#a9b1d6"), Accent: hex("#e0af68"),
		Human: hex("#73daca"), Agent: hex("#7aa2f7"), Warn: hex("#ff9e64"), Red: hex("#f7768e"),
		Green: hex("#9ece6a"), Purple: hex("#bb9af7"), Cyan: hex("#7dcfff")},
	{Name: "catppuccin-mocha", Base: hex("#1e1e2e"), Surface: hex("#313244"), Overlay: hex("#585b70"),
		Text: hex("#cdd6f4"), Bright: hex("#f5e0dc"), Muted: hex("#a6adc8"), Accent: hex("#b4befe"),
		Human: hex("#f9e2af"), Agent: hex("#89b4fa"), Warn: hex("#fab387"), Red: hex("#f38ba8"),
		Green: hex("#a6e3a1"), Purple: hex("#cba6f7"), Cyan: hex("#89dceb")},
	{Name: "catppuccin-latte", Light: true, Base: hex("#eff1f5"), Surface: hex("#e6e9ef"), Overlay: hex("#bcc0cc"),
		Text: hex("#4c4f69"), Bright: hex("#4c4f69"), Muted: hex("#5c5f77"), Accent: hex("#8839ef"),
		Human: hex("#e64553"), Agent: hex("#1e66f5"), Warn: hex("#e05a00"), Red: hex("#d20f39"),
		Green: hex("#3d9829"), Purple: hex("#637afd"), Cyan: hex("#179299")},
	{Name: "gruvbox-dark", Base: hex("#282828"), Surface: hex("#3c3836"), Overlay: hex("#665c54"),
		Text: hex("#ebdbb2"), Bright: hex("#fbf1c7"), Muted: hex("#bdae93"), Accent: hex("#fabd2f"),
		Human: hex("#d79921"), Agent: hex("#83a598"), Warn: hex("#fe8019"), Red: hex("#fb4934"),
		Green: hex("#b8bb26"), Purple: hex("#d3869b"), Cyan: hex("#8ec07c")},
	{Name: "gruvbox-light", Light: true, Base: hex("#fbf1c7"), Surface: hex("#f2e5bc"), Overlay: hex("#d5c4a1"),
		Text: hex("#3c3836"), Bright: hex("#282828"), Muted: hex("#665c54"), Accent: hex("#b57614"),
		Human: hex("#946011"), Agent: hex("#076678"), Warn: hex("#af3a03"), Red: hex("#9d0006"),
		Green: hex("#79740e"), Purple: hex("#8f3f71"), Cyan: hex("#427b58")},
	{Name: "dracula", Base: hex("#282a36"), Surface: hex("#343746"), Overlay: hex("#6272a4"),
		Text: hex("#f8f8f2"), Bright: hex("#ffffff"), Muted: hex("#bdc3d9"), Accent: hex("#ff79c6"),
		Human: hex("#f1fa8c"), Agent: hex("#8ea0f0"), Warn: hex("#ffb86c"), Red: hex("#ff5555"),
		Green: hex("#50fa7b"), Purple: hex("#bd93f9"), Cyan: hex("#8be9fd")},
	{Name: "nord", Base: hex("#2e3440"), Surface: hex("#3b4252"), Overlay: hex("#4c566a"),
		Text: hex("#e5e9f0"), Bright: hex("#eceff4"), Muted: hex("#c0c8d6"), Accent: hex("#88c0d0"),
		Human: hex("#ebcb8b"), Agent: hex("#81a1c1"), Warn: hex("#dba06f"), Red: hex("#c7757d"),
		Green: hex("#a3be8c"), Purple: hex("#b48ead"), Cyan: hex("#8fbcbb")},
	{Name: "solarized-dark", Base: hex("#002b36"), Surface: hex("#073642"), Overlay: hex("#586e75"),
		Text: hex("#eee8d5"), Bright: hex("#fdf6e3"), Muted: hex("#93a1a1"), Accent: hex("#b58900"),
		Human: hex("#dd3d8c"), Agent: hex("#268bd2"), Warn: hex("#d44e17"), Red: hex("#de3f3c"),
		Green: hex("#859900"), Purple: hex("#6e73c5"), Cyan: hex("#2aa198")},
}

// ThemeNames returns the names of the built-in themes, default first.
func ThemeNames() []string {
	names := make([]string, len(Themes))
	for i, p := range Themes {
		names[i] = p.Name
	}
	return names
}

// themeIndex returns the index of the named theme, or -1.
func themeIndex(name string) int {
	for i, p := range Themes {
		if p.Name == name {
			return i
		}
	}
	return -1
}

// theme holds the styles derived from a palette.
type theme struct {
	p                                             Palette
	text, bright, muted, warn, badge, stale       lipgloss.Style
	logo, tabOn, tabOff, chip                     lipgloss.Style
	typeColor, phaseColor, statusColor, prioColor map[any]color.Color
	testColor                                     map[core.TestState]color.Color
}

func newTheme(p Palette) *theme {
	s := lipgloss.NewStyle
	return &theme{
		p:      p,
		text:   s().Foreground(p.Text),
		bright: s().Foreground(p.Bright).Bold(true),
		muted:  s().Foreground(p.Muted),
		warn:   s().Foreground(p.Warn),
		badge:  s().Foreground(readableOn(p, p.Warn)).Background(p.Warn).Bold(true),
		stale:  s().Foreground(p.Warn).Bold(true),
		logo:   s().Foreground(readableOn(p, p.Accent)).Background(p.Accent).Bold(true),
		tabOn:  s().Foreground(p.Accent).Background(p.Surface).Bold(true),
		tabOff: s().Foreground(p.Muted),
		chip:   s().Foreground(p.Text).Background(p.Overlay),
		typeColor: map[any]color.Color{core.TypeEpic: p.Accent, core.TypeStory: p.Green, core.TypeTask: p.Cyan,
			core.TypeBug: p.Warn, core.TypeSubtask: p.Muted},
		phaseColor: map[any]color.Color{core.PhaseRed: p.Red, core.PhaseGreen: p.Green, core.PhaseRefactor: p.Purple},
		statusColor: map[any]color.Color{core.StatusBacklog: p.Muted, core.StatusTodo: p.Text, core.StatusDoing: p.Cyan,
			core.StatusReview: p.Purple, core.StatusDone: p.Green},
		prioColor: map[any]color.Color{core.PriorityHigh: p.Warn, core.PriorityMedium: p.Text, core.PriorityLow: p.Muted},
		testColor: map[core.TestState]color.Color{core.TestPassing: p.Green, core.TestFailing: p.Red, core.TestPending: p.Muted},
	}
}

// readableOn returns the palette's background or text color, whichever
// contrasts more with bg: used for text on filled badges.
func readableOn(p Palette, bg color.Color) color.Color {
	if contrastRatio(p.Base, bg) >= contrastRatio(p.Text, bg) {
		return p.Base
	}
	return p.Text
}

// contrastRatio is the WCAG 2 contrast ratio between two colors.
func contrastRatio(a, b color.Color) float64 {
	lum := func(c color.Color) float64 {
		r, g, bl, _ := c.RGBA()
		ch := func(v uint32) float64 {
			x := float64(v) / 0xffff
			if x <= 0.03928 {
				return x / 12.92
			}
			return math.Pow((x+0.055)/1.055, 2.4)
		}
		return 0.2126*ch(r) + 0.7152*ch(g) + 0.0722*ch(bl)
	}
	la, lb := lum(a), lum(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

func (t *theme) fg(c color.Color) lipgloss.Style { return lipgloss.NewStyle().Foreground(c) }

// actor colors people and agents differently.
func (t *theme) actor(actor string) lipgloss.Style {
	switch {
	case actor == "" || actor == "kb":
		return t.muted
	case core.IsHumanActor(actor):
		return t.fg(t.p.Human)
	default:
		return t.fg(t.p.Agent)
	}
}

var prioMarks = map[core.Priority]string{core.PriorityHigh: "▲▲", core.PriorityMedium: "▲", core.PriorityLow: "·"}

var testMarks = map[core.TestState]string{core.TestPassing: "✓", core.TestFailing: "✗", core.TestPending: "○"}
