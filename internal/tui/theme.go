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

	// Tags paints each tag color name; read it with TagColor.
	Tags map[core.TagColor]color.Color
}

// TagColor returns this theme's color for a tag color name, or nil for an
// unknown name.
func (p Palette) TagColor(c core.TagColor) color.Color { return p.Tags[c] }

func hex(s string) color.Color { return lipgloss.Color(s) }

// tagSet builds the tag colors of a theme from hex strings given in the
// order of core.TagColors (red, orange, yellow, green, cyan, blue, purple,
// pink).
func tagSet(hexes ...string) map[core.TagColor]color.Color {
	m := make(map[core.TagColor]color.Color, len(hexes))
	for i, h := range hexes {
		m[core.TagColors[i]] = hex(h)
	}
	return m
}

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
//   - rose-pine Warn #f0956b: orange between Gold #f6c177 and Love #eb6f92, Rose Pine has no orange
//   - rose-pine Green #8fd0a8: Foam #9ccfd8 turned toward green, Rose Pine has no green
//   - rose-pine-dawn Accent #d16f6b: Rose #d7827e darkened (2.6:1 on Base)
//   - rose-pine-dawn Human #c27914: Gold #ea9d34 darkened (2.1:1 on Base)
//   - rose-pine-dawn Warn #d07125: orange between Gold and Love, darkened to 3:1
//   - rose-pine-dawn Green #3a9a6e: Foam #56949f turned toward green, Rose Pine has no green
//   - solarized-light Overlay #bcc5c4: base1 #93a1a1 lightened (key hints need 3:1 with Text)
//   - solarized-light Text #566b72: base01 #586e75 darkened (4.4:1 on Surface)
//   - solarized-light Accent #a67d00: yellow #b58900 darkened (2.6:1 on Surface)
//   - solarized-light Agent #2588ce: blue #268bd2 darkened (3.0:1 on Surface)
//   - solarized-light Green #7a8c00: green #859900 darkened (2.6:1 on Surface)
//   - solarized-light Cyan #269189: cyan #2aa198 darkened (2.6:1 on Surface)
//   - tokyo-night-day Text #3257ad: foreground #3760bf darkened (4.0:1 on Surface)
//   - tokyo-night-day Human #10856e: teal #118c74 darkened (2.9:1 on Surface)
//   - tokyo-night-day Agent #1e73e7: blue #2e7de9 darkened (2.7:1 on Surface)
//   - tokyo-night-day Red #eb0b4c: red #f52a65 darkened (2.7:1 on Surface)
//   - tokyo-night-day Purple #954ff1: purple #9854f1 darkened (2.9:1 on Surface)
//   - dracula-alucard Surface #f5efd6: between Alucard background and Current Line, which is a blue-gray
//   - dracula-alucard Agent #3a5bc7: blue between Cyan and Purple, Alucard has no blue
//   - nord-light is derived as a whole: Nord has no official light theme. Backgrounds, Overlay, Text, Bright and Muted are official
//     Snow Storm and Polar Night colors; Red and Agent are official Aurora and Frost colors that already pass 3:1
//   - nord-light Accent #418fa5: Frost nord8 #88c0d0 darkened (lightness only)
//   - nord-light Cyan #568f8e: Frost nord7 #8fbcbb darkened
//   - nord-light Human #ad7d1e: Aurora nord13 #ebcb8b darkened
//   - nord-light Warn #ac5438: Aurora nord12 #d08770 darkened further than 3:1 needs, to stay 30 apart from Red
//   - nord-light Green #6d8f51: Aurora nord14 #a3be8c darkened
//   - nord-light Purple #a5779d: Aurora nord15 #b48ead darkened
//   - tag colors (TAG-2) reuse the role colors above when the theme has one of that hue (red, orange,
//     yellow, green, cyan, blue, purple); the ones below are derived or official colors not used by a role
//   - kb-neon-rain tag yellow #ffd84a and pink #ff7ad1: the theme has no such hue, own colors in the neon style
//   - tokyo-night tag pink #ff007c: official magenta2
//   - catppuccin-latte tag yellow #b97618: Yellow #df8e1d darkened (2.3:1 on Base)
//   - catppuccin-latte tag pink #e243b8: Pink #ea76cb darkened (2.3:1 on Base)
//   - gruvbox-dark tag pink #d3869b: official bright purple, which is pink; tag purple #b08ee0: Gruvbox has no violet, derived
//   - gruvbox-light tag pink #8f3f71: official purple, which is magenta; tag purple #6f4fa8: Gruvbox has no violet, derived
//   - nord tag pink #e59ac0: Nord has no pink, derived from Aurora purple nord15 #b48ead turned toward pink
//   - rose-pine-dawn tag yellow #a97f13: Gold #ea9d34 darkened further than 3:1 needs, to stay 30 apart from orange
//   - tokyo-night-day tag pink #c9448c: Tokyo Night has no pink, derived from magenta #bb9af7 turned toward pink and darkened
//   - nord-light tag pink #c0699a: Nord has no pink, derived from nord15 #b48ead turned toward pink and darkened
//
// The external themes (rose-pine, rose-pine-dawn and the light ones) come after the kb themes
// and use the official colors by meaning: Love is Red, Iris is Purple,
// Foam is Cyan, Pine is Agent, Rose is Accent, Gold is Human (dark) and
// Subtle is Muted.
var Themes = []Palette{
	{Name: "kb-neon-rain", Base: hex("#0a0f14"), Surface: hex("#121a22"), Overlay: hex("#2a3a4a"),
		Text: hex("#e0ecf5"), Bright: hex("#ffffff"), Muted: hex("#8aa0b4"), Accent: hex("#ff6a3d"),
		Human: hex("#c3f73a"), Agent: hex("#5eb3ff"), Warn: hex("#ffb347"), Red: hex("#ff3355"),
		Green: hex("#3dffa8"), Purple: hex("#b58cff"), Cyan: hex("#2de2e6"),
		Tags: tagSet("#ff3355", "#ff6a3d", "#ffd84a", "#3dffa8", "#2de2e6", "#5eb3ff", "#b58cff", "#ff7ad1")},
	{Name: "tokyo-night", Base: hex("#1a1b26"), Surface: hex("#24283b"), Overlay: hex("#414868"),
		Text: hex("#c0caf5"), Bright: hex("#ffffff"), Muted: hex("#a9b1d6"), Accent: hex("#e0af68"),
		Human: hex("#73daca"), Agent: hex("#7aa2f7"), Warn: hex("#ff9e64"), Red: hex("#f7768e"),
		Green: hex("#9ece6a"), Purple: hex("#bb9af7"), Cyan: hex("#7dcfff"),
		Tags: tagSet("#f7768e", "#ff9e64", "#e0af68", "#9ece6a", "#7dcfff", "#7aa2f7", "#bb9af7", "#ff007c")},
	{Name: "catppuccin-mocha", Base: hex("#1e1e2e"), Surface: hex("#313244"), Overlay: hex("#585b70"),
		Text: hex("#cdd6f4"), Bright: hex("#f5e0dc"), Muted: hex("#a6adc8"), Accent: hex("#b4befe"),
		Human: hex("#f9e2af"), Agent: hex("#89b4fa"), Warn: hex("#fab387"), Red: hex("#f38ba8"),
		Green: hex("#a6e3a1"), Purple: hex("#cba6f7"), Cyan: hex("#89dceb"),
		Tags: tagSet("#f38ba8", "#fab387", "#f9e2af", "#a6e3a1", "#89dceb", "#89b4fa", "#cba6f7", "#f5c2e7")},
	{Name: "catppuccin-latte", Light: true, Base: hex("#eff1f5"), Surface: hex("#e6e9ef"), Overlay: hex("#bcc0cc"),
		Text: hex("#4c4f69"), Bright: hex("#4c4f69"), Muted: hex("#5c5f77"), Accent: hex("#8839ef"),
		Human: hex("#e64553"), Agent: hex("#1e66f5"), Warn: hex("#e05a00"), Red: hex("#d20f39"),
		Green: hex("#3d9829"), Purple: hex("#637afd"), Cyan: hex("#179299"),
		Tags: tagSet("#d20f39", "#e05a00", "#b97618", "#3d9829", "#179299", "#1e66f5", "#8839ef", "#e243b8")},
	{Name: "gruvbox-dark", Base: hex("#282828"), Surface: hex("#3c3836"), Overlay: hex("#665c54"),
		Text: hex("#ebdbb2"), Bright: hex("#fbf1c7"), Muted: hex("#bdae93"), Accent: hex("#fabd2f"),
		Human: hex("#d79921"), Agent: hex("#83a598"), Warn: hex("#fe8019"), Red: hex("#fb4934"),
		Green: hex("#b8bb26"), Purple: hex("#d3869b"), Cyan: hex("#8ec07c"),
		Tags: tagSet("#fb4934", "#fe8019", "#fabd2f", "#b8bb26", "#8ec07c", "#83a598", "#b08ee0", "#d3869b")},
	{Name: "gruvbox-light", Light: true, Base: hex("#fbf1c7"), Surface: hex("#f2e5bc"), Overlay: hex("#d5c4a1"),
		Text: hex("#3c3836"), Bright: hex("#282828"), Muted: hex("#665c54"), Accent: hex("#b57614"),
		Human: hex("#946011"), Agent: hex("#076678"), Warn: hex("#af3a03"), Red: hex("#9d0006"),
		Green: hex("#79740e"), Purple: hex("#8f3f71"), Cyan: hex("#427b58"),
		Tags: tagSet("#9d0006", "#af3a03", "#b57614", "#79740e", "#427b58", "#076678", "#6f4fa8", "#8f3f71")},
	{Name: "dracula", Base: hex("#282a36"), Surface: hex("#343746"), Overlay: hex("#6272a4"),
		Text: hex("#f8f8f2"), Bright: hex("#ffffff"), Muted: hex("#bdc3d9"), Accent: hex("#ff79c6"),
		Human: hex("#f1fa8c"), Agent: hex("#8ea0f0"), Warn: hex("#ffb86c"), Red: hex("#ff5555"),
		Green: hex("#50fa7b"), Purple: hex("#bd93f9"), Cyan: hex("#8be9fd"),
		Tags: tagSet("#ff5555", "#ffb86c", "#f1fa8c", "#50fa7b", "#8be9fd", "#8ea0f0", "#bd93f9", "#ff79c6")},
	{Name: "nord", Base: hex("#2e3440"), Surface: hex("#3b4252"), Overlay: hex("#4c566a"),
		Text: hex("#e5e9f0"), Bright: hex("#eceff4"), Muted: hex("#c0c8d6"), Accent: hex("#88c0d0"),
		Human: hex("#ebcb8b"), Agent: hex("#81a1c1"), Warn: hex("#dba06f"), Red: hex("#c7757d"),
		Green: hex("#a3be8c"), Purple: hex("#b48ead"), Cyan: hex("#8fbcbb"),
		Tags: tagSet("#c7757d", "#dba06f", "#ebcb8b", "#a3be8c", "#8fbcbb", "#81a1c1", "#b48ead", "#e59ac0")},
	{Name: "solarized-dark", Base: hex("#002b36"), Surface: hex("#073642"), Overlay: hex("#586e75"),
		Text: hex("#eee8d5"), Bright: hex("#fdf6e3"), Muted: hex("#93a1a1"), Accent: hex("#b58900"),
		Human: hex("#dd3d8c"), Agent: hex("#268bd2"), Warn: hex("#d44e17"), Red: hex("#de3f3c"),
		Green: hex("#859900"), Purple: hex("#6e73c5"), Cyan: hex("#2aa198"),
		Tags: tagSet("#de3f3c", "#d44e17", "#b58900", "#859900", "#2aa198", "#268bd2", "#6e73c5", "#dd3d8c")},
	{Name: "kb-synth-sunset", Base: hex("#1f0f2a"), Surface: hex("#2c1840"), Overlay: hex("#553072"),
		Text: hex("#ffe8f3"), Bright: hex("#ffffff"), Muted: hex("#c4a3cf"), Accent: hex("#ff5fa2"),
		Human: hex("#ffd36e"), Agent: hex("#8fa8ff"), Warn: hex("#ff8f3c"), Red: hex("#ff3d5a"),
		Green: hex("#7cf5b4"), Purple: hex("#c792ff"), Cyan: hex("#5ce1e6"),
		Tags: tagSet("#ff3d5a", "#ff8f3c", "#ffd36e", "#7cf5b4", "#5ce1e6", "#8fa8ff", "#c792ff", "#ff5fa2")},
	{Name: "kb-chrome-haze", Light: true, Base: hex("#eef1f8"), Surface: hex("#e2e7f3"), Overlay: hex("#b8c2dc"),
		Text: hex("#1b1f3a"), Bright: hex("#0a0c1f"), Muted: hex("#4a5278"), Accent: hex("#c2008f"),
		Human: hex("#8a6d00"), Agent: hex("#0057d9"), Warn: hex("#c24a00"), Red: hex("#c8002e"),
		Green: hex("#00804a"), Purple: hex("#6b2fd6"), Cyan: hex("#007f99"),
		Tags: tagSet("#c8002e", "#c24a00", "#8a6d00", "#00804a", "#007f99", "#0057d9", "#6b2fd6", "#c2008f")},
	{Name: "kb-holo-dawn", Light: true, Base: hex("#f7f0fa"), Surface: hex("#efe3f5"), Overlay: hex("#cdb6dc"),
		Text: hex("#2a1838"), Bright: hex("#14081f"), Muted: hex("#5e4a70"), Accent: hex("#d1007a"),
		Human: hex("#8a5a00"), Agent: hex("#2b4fd6"), Warn: hex("#c43d00"), Red: hex("#b8002a"),
		Green: hex("#0a7d55"), Purple: hex("#7a2ad1"), Cyan: hex("#006f9e"),
		Tags: tagSet("#b8002a", "#c43d00", "#8a5a00", "#0a7d55", "#006f9e", "#2b4fd6", "#7a2ad1", "#d1007a")},
	// rose-pine (main): Base base, Surface surface, Overlay highlight med,
	// Text and Bright text, Muted subtle, Accent rose, Human gold, Agent pine,
	// Red love, Purple iris, Cyan foam. Warn and Green are derived.
	{Name: "rose-pine", Base: hex("#191724"), Surface: hex("#1f1d2e"), Overlay: hex("#403d52"),
		Text: hex("#e0def4"), Bright: hex("#e0def4"), Muted: hex("#908caa"), Accent: hex("#ebbcba"),
		Human: hex("#f6c177"), Agent: hex("#31748f"), Warn: hex("#f0956b"), Red: hex("#eb6f92"),
		Green: hex("#8fd0a8"), Purple: hex("#c4a7e7"), Cyan: hex("#9ccfd8"),
		Tags: tagSet("#eb6f92", "#f0956b", "#f6c177", "#8fd0a8", "#9ccfd8", "#31748f", "#c4a7e7", "#ebbcba")},
	// rose-pine-dawn: Base base, Surface surface, Overlay highlight med,
	// Text and Bright text, Muted subtle, Agent pine, Red love, Purple iris,
	// Cyan foam. Accent, Human, Warn and Green are derived.
	{Name: "rose-pine-dawn", Light: true, Base: hex("#faf4ed"), Surface: hex("#fffaf3"), Overlay: hex("#dfdad9"),
		Text: hex("#575279"), Bright: hex("#575279"), Muted: hex("#797593"), Accent: hex("#d16f6b"),
		Human: hex("#c27914"), Agent: hex("#286983"), Warn: hex("#d07125"), Red: hex("#b4637a"),
		Green: hex("#3a9a6e"), Purple: hex("#907aa9"), Cyan: hex("#56949f"),
		Tags: tagSet("#b4637a", "#d07125", "#a97f13", "#3a9a6e", "#56949f", "#286983", "#907aa9", "#d16f6b")},
	// solarized-light: Base base3, Surface base2, Overlay base1 lightened, Text base01,
	// Bright base03, Muted base00, Accent yellow, Human magenta, Agent blue,
	// Warn orange, Red red, Green green, Purple violet, Cyan cyan, as in
	// solarized-dark. Text, Accent, Agent, Green and Cyan are darkened shades.
	{Name: "solarized-light", Light: true, Base: hex("#fdf6e3"), Surface: hex("#eee8d5"), Overlay: hex("#bcc5c4"),
		Text: hex("#566b72"), Bright: hex("#002b36"), Muted: hex("#657b83"), Accent: hex("#a67d00"),
		Human: hex("#d33682"), Agent: hex("#2588ce"), Warn: hex("#cb4b16"), Red: hex("#dc322f"),
		Green: hex("#7a8c00"), Purple: hex("#6c71c4"), Cyan: hex("#269189"),
		Tags: tagSet("#dc322f", "#cb4b16", "#a67d00", "#7a8c00", "#269189", "#2588ce", "#6c71c4", "#d33682")},
	// tokyo-night-day (tokyonight_day.conf): Base background, Surface bg_dark,
	// Overlay selection (#b7c1e3), Text foreground, Bright fg dark (#343b58),
	// Muted fg_dark, Accent yellow, Human teal, Agent blue, Warn orange,
	// Red red, Green green, Purple magenta, Cyan cyan. Text, Human, Agent,
	// Red and Purple are darkened shades.
	{Name: "tokyo-night-day", Light: true, Base: hex("#e1e2e7"), Surface: hex("#d0d5e3"), Overlay: hex("#b7c1e3"),
		Text: hex("#3257ad"), Bright: hex("#343b58"), Muted: hex("#6172b0"), Accent: hex("#8c6c3e"),
		Human: hex("#10856e"), Agent: hex("#1e73e7"), Warn: hex("#b15c00"), Red: hex("#eb0b4c"),
		Green: hex("#587539"), Purple: hex("#954ff1"), Cyan: hex("#007197"),
		Tags: tagSet("#eb0b4c", "#b15c00", "#8c6c3e", "#587539", "#007197", "#1e73e7", "#954ff1", "#c9448c")},
	// dracula-alucard: Base background, Overlay current line, Text and Bright
	// foreground, Muted comment, Accent pink, Human yellow, Warn orange, Red
	// red, Green green, Purple purple, Cyan cyan. Surface and Agent are derived.
	{Name: "dracula-alucard", Light: true, Base: hex("#fffbeb"), Surface: hex("#f5efd6"), Overlay: hex("#cfcfde"),
		Text: hex("#1f1f1f"), Bright: hex("#1f1f1f"), Muted: hex("#6c664b"), Accent: hex("#a3144d"),
		Human: hex("#846e15"), Agent: hex("#3a5bc7"), Warn: hex("#a34d14"), Red: hex("#cb3a2a"),
		Green: hex("#14710a"), Purple: hex("#644ac9"), Cyan: hex("#036a96"),
		Tags: tagSet("#cb3a2a", "#a34d14", "#846e15", "#14710a", "#036a96", "#3a5bc7", "#644ac9", "#a3144d")},
	// nord-light (derived, see the list above): Base nord6, Surface nord5,
	// Overlay nord4, Text and Bright nord0, Muted nord3, Agent nord10, Red
	// nord11, as in the dark nord by meaning (Accent Frost nord8, Human
	// Aurora yellow, Warn orange, Green, Purple, Cyan Frost nord7).
	// Accent, Human, Warn, Green, Purple and Cyan are darkened shades.
	{Name: "nord-light", Light: true, Base: hex("#eceff4"), Surface: hex("#e5e9f0"), Overlay: hex("#d8dee9"),
		Text: hex("#2e3440"), Bright: hex("#2e3440"), Muted: hex("#4c566a"), Accent: hex("#418fa5"),
		Human: hex("#ad7d1e"), Agent: hex("#5e81ac"), Warn: hex("#ac5438"), Red: hex("#bf616a"),
		Green: hex("#6d8f51"), Purple: hex("#a5779d"), Cyan: hex("#568f8e"),
		Tags: tagSet("#bf616a", "#ac5438", "#ad7d1e", "#6d8f51", "#568f8e", "#5e81ac", "#a5779d", "#c0699a")},
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
