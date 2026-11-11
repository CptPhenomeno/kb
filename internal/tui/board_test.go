package tui

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/CptPhenomeno/kb/internal/core"
)

// --- 6.1 application shell ---

func TestTabCyclesViews(t *testing.T) {
	m := newModel(t, sampleBoard(), 160, 48)
	want := []viewKind{viewTree, viewActivity, viewBoard}
	for _, v := range want {
		press(t, m, "tab")
		if m.view != v {
			t.Fatalf("after tab: view %v, want %v", m.view, v)
		}
	}
	press(t, m, "shift+tab")
	if m.view != viewActivity {
		t.Fatalf("shift+tab from board: view %v, want activity", m.view)
	}
}

func TestQuitKey(t *testing.T) {
	m := newModel(t, sampleBoard(), 160, 48)
	if !press(t, m, "q") {
		t.Fatal("q did not quit")
	}
	m = newModel(t, sampleBoard(), 160, 48)
	if !press(t, m, "ctrl+c") {
		t.Fatal("ctrl+c did not quit")
	}
}

func TestHelpToggle(t *testing.T) {
	m := newModel(t, sampleBoard(), 160, 48)
	press(t, m, "?")
	s := screen(m)
	if !strings.Contains(s, "move to the previous or next column") {
		t.Fatalf("help not shown:\n%s", s)
	}
	press(t, m, "esc")
	if strings.Contains(screen(m), "move to the previous or next column") {
		t.Fatal("esc did not close the help")
	}
	press(t, m, "?", "?")
	if strings.Contains(screen(m), "move to the previous or next column") {
		t.Fatal("? did not toggle the help off")
	}
}

func TestCommandBarShowsViewKeys(t *testing.T) {
	m := newModel(t, sampleBoard(), 160, 48)
	cases := []struct {
		view viewKind
		want []string
	}{
		{viewBoard, []string{"← → ↑ ↓ navigate", "shift ← → move", "shift ↑ ↓ reorder", "enter details", "/ search", "f filter", "t theme", "q quit"}},
		// Tree and Activity land in phase 8: only the global keys work there.
		{viewTree, []string{"tab next view", "q quit"}},
		{viewActivity, []string{"tab next view", "q quit"}},
	}
	for _, c := range cases {
		m.view = c.view
		bar := lastLines(screen(m), 2)
		for _, w := range c.want {
			if !strings.Contains(bar, w) {
				t.Errorf("%v command bar lacks %q:\n%s", c.view, w, bar)
			}
		}
	}
}

