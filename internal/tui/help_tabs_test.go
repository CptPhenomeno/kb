package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

var wantHelpTabs = []string{"Board", "Detail", "Criteria", "Form"}

// activeHelpTab returns the name of the help tab drawn with the active tab
// style of the header, or "" when none is.
func activeHelpTab(m *Model) string {
	raw := m.renderHelp(m.bodyHeight())
	for _, n := range []string{"Board", "Tree", "Activity", "Detail", "Criteria", "Form"} {
		if strings.Contains(raw, m.th.tabOn.Render(" "+n+" ")) {
			return n
		}
	}
	return ""
}

func helpModel(t *testing.T) *Model {
	t.Helper()
	m := newModel(t, sampleBoard(), 160, 48)
	press(t, m, "?")
	return m
}

func TestHelpOpensOnTheTabOfTheCurrentView(t *testing.T) {
	for view, name := range map[string]string{"": "Board", "tab": "Tree", "tab,tab": "Activity"} {
		m := newModel(t, sampleBoard(), 160, 48)
		if view != "" {
			press(t, m, strings.Split(view, ",")...)
		}
		press(t, m, "?")
		if got := activeHelpTab(m); got != name {
			t.Errorf("view %q: active help tab %q, want %q", name, got, name)
		}
	}
}

func TestHelpTabBarListsTheViewDetailCriteriaAndForm(t *testing.T) {
	m := helpModel(t)
	lines := strings.Split(screen(m), "\n")
	bar := lines[1]
	last := -1
	for _, n := range wantHelpTabs {
		i := strings.Index(bar, " "+n+" ")
		if i < 0 || i < last {
			t.Fatalf("tab bar lacks %q in order:\n%s", n, bar)
		}
		last = i
	}
	if strings.Contains(screen(m), "Keys  (") {
		t.Errorf("the old Keys title is still there:\n%s", screen(m))
	}
}

func TestActiveHelpTabUsesTheHeaderTabStyle(t *testing.T) {
	m := helpModel(t)
	raw := m.renderHelp(m.bodyHeight())
	if !strings.Contains(raw, m.th.tabOn.Render(" Board ")) {
		t.Errorf("active tab does not use tabOn")
	}
	for _, n := range []string{"Detail", "Criteria", "Form"} {
		if !strings.Contains(raw, m.th.tabOff.Render(" "+n+" ")) {
			t.Errorf("inactive tab %s does not use tabOff", n)
		}
	}
}

func TestTabAndRightGoToTheNextHelpTab(t *testing.T) {
	for _, k := range []string{"tab", "right"} {
		m := helpModel(t)
		for _, want := range []string{"Detail", "Criteria", "Form"} {
			press(t, m, k)
			if got := activeHelpTab(m); got != want {
				t.Errorf("%s: active tab %q, want %q", k, got, want)
			}
		}
	}
}

func TestShiftTabAndLeftGoToThePreviousHelpTab(t *testing.T) {
	for _, k := range []string{"shift+tab", "left"} {
		m := helpModel(t)
		press(t, m, "tab", "tab", "tab")
		for _, want := range []string{"Criteria", "Detail", "Board"} {
			press(t, m, k)
			if got := activeHelpTab(m); got != want {
				t.Errorf("%s: active tab %q, want %q", k, got, want)
			}
		}
	}
}

func TestHelpTabsWrapAroundInBothDirections(t *testing.T) {
	m := helpModel(t)
	press(t, m, "shift+tab")
	if got := activeHelpTab(m); got != "Form" {
		t.Errorf("shift+tab from the first tab: %q, want Form", got)
	}
	press(t, m, "tab")
	if got := activeHelpTab(m); got != "Board" {
		t.Errorf("tab from the last tab: %q, want Board", got)
	}
	press(t, m, "left")
	if got := activeHelpTab(m); got != "Form" {
		t.Errorf("left from the first tab: %q, want Form", got)
	}
	press(t, m, "right")
	if got := activeHelpTab(m); got != "Board" {
		t.Errorf("right from the last tab: %q, want Board", got)
	}
}

func TestTabChangesTheHelpTabAndNotTheViewWhileTheHelpIsOpen(t *testing.T) {
	m := helpModel(t)
	press(t, m, "tab", "shift+tab", "shift+tab", "right", "left")
	if m.view != viewBoard {
		t.Errorf("view changed to %v", m.view)
	}
	press(t, m, "esc", "tab")
	if m.view != viewTree {
		t.Errorf("tab does not switch views once the help is closed: %v", m.view)
	}
}

