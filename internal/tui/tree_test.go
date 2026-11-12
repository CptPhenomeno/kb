package tui

import (
	"github.com/CptPhenomeno/kb/internal/core"
	"regexp"
	"strings"
	"testing"
)

// treeModel opens the Tree view of the sample board.
func treeModel(t *testing.T) *Model {
	t.Helper()
	m := newModel(t, sampleBoard(), 160, 48)
	press(t, m, "tab")
	if m.view != viewTree {
		t.Fatalf("view = %v, want Tree", m.view)
	}
	return m
}

func treeIDs(m *Model) []string {
	var ids []string
	for _, r := range m.tree.rows(m) {
		ids = append(ids, r.it.ID)
	}
	return ids
}

func TestTreeSnapshot(t *testing.T) {
	m := treeModel(t)
	press(t, m, "down")
	golden(t, "tree", screen(m))
}

func TestTreeRowsFollowHierarchy(t *testing.T) {
	m := treeModel(t)
	got := strings.Join(treeIDs(m), " ")
	want := "KB-1 KB-2 KB-3 KB-4 KB-5 KB-6 KB-7 KB-8 KB-12 KB-13 KB-15 KB-17"
	if got != want {
		t.Fatalf("rows = %s, want %s", got, want)
	}
}

func TestCollapseExpand(t *testing.T) {
	m := treeModel(t)
	press(t, m, "left") // KB-1 collapses
	if got := strings.Join(treeIDs(m), " "); got != "KB-1 KB-6 KB-7 KB-8 KB-12 KB-13 KB-15 KB-17" {
		t.Fatalf("after collapse rows = %s", got)
	}
	if !strings.Contains(screen(m), "▸") {
		t.Fatal("a collapsed branch is not marked with ▸")
	}
	press(t, m, "right") // and expands again
	if len(treeIDs(m)) != 12 {
		t.Fatalf("after expand rows = %v", treeIDs(m))
	}
	// On a child, left jumps to the parent; right on an open branch enters it.
	press(t, m, "down", "down", "left")
	if id := m.selectedID(); id != "KB-2" {
		t.Fatalf("left on KB-3 selected %s, want its parent KB-2", id)
	}
	press(t, m, "right")
	if id := m.selectedID(); id != "KB-3" {
		t.Fatalf("right on open KB-2 selected %s, want its first child KB-3", id)
	}
}

func TestTreeKeepsSelectionAcrossReload(t *testing.T) {
	m := treeModel(t)
	press(t, m, "down", "down", "down") // KB-4
	be := m.be.(*fakeBackend)
	be.version++
	send(t, m, tickMsg{})
	runCmd(t, m, m.checkVersion(), 0)
	if id := m.selectedID(); id != "KB-4" {
		t.Fatalf("selection after reload = %s, want KB-4", id)
	}
}

func TestNewChildDefaultsType(t *testing.T) {
	cases := []struct {
		downs      int
		parent     string
		wantType   string
		wantNotice bool
	}{
		{0, "KB-1", "story", false}, // epic → story
		{1, "KB-2", "task", false},  // story → task
		{2, "KB-3", "subtask", false},
		{9, "KB-13", "", true}, // a subtask has no children
	}
	for _, c := range cases {
		m := treeModel(t)
		for range c.downs {
			press(t, m, "down")
		}
		press(t, m, "n")
		if c.wantNotice {
			if m.form != nil || m.notice == "" {
				t.Fatalf("n on %s: form=%v notice=%q, want a notice and no form", c.parent, m.form != nil, m.notice)
			}
			continue
		}
		if m.form == nil {
			t.Fatalf("n on %s opened no form", c.parent)
		}
		if p, ty := m.form.fields[fParent].value, m.form.fields[fType].value; p != c.parent || ty != c.wantType {
			t.Fatalf("n on %s: parent=%s type=%s, want %s %s", c.parent, p, ty, c.parent, c.wantType)
		}
	}
}

func TestTreeProgressBars(t *testing.T) {
	m := treeModel(t)
	scr := screen(m)
	for _, want := range []*regexp.Regexp{
		regexp.MustCompile(`KB-2 .*Token login.*In progress.*0/3 items`),
		regexp.MustCompile(`KB-3 .*Validate token.*Review.*█+.*3/3 tests.*3`),
		regexp.MustCompile(`KB-1 .*Authentication.*0/1 items.*6`), // points summed below the epic
		regexp.MustCompile(`KB-15 .*Load test.*—`),
	} {
		if !want.MatchString(scr) {
			t.Fatalf("tree misses %s:\n%s", want, scr)
		}
	}
}

func TestTreeActionsUseTreeSelection(t *testing.T) {
	m := treeModel(t)
	for range 9 {
		press(t, m, "down")
	}
	if id := m.selectedID(); id != "KB-13" {
		t.Fatalf("selected %s, want KB-13", id)
	}
	press(t, m, "enter")
	if !m.detailOpen || !strings.Contains(screen(m), "Bounded route label") {
		t.Fatalf("enter did not open the detail of the subtask:\n%s", screen(m))
	}
	press(t, m, "esc", "e")
	if m.form == nil || m.form.editID != "KB-13" {
		t.Fatal("e in the Tree view does not edit the selected row")
	}
}