func lastLines(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

// --- 6.2 live refresh ---

func TestRefreshOnDataVersionChange(t *testing.T) {
	be := sampleBoard()
	m := newModel(t, be, 160, 48)
	if strings.Contains(screen(m), "Brand new task") {
		t.Fatal("fixture already has the new task")
	}
	be.snap = core.NewSnapshot(append(be.snap.Items(), item("KB-30", core.TypeTask, core.StatusTodo, "", "Brand new task", pos(9))), be.snap.Relations)
	be.events = append(be.events, core.Event{ID: 4, At: testNow, Actor: "agent-plan", Item: "KB-30", Kind: core.EventCreated,
		Data: map[string]any{"type": "task", "status": "todo"}})
	be.version++
	send(t, m, tickMsg{})
	s := screen(m)
	if !strings.Contains(s, "Brand new task") || !strings.Contains(s, "agent-plan") {
		t.Fatalf("change from another process not shown:\n%s", s)
	}
}

func TestSelectionPreservedAfterRefresh(t *testing.T) {
	be := sampleBoard()
	m := newModel(t, be, 160, 48)
	press(t, m, "right", "down") // To do, second card
	sel := m.selectedID()
	if sel == "" {
		t.Fatal("nothing selected")
	}
	// Another process puts a new card on top of the same column.
	be.snap = core.NewSnapshot(append(be.snap.Items(), item("KB-31", core.TypeTask, core.StatusTodo, "", "New top card", pos(-1))), be.snap.Relations)
	be.version++
	send(t, m, tickMsg{})
	if got := m.selectedID(); got != sel {
		t.Fatalf("selection moved from %s to %s after refresh", sel, got)
	}
}

func TestNoReloadWhenUnchanged(t *testing.T) {
	be := sampleBoard()
	m := newModel(t, be, 160, 48)
	calls := be.boardCalls
	send(t, m, tickMsg{})
	send(t, m, tickMsg{})
	if be.boardCalls != calls {
		t.Fatalf("board reloaded %d times without changes", be.boardCalls-calls)
	}
}

// --- 6.3 board rendering ---

func TestBoardSnapshot(t *testing.T) {
	golden(t, "board_160x48", screen(newModel(t, sampleBoard(), 160, 48)))
}

func TestBoardShowsEpicsAndStoriesHidesSubtasks(t *testing.T) {
	s := screen(newModel(t, sampleBoard(), 160, 48))
	for _, shown := range []string{"Authentication", "API keys per tenant", "Token login"} {
		if !strings.Contains(s, shown) {
			t.Errorf("board does not show %q", shown)
		}
	}
	if strings.Contains(s, "Bounded route label") {
		t.Error("board shows a subtask by default")
	}
}

func TestCardBlockedBadge(t *testing.T) {
	if s := screen(newModel(t, sampleBoard(), 160, 48)); !strings.Contains(s, "BLOCKED by KB-7") {
		t.Fatalf("blocked badge missing:\n%s", s)
	}
}

func TestCardStaleBadge(t *testing.T) {
	s := screen(newModel(t, sampleBoard(), 160, 48))
	if !strings.Contains(s, "STALE 22m") {
		t.Fatalf("stale badge missing:\n%s", s)
	}
	if !strings.Contains(s, "1 stale") {
		t.Fatalf("header stale count missing:\n%s", s)
	}
}

func TestColumnScroll(t *testing.T) {
	var items []*core.Item
	for i := range 20 {
		items = append(items, item(fmt.Sprintf("KB-%d", i+1), core.TypeTask, core.StatusTodo, "", fmt.Sprintf("Task number %d", i+1), pos(i)))
	}
	be := &fakeBackend{snap: core.NewSnapshot(items, nil), version: 1}
	m := newModel(t, be, 120, 30)
	s := screen(m)
	if !strings.Contains(s, "Task number 1") || strings.Contains(s, "Task number 20") || !strings.Contains(s, "more") {
		t.Fatalf("first screen should show the top and a 'more' marker:\n%s", s)
	}
	for range 19 {
		press(t, m, "down")
	}
	s = screen(m)
	if !strings.Contains(s, "Task number 20") || strings.Contains(s, "Task number 1\n") || strings.Contains(s, "Task number 1 ") {
		t.Fatalf("selected last card not scrolled into view:\n%s", s)
	}
}

func TestNarrowTerminalLayout(t *testing.T) {
	m := newModel(t, sampleBoard(), 100, 40)
	s := screen(m)
	for i, line := range strings.Split(s, "\n") {
		if w := displayWidth(line); w > 100 {
			t.Errorf("line %d is %d columns wide:\n%s", i, w, line)
		}
	}
	for _, col := range []string{"Backlog", "To do", "In progress", "Review", "Done"} {
		if !strings.Contains(s, col) {
			t.Errorf("column %q missing at 100 columns", col)
		}
	}
	tiny := newModel(t, sampleBoard(), 50, 12)
	if !strings.Contains(screen(tiny), "too small") {
		t.Errorf("tiny terminal should explain it is too small:\n%s", screen(tiny))
	}
}

// --- 6.4 navigation and detail ---

func TestArrowNavigation(t *testing.T) {
	m := newModel(t, sampleBoard(), 160, 48)
	if got := m.selectedID(); got != "KB-15" {
		t.Fatalf("initial selection = %s, want KB-15 (first card of the first column)", got)
	}
	steps := []struct{ key, want string }{
		{"right", "KB-5"}, {"down", "KB-6"}, {"down", "KB-7"}, {"down", "KB-8"}, {"down", "KB-8"},
		// Moving across columns keeps the row, clamped to the column length.
		{"right", "KB-12"}, {"up", "KB-4"}, {"up", "KB-2"}, {"left", "KB-6"},
	}
	for _, s := range steps {
		press(t, m, s.key)
		if got := m.selectedID(); got != s.want {
			t.Fatalf("after %s: selected %s, want %s", s.key, got, s.want)
		}
	}
}

func TestNavigationSkipsEmptyColumns(t *testing.T) {
	items := []*core.Item{
		item("KB-1", core.TypeTask, core.StatusBacklog, "", "a"),
		item("KB-2", core.TypeTask, core.StatusDone, "", "b"),
	}
	m := newModel(t, &fakeBackend{snap: core.NewSnapshot(items, nil), version: 1}, 160, 48)
	press(t, m, "right")
	if got := m.selectedID(); got != "KB-2" {
		t.Fatalf("right from backlog = %s, want KB-2 in done (skipping empty columns)", got)
	}
	press(t, m, "right")
	if got := m.selectedID(); got != "KB-2" {
		t.Fatalf("right from the last column = %s, want to stay on KB-2", got)
	}
}

func TestDetailOpensAndCloses(t *testing.T) {
	m := newModel(t, sampleBoard(), 160, 48)
	press(t, m, "right", "right", "down", "down", "enter") // KB-4 // KB-4
	s := screen(m)
	for _, want := range []string{"Token refresh", "history", "refresh rotates the token", "relates", "KB-5"} {
		if !strings.Contains(s, want) {
			t.Errorf("detail lacks %q", want)
		}
	}
	press(t, m, "esc")
	if strings.Contains(screen(m), "history") {
		t.Fatal("esc did not close the detail")
	}
}

func TestBoardDetailGoldenShowsTheCenteredPopup(t *testing.T) {
	m := newModel(t, sampleBoard(), 160, 48)
	press(t, m, "right", "right", "down", "down", "enter") // KB-4
	if !popupCorners(screen(m), 20, 6, 120, 36) {
		t.Fatal("the detail is not a centered popup")
	}
	golden(t, "board_detail_160x48", screen(m))
}

func TestMouseClickSelects(t *testing.T) {
	m := newModel(t, sampleBoard(), 160, 48)
	x, y := find(t, screen(m), "KB-17")
	send(t, m, tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft})
	if got := m.selectedID(); got != "KB-17" {
		t.Fatalf("click on KB-17 selected %s", got)
	}
	x, y = find(t, screen(m), "Key rotation endpoint")
	send(t, m, tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft})
	if got := m.selectedID(); got != "KB-8" {
		t.Fatalf("click on the title of KB-8 selected %s", got)
	}
	send(t, m, tea.MouseWheelMsg{X: x, Y: y, Button: tea.MouseWheelUp})
	if got := m.selectedID(); got != "KB-7" {
		t.Fatalf("wheel up selected %s, want KB-7", got)
	}
}