func TestReopeningTheHelpStartsFromTheViewTab(t *testing.T) {
	m := helpModel(t)
	press(t, m, "tab", "tab", "esc", "?")
	if got := activeHelpTab(m); got != "Board" {
		t.Errorf("after esc: %q", got)
	}
	press(t, m, "tab", "?", "?")
	if got := activeHelpTab(m); got != "Board" {
		t.Errorf("after ?: %q", got)
	}
}

func TestPaletteHelpOpensTheHelpOnTheViewTab(t *testing.T) {
	m := helpModel(t)
	press(t, m, "tab", "esc", "ctrl+p")
	for _, r := range "Help" {
		press(t, m, string(r))
	}
	press(t, m, "enter")
	if !m.help {
		t.Fatal("the palette did not open the help")
	}
	if got := activeHelpTab(m); got != "Board" {
		t.Errorf("active tab %q, want Board", got)
	}
}

func TestOtherKeysDoNothingWhileTheHelpIsOpen(t *testing.T) {
	m := helpModel(t)
	for _, k := range []string{"t", "ctrl+p", "n", "e", "c", "enter", "/", "f", "s", "1", "up", "down", "shift+right"} {
		if press(t, m, k) {
			t.Fatalf("%s quit", k)
		}
		if !m.help || m.picker != nil || m.palette != nil || m.form != nil || m.accept != nil || m.filterOpen || m.sort != nil || m.detailOpen || m.board.searching {
			t.Fatalf("%s did something while the help is open", k)
		}
		if m.view != viewBoard {
			t.Fatalf("%s changed the view", k)
		}
		if got := activeHelpTab(m); got != "Board" {
			t.Fatalf("%s changed the help tab to %q", k, got)
		}
	}
}

func TestQDoesNotQuitWhileTheHelpIsOpen(t *testing.T) {
	m := helpModel(t)
	if press(t, m, "q") {
		t.Fatal("q quit with the help open")
	}
	if !m.help {
		t.Error("q closed the help")
	}
}

func TestCtrlCQuitsWithTheHelpOpen(t *testing.T) {
	m := helpModel(t)
	if !press(t, m, "ctrl+c") {
		t.Fatal("ctrl+c did not quit")
	}
}

func TestEscAndQuestionMarkCloseTheHelp(t *testing.T) {
	for _, k := range []string{"esc", "?"} {
		m := helpModel(t)
		press(t, m, "tab", k)
		if m.help {
			t.Errorf("%s did not close the help", k)
		}
	}
}

func TestDetailTabListsTheDetailPopupKeys(t *testing.T) {
	m := helpModel(t)
	press(t, m, "tab")
	s := screen(m)
	for _, h := range detailKeys {
		if !strings.Contains(s, h.key) || !strings.Contains(s, h.help) {
			t.Errorf("detail tab lacks %q %q:\n%s", h.key, h.help, s)
		}
	}
	if strings.Contains(s, "add a criterion") || strings.Contains(s, "save the form") {
		t.Errorf("detail tab shows other tabs' keys:\n%s", s)
	}
}

func TestCriteriaTabListsTheListInputAndConfirmationKeys(t *testing.T) {
	m := helpModel(t)
	press(t, m, "tab", "tab")
	s := screen(m)
	for _, hints := range [][]keyHint{acceptKeys, acceptInputKeys, acceptConfirmKeys} {
		for _, h := range hints {
			if !strings.Contains(s, h.help) {
				t.Errorf("criteria tab lacks %q:\n%s", h.help, s)
			}
		}
	}
}

// smallHelp opens the help on a terminal of 80 by 24.
func smallHelp(t *testing.T) *Model {
	t.Helper()
	m := newModel(t, sampleBoard(), 80, 24)
	press(t, m, "?")
	return m
}

