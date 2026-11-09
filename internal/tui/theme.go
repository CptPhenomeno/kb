package tui

import (
	"image/color"

	"charm.land/lipgloss/v2"

	"github.com/CptPhenomeno/kb/internal/core"
)

// Palette lists the colors a theme assigns to each role of the UI.
type Palette struct {
	Name   string
	Accent color.Color // selection and the kb logo
	Human  color.Color // people (@name)
	Agent  color.Color // coding agents
	Text   color.Color
	Bright color.Color // titles
	Muted  color.Color // secondary text
	Border color.Color
	Warn   color.Color // blocked, stale, high priority, bugs
	Red    color.Color // failing tests, TDD red
	Green  color.Color // passing tests, done
	Purple color.Color // review, refactor
	Cyan   color.Color // in progress, tasks
	Ink    color.Color // text on filled badges
	Chip   color.Color // background of key hints
}

func hex(s string) color.Color { return lipgloss.Color(s) }

// Themes are the built-in palettes. The first one is the default: the
// palette of the mockups in design/project. The others follow the official
// palettes of popular terminal themes.
var Themes = []Palette{
	{Name: "tokyo-night", Accent: hex("#e0af68"), Human: hex("#e0af68"), Agent: hex("#7aa2f7"),
		Text: hex("#d6dae3"), Bright: hex("#e6e9ef"), Muted: hex("#8b93a7"), Border: hex("#3a3f4b"),
		Warn: hex("#ff9e64"), Red: hex("#f7768e"), Green: hex("#9ece6a"), Purple: hex("#bb9af7"),
		Cyan: hex("#7dcfff"), Ink: hex("#0b0d12"), Chip: hex("#c3c8d4")},
	{Name: "catppuccin-mocha", Accent: hex("#f9e2af"), Human: hex("#f9e2af"), Agent: hex("#89b4fa"),
		Text: hex("#cdd6f4"), Bright: hex("#f5e0dc"), Muted: hex("#a6adc8"), Border: hex("#45475a"),
		Warn: hex("#fab387"), Red: hex("#f38ba8"), Green: hex("#a6e3a1"), Purple: hex("#cba6f7"),
		Cyan: hex("#89dceb"), Ink: hex("#1e1e2e"), Chip: hex("#bac2de")},
	{Name: "catppuccin-latte", Accent: hex("#df8e1d"), Human: hex("#df8e1d"), Agent: hex("#1e66f5"),
		Text: hex("#4c4f69"), Bright: hex("#4c4f69"), Muted: hex("#6c6f85"), Border: hex("#bcc0cc"),
		Warn: hex("#fe640b"), Red: hex("#d20f39"), Green: hex("#40a02b"), Purple: hex("#8839ef"),
		Cyan: hex("#04a5e5"), Ink: hex("#eff1f5"), Chip: hex("#5c5f77")},
	{Name: "gruvbox-dark", Accent: hex("#fabd2f"), Human: hex("#fabd2f"), Agent: hex("#83a598"),
		Text: hex("#ebdbb2"), Bright: hex("#fbf1c7"), Muted: hex("#a89984"), Border: hex("#504945"),
		Warn: hex("#fe8019"), Red: hex("#fb4934"), Green: hex("#b8bb26"), Purple: hex("#d3869b"),
		Cyan: hex("#8ec07c"), Ink: hex("#282828"), Chip: hex("#d5c4a1")},
	{Name: "gruvbox-light", Accent: hex("#b57614"), Human: hex("#b57614"), Agent: hex("#076678"),
		Text: hex("#3c3836"), Bright: hex("#282828"), Muted: hex("#7c6f64"), Border: hex("#d5c4a1"),
		Warn: hex("#af3a03"), Red: hex("#9d0006"), Green: hex("#79740e"), Purple: hex("#8f3f71"),
		Cyan: hex("#427b58"), Ink: hex("#fbf1c7"), Chip: hex("#504945")},
	{Name: "dracula", Accent: hex("#ff79c6"), Human: hex("#f1fa8c"), Agent: hex("#8be9fd"),
		Text: hex("#f8f8f2"), Bright: hex("#ffffff"), Muted: hex("#6272a4"), Border: hex("#44475a"),
		Warn: hex("#ffb86c"), Red: hex("#ff5555"), Green: hex("#50fa7b"), Purple: hex("#bd93f9"),
		Cyan: hex("#8be9fd"), Ink: hex("#282a36"), Chip: hex("#f8f8f2")},
	{Name: "nord", Accent: hex("#ebcb8b"), Human: hex("#ebcb8b"), Agent: hex("#88c0d0"),
		Text: hex("#d8dee9"), Bright: hex("#eceff4"), Muted: hex("#7b88a1"), Border: hex("#434c5e"),
		Warn: hex("#d08770"), Red: hex("#bf616a"), Green: hex("#a3be8c"), Purple: hex("#b48ead"),
		Cyan: hex("#8fbcbb"), Ink: hex("#2e3440"), Chip: hex("#d8dee9")},
	{Name: "solarized-dark", Accent: hex("#b58900"), Human: hex("#b58900"), Agent: hex("#268bd2"),
		Text: hex("#93a1a1"), Bright: hex("#eee8d5"), Muted: hex("#839496"), Border: hex("#073642"),
		Warn: hex("#cb4b16"), Red: hex("#dc322f"), Green: hex("#859900"), Purple: hex("#6c71c4"),
		Cyan: hex("#2aa198"), Ink: hex("#002b36"), Chip: hex("#93a1a1")},
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
		badge:  s().Foreground(p.Ink).Background(p.Warn).Bold(true),
		stale:  s().Foreground(p.Warn).Bold(true),
		logo:   s().Foreground(p.Ink).Background(p.Accent).Bold(true),
		tabOn:  s().Foreground(p.Ink).Background(p.Text).Bold(true),
		tabOff: s().Foreground(p.Muted),
		chip:   s().Foreground(p.Ink).Background(p.Chip),
		typeColor: map[any]color.Color{core.TypeEpic: p.Accent, core.TypeStory: p.Green, core.TypeTask: p.Cyan,
			core.TypeBug: p.Warn, core.TypeSubtask: p.Muted},
		phaseColor: map[any]color.Color{core.PhaseRed: p.Red, core.PhaseGreen: p.Green, core.PhaseRefactor: p.Purple},
		statusColor: map[any]color.Color{core.StatusBacklog: p.Muted, core.StatusTodo: p.Text, core.StatusDoing: p.Cyan,
			core.StatusReview: p.Purple, core.StatusDone: p.Green},
		prioColor: map[any]color.Color{core.PriorityHigh: p.Warn, core.PriorityMedium: p.Text, core.PriorityLow: p.Muted},
		testColor: map[core.TestState]color.Color{core.TestPassing: p.Green, core.TestFailing: p.Red, core.TestPending: p.Muted},
	}
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