// find returns the screen cell where text starts.
func find(t *testing.T, s, text string) (int, int) {
	t.Helper()
	for y, line := range strings.Split(s, "\n") {
		if i := strings.Index(line, text); i >= 0 {
			return displayWidth(line[:i]), y
		}
	}
	t.Fatalf("%q not on screen", text)
	return 0, 0
}

var _ = time.Second

func TestBoardAndTreeShowNoStaleBadgeOnFinishedItems(t *testing.T) {
	be := sampleBoard()
	be.snap.Get("KB-17").Claim = &core.Claim{Actor: "agent-x", ExpiresAt: testNow.Add(-time.Hour)}
	s := screen(newModel(t, be, 160, 48))
	// KB-3 (review) and KB-17 (done) carry expired claims; only KB-12 (doing) is stale.
	if n := strings.Count(s, "STALE"); n != 1 {
		t.Fatalf("%d STALE badges, want only the one of KB-12 in progress:\n%s", n, s)
	}
	if !strings.Contains(strings.Split(s, "\n")[0], "1 stale") {
		t.Fatalf("header must count one stale item: %s", strings.Split(s, "\n")[0])
	}
}

// The Board starts with the newest change first in each column (KB-39);
// ties keep the position order.
func TestColumnsNewestFirst(t *testing.T) {
	be := sampleBoard()
	be.snap.Get("KB-8").UpdatedAt = testNow.Add(-time.Minute)
	be.snap.Get("KB-5").UpdatedAt = testNow.Add(-time.Hour)
	m := newModel(t, be, 160, 48)
	if got := boardColumn(m, core.StatusTodo); got != "KB-8 KB-5 KB-6 KB-7" {
		t.Fatalf("To do = %s, want the newest change first", got)
	}
	if !strings.Contains(m.header(), "sorted by date ↓") {
		t.Fatalf("header does not say the order: %s", m.header())
	}
}

// sortBoard is the sample board with distinct change dates and points.
func sortBoard() *fakeBackend {
	be := sampleBoard()
	// To do: KB-5 (1pt), KB-6 (story: 6pt below), KB-7 (3pt), KB-8 (3pt).
	be.snap.Get("KB-8").UpdatedAt = testNow.Add(-time.Minute)
	be.snap.Get("KB-5").UpdatedAt = testNow.Add(-time.Hour)
	be.snap.Get("KB-7").UpdatedAt = testNow.Add(-2 * time.Hour)
	return be
}