func TestLongHelpDescriptionsWrapUnderTheirColumn(t *testing.T) {
	m := smallHelp(t)
	const col = 16
	wrapped, indented := 0, false
	for tab := 0; tab <= len(helpTabNames); tab++ {
		m.helpTab = tab
		_, _, hints := m.helpHints(tab)
		lines := m.helpLines(m.width)
		var plain []string
		for _, l := range lines {
			if w := displayWidth(l); w > m.width {
				t.Fatalf("tab %d: line %q is %d cells wide, over %d", tab, ansi.Strip(l), w, m.width)
			}
			p := ansi.Strip(l)
			plain = append(plain, p)
			if strings.HasPrefix(p, strings.Repeat(" ", col)) && strings.TrimSpace(p) != "" {
				indented = true
			}
		}
		rest := strings.Join(plain, "\n")
		for _, h := range hints {
			if len(h.help) > m.width-col {
				wrapped++
			}
			for _, w := range strings.Fields(h.help) {
				i := strings.Index(rest, w)
				if i < 0 {
					t.Fatalf("tab %d: word %q of %q is cut", tab, w, h.help)
				}
				rest = rest[i+len(w):]
			}
		}
	}
	if wrapped == 0 {
		t.Fatal("fixture: no description is wider than the body at 80 columns")
	}
	if !indented {
		t.Error("no continuation line is indented under the description column")
	}
}

func TestDownScrollsTheHelpTabByOneLine(t *testing.T) {
	m := smallHelp(t)
	press(t, m, "down")
	if m.helpScroll != 1 {
		t.Fatalf("helpScroll %d after down, want 1", m.helpScroll)
	}
	press(t, m, "down")
	if m.helpScroll != 2 {
		t.Fatalf("helpScroll %d after two downs, want 2", m.helpScroll)
	}
}

func TestUpScrollsTheHelpTabBackByOneLine(t *testing.T) {
	m := smallHelp(t)
	press(t, m, "down", "down", "down", "up")
	if m.helpScroll != 2 {
		t.Fatalf("helpScroll %d, want 2", m.helpScroll)
	}
	press(t, m, "up", "up", "up")
	if m.helpScroll != 0 {
		t.Fatalf("helpScroll %d above the top, want 0", m.helpScroll)
	}
}

func TestPgdownAndPgupScrollTheHelpTabByOnePage(t *testing.T) {
	m := newModel(t, sampleBoard(), 80, 20)
	press(t, m, "?")
	page := helpWindow(m.helpContentHeight())
	total := len(m.helpLines(m.width))
	if total < 2*page {
		t.Fatalf("fixture: %d lines, want at least two pages of %d", total, page)
	}
	press(t, m, "pgdown")
	if m.helpScroll != page {
		t.Fatalf("helpScroll %d after pgdown, want %d", m.helpScroll, page)
	}
	press(t, m, "pgup")
	if m.helpScroll != 0 {
		t.Fatalf("helpScroll %d after pgup, want 0", m.helpScroll)
	}
	for range 20 {
		press(t, m, "pgdown")
	}
	if m.helpScroll != total-page {
		t.Fatalf("helpScroll %d at the end, want %d", m.helpScroll, total-page)
	}
}

func TestMouseWheelScrollsTheHelpTab(t *testing.T) {
	m := smallHelp(t)
	send(t, m, tea.MouseWheelMsg{X: 10, Y: 10, Button: tea.MouseWheelDown})
	if m.helpScroll != 1 {
		t.Fatalf("helpScroll %d after wheel down, want 1", m.helpScroll)
	}
	send(t, m, tea.MouseWheelMsg{X: 10, Y: 10, Button: tea.MouseWheelUp})
	send(t, m, tea.MouseWheelMsg{X: 10, Y: 10, Button: tea.MouseWheelUp})
	if m.helpScroll != 0 {
		t.Fatalf("helpScroll %d after wheel up, want 0", m.helpScroll)
	}
}

func TestMarkersShowTheHelpLinesHiddenAboveAndBelow(t *testing.T) {
	m := smallHelp(t)
	if s := screen(m); !strings.Contains(s, "more lines · pgdown") || strings.Contains(s, "· pgup") {
		t.Fatalf("top of the help: want only the marker below:\n%s", s)
	}
	press(t, m, "down", "down")
	s := screen(m)
	if !strings.Contains(s, "↑ 2 more lines · pgup") || !strings.Contains(s, "↓") {
		t.Fatalf("scrolled help: want both markers, 2 lines above:\n%s", s)
	}
	for range 20 {
		press(t, m, "pgdown")
	}
	if s := screen(m); !strings.Contains(s, "· pgup") || strings.Contains(s, "· pgdown") {
		t.Fatalf("end of the help: want only the marker above:\n%s", s)
	}
}