func TestTreeSearchKeepsAncestors(t *testing.T) {
	m := treeModel(t)
	press(t, m, "/", "r", "e", "f", "r", "e", "s", "h", "enter")
	if got := strings.Join(treeIDs(m), " "); got != "KB-1 KB-2 KB-4" {
		t.Fatalf("search rows = %s, want the match and its ancestors", got)
	}
}

// doneBoard has a finished epic with a nested story, a finished task without
// children, and an epic still in progress.
func doneBoard() *fakeBackend {
	be := sampleBoard()
	items := append(be.snap.Items(),
		item("KB-20", core.TypeEpic, core.StatusDone, "", "Finished epic"),
		item("KB-21", core.TypeStory, core.StatusDone, "KB-20", "Finished story"),
		item("KB-22", core.TypeTask, core.StatusDone, "KB-21", "Finished task"),
		item("KB-23", core.TypeTask, core.StatusDone, "", "Finished loner"),
	)
	be.snap = core.NewSnapshot(items, nil)
	return be
}

func doneTreeModel(t *testing.T) *Model {
	t.Helper()
	m := newModel(t, doneBoard(), 160, 48)
	press(t, m, "tab")
	return m
}

// refreshTree makes the model poll and load the backend's current snapshot.
func refreshTree(t *testing.T, m *Model) {
	t.Helper()
	m.be.(*fakeBackend).version++
	send(t, m, tickMsg{})
	runCmd(t, m, m.checkVersion(), 0)
}

func rowIDs(m *Model) string { return strings.Join(treeIDs(m), " ") }

func TestTreeStartsWithDoneBranchesClosed(t *testing.T) {
	m := doneTreeModel(t)
	got := rowIDs(m)
	if !strings.Contains(got, "KB-20 ") || strings.Contains(got, "KB-21") || strings.Contains(got, "KB-22") {
		t.Fatalf("rows = %s, want KB-20 closed", got)
	}
	if !strings.Contains(got, "KB-1 KB-2 KB-3") {
		t.Fatalf("rows = %s, the open work must stay open", got)
	}
}

func TestTreeDoneItemsWithoutChildrenUnaffected(t *testing.T) {
	m := doneTreeModel(t)
	if !strings.Contains(rowIDs(m), "KB-23") || m.tree.collapsed["KB-23"] || m.tree.collapsed["KB-17"] {
		t.Fatalf("leaf done items must not be collapsed: %v", m.tree.collapsed)
	}
}

func TestTreeReopenedDoneBranchSurvivesRefresh(t *testing.T) {
	m := doneTreeModel(t)
	for i, r := range m.tree.rows(m) {
		if r.it.ID == "KB-20" {
			m.tree.cursor = i
		}
	}
	press(t, m, "right")
	if !strings.Contains(rowIDs(m), "KB-21") {
		t.Fatalf("right did not open KB-20: %s", rowIDs(m))
	}
	refreshTree(t, m)
	if !strings.Contains(rowIDs(m), "KB-21") {
		t.Fatalf("refresh closed the reopened branch: %s", rowIDs(m))
	}
}

func TestTreeItemDoneDuringSessionStaysOpen(t *testing.T) {
	m := doneTreeModel(t)
	be := m.be.(*fakeBackend)
	var items []*core.Item
	for _, it := range be.snap.Items() {
		c := *it
		if c.ID == "KB-6" || c.ID == "KB-7" || c.ID == "KB-8" {
			c.Status = core.StatusDone
		}
		items = append(items, &c)
	}
	be.snap = core.NewSnapshot(items, nil)
	refreshTree(t, m)
	if got := rowIDs(m); !strings.Contains(got, "KB-6 KB-7 KB-8") {
		t.Fatalf("an item done during the session was closed: %s", got)
	}
}

func TestTreeSearchRowsUnchangedByStartupCollapse(t *testing.T) {
	m := doneTreeModel(t)
	press(t, m, "/", "r", "e", "f", "r", "e", "s", "h", "enter")
	if got := rowIDs(m); got != "KB-1 KB-2 KB-4" {
		t.Fatalf("search rows = %s", got)
	}
	// A match inside a done branch shows, as it did before the startup collapse.
	m = doneTreeModel(t)
	press(t, m, "/", "F", "i", "n", "i", "s", "h", "e", "d", " ", "t", "a", "s", "k", "enter")
	if got := rowIDs(m); got != "KB-20 KB-21 KB-22" {
		t.Fatalf("search rows = %s, want the match under its open ancestors", got)
	}
}

func TestTreeDoneBranchesClosedSnapshot(t *testing.T) {
	m := doneTreeModel(t)
	golden(t, "tree_done_closed", screen(m))
}