func sortModel(t *testing.T, be *fakeBackend, opts Options) *Model {
	t.Helper()
	opts.Project, opts.Actor, opts.Loc = "p", "@dev", time.UTC
	opts.Now = func() time.Time { return testNow }
	m := New(context.Background(), be, opts)
	m.tickEvery = 0
	send(t, m, windowSize(160, 48))
	runCmd(t, m, m.Init(), 0)
	return m
}

func TestSortPopupOpens(t *testing.T) {
	m := sortModel(t, sortBoard(), Options{})
	press(t, m, "s")
	if m.sort == nil {
		t.Fatal("s did not open the sort popup")
	}
	scr := screen(m)
	for _, want := range []string{"Sort columns", "newest change first", "oldest change first", "lowest ID first",
		"highest ID first", "most points first", "fewest points first"} {
		if !strings.Contains(scr, want) {
			t.Fatalf("sort popup lacks %q:\n%s", want, scr)
		}
	}
	// One choice: a single option is marked active, and no row carries a
	// direction of its own beside another field.
	active := 0
	for _, l := range strings.Split(scr, "\n") {
		if strings.Contains(l, " first") && strings.Contains(l, "●") {
			active++
		}
	}
	if n := active; n != 1 {
		t.Fatalf("%d options marked active, want 1:\n%s", n, scr)
	}
	if strings.Contains(scr, "descending") || strings.Contains(scr, "ascending") {
		t.Fatalf("the popup still shows a direction per row:\n%s", scr)
	}
	press(t, m, "esc")
	if m.sort != nil {
		t.Fatal("esc did not close the sort popup")
	}
}

func TestSortByEveryField(t *testing.T) {
	cases := []struct {
		keys []string
		want string
	}{
		{nil, "KB-8 KB-5 KB-7 KB-6"},                                    // default: date descending
		{[]string{"s", "enter"}, "KB-8 KB-5 KB-7 KB-6"},                 // the cursor starts on the active option
		{[]string{"s", "down", "enter"}, "KB-6 KB-7 KB-5 KB-8"},         // date ascending
		{[]string{"s", "down", "down", "enter"}, "KB-8 KB-7 KB-6 KB-5"}, // id descending
		{[]string{"s", "down", "down", "down", "enter"}, "KB-5 KB-6 KB-7 KB-8"},
		{[]string{"s", "up", "up", "enter"}, "KB-6 KB-7 KB-8 KB-5"}, // points descending, ties by position
		{[]string{"s", "up", "enter"}, "KB-5 KB-7 KB-8 KB-6"},       // points ascending
	}
	for _, c := range cases {
		m := sortModel(t, sortBoard(), Options{})
		press(t, m, c.keys...)
		if got := boardColumn(m, core.StatusTodo); got != c.want {
			t.Errorf("%v: To do = %s, want %s", c.keys, got, c.want)
		}
		if m.sort != nil {
			t.Errorf("%v: enter did not close the popup", c.keys)
		}
	}
}

func TestSortChoiceSavedAndRestored(t *testing.T) {
	var saved []string
	m := sortModel(t, sortBoard(), Options{SaveBoardSort: func(s string) error { saved = append(saved, s); return nil }})
	press(t, m, "s", "down", "down", "down", "enter")
	if strings.Join(saved, ",") != "id-asc" {
		t.Fatalf("saved = %v, want id-asc", saved)
	}
	if !strings.Contains(m.header(), "id ↑") {
		t.Fatalf("header does not show the sort: %s", m.header())
	}
	m2 := sortModel(t, sortBoard(), Options{BoardSort: "id-asc"})
	if got := boardColumn(m2, core.StatusTodo); got != "KB-5 KB-6 KB-7 KB-8" {
		t.Fatalf("BoardSort id-asc at start-up = %s", got)
	}
	m3 := sortModel(t, sortBoard(), Options{BoardSort: "manual"})
	if got := boardColumn(m3, core.StatusTodo); got != "KB-5 KB-6 KB-7 KB-8" || !strings.Contains(m3.header(), "manual order") {
		t.Fatalf("BoardSort manual at start-up = %s, header %s", got, m3.header())
	}
}

func boardColumn(m *Model, st core.Status) string {
	var ids []string
	for _, it := range m.board.columns(m)[st] {
		ids = append(ids, it.ID)
	}
	return strings.Join(ids, " ")
}