func TestHelpTabThatFitsDoesNotScroll(t *testing.T) {
	m := helpModel(t)
	press(t, m, "down", "pgdown", "down")
	send(t, m, tea.MouseWheelMsg{X: 10, Y: 10, Button: tea.MouseWheelDown})
	if m.helpScroll != 0 {
		t.Fatalf("helpScroll %d on a tab that fits, want 0", m.helpScroll)
	}
	if s := screen(m); strings.Contains(s, "more lines") {
		t.Fatalf("a tab that fits shows a marker:\n%s", s)
	}
}

func TestEveryHelpTabReadsToItsLastLineOn80By24(t *testing.T) {
	m := smallHelp(t)
	for tab := 0; tab <= len(helpTabNames); tab++ {
		m.helpTab, m.helpScroll = tab, 0
		lines := m.helpLines(m.width)
		last := strings.TrimSpace(ansi.Strip(lines[len(lines)-1]))
		for range 40 {
			press(t, m, "down")
		}
		if s := screen(m); !strings.Contains(s, last) {
			t.Errorf("tab %d: last line %q not reachable:\n%s", tab, last, s)
		}
		if strings.Contains(screen(m), "· pgdown") {
			t.Errorf("tab %d: still lines below after scrolling to the end", tab)
		}
	}
}

func TestChangingHelpTabStartsFromTheTop(t *testing.T) {
	for _, k := range []string{"tab", "shift+tab", "right", "left"} {
		m := smallHelp(t)
		press(t, m, "down", "down", "down", k)
		if m.helpScroll != 0 {
			t.Errorf("%s: helpScroll %d after changing tab, want 0", k, m.helpScroll)
		}
	}
	m := smallHelp(t)
	press(t, m, "down", "esc", "?")
	if m.helpScroll != 0 {
		t.Errorf("reopening the help keeps the scroll %d", m.helpScroll)
	}
}

func TestClickOnAHelpTabOpensIt(t *testing.T) {
	m := smallHelp(t)
	press(t, m, "down")
	bar := ansi.Strip(strings.Split(screen(m), "\n")[1])
	for i, n := range []string{"Board", "Detail", "Criteria", "Form"} {
		x := strings.Index(bar, " "+n+" ") + 2
		send(t, m, tea.MouseClickMsg{X: x, Y: 1, Button: tea.MouseLeft})
		if m.helpTab != i {
			t.Errorf("click on %s: helpTab %d, want %d", n, m.helpTab, i)
		}
		if m.helpScroll != 0 {
			t.Errorf("click on %s: helpScroll %d, want 0", n, m.helpScroll)
		}
	}
}

func TestClickOutsideTheHelpTabBarLeavesTheHelpUnchanged(t *testing.T) {
	m := smallHelp(t)
	press(t, m, "tab", "down", "down")
	tab, scroll := m.helpTab, m.helpScroll
	for _, p := range [][2]int{{5, 0}, {5, 2}, {5, 10}, {79, 1}, {70, 1}, {5, 23}} {
		send(t, m, tea.MouseClickMsg{X: p[0], Y: p[1], Button: tea.MouseLeft})
		if !m.help || m.helpTab != tab || m.helpScroll != scroll {
			t.Fatalf("click at %v changed the help: open %v tab %d scroll %d", p, m.help, m.helpTab, m.helpScroll)
		}
	}
}

func TestDetailPopupMarkersAreUnchangedAfterTheSharedScrollHelper(t *testing.T) {
	m := longDetail(t)
	w, h := m.detailSize()
	iw := w - 4
	total := len(m.detailLines(iw))
	win := detailWindow(h - 2)
	if win != h-4 {
		t.Fatalf("detail window %d, want %d", win, h-4)
	}
	s := screen(m)
	if !strings.Contains(s, fmt.Sprintf("↓ %d more lines · pgdown", total-win)) || strings.Contains(s, "· pgup") {
		t.Fatalf("top of the detail:\n%s", s)
	}
	press(t, m, "down", "down", "down")
	s = screen(m)
	if !strings.Contains(s, "↑ 3 more lines · pgup") || !strings.Contains(s, fmt.Sprintf("↓ %d more lines · pgdown", total-win-3)) {
		t.Fatalf("scrolled detail:\n%s", s)
	}
	for range 30 {
		press(t, m, "pgdown")
	}
	if m.detailScroll != total-win {
		t.Fatalf("detailScroll %d at the end, want %d", m.detailScroll, total-win)
	}
	if s := screen(m); strings.Contains(s, "· pgdown") || !strings.Contains(s, fmt.Sprintf("↑ %d more lines · pgup", total-win)) {
		t.Fatalf("end of the detail:\n%s", s)
	}
}
