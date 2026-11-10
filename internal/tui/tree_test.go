package tui

import (
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
