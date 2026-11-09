package core

import "testing"

// it builds a test item with the fields that matter to most rules.
func it(id string, typ Type, status Status, parent string) *Item {
	return &Item{ID: id, Type: typ, Title: id, Status: status, Parent: parent}
}

// blocks builds a "from blocks to" relation.
func blocks(from, to string) Relation { return Relation{Kind: RelBlocks, From: from, To: to} }

// board builds a snapshot from items and relations.
func board(items []*Item, rels ...Relation) *Snapshot { return NewSnapshot(items, rels) }

// items is a shorthand for a slice literal of items.
func items(xs ...*Item) []*Item { return xs }

func TestSnapshotNavigation(t *testing.T) {
	s := board(items(
		it("E", TypeEpic, StatusDoing, ""),
		it("S", TypeStory, StatusDoing, "E"),
		it("T2", TypeTask, StatusTodo, "S"),
		it("T1", TypeTask, StatusDoing, "S"),
		it("ST", TypeSubtask, StatusDoing, "T1"),
	))
	s.Get("T2").Position = 2
	s.Get("T1").Position = 1

	if got := ids(s.Children("S")); !equal(got, []string{"T1", "T2"}) {
		t.Errorf("Children(S) = %v, want [T1 T2] (by position)", got)
	}
	if got := ids(s.Ancestors("ST")); !equal(got, []string{"T1", "S", "E"}) {
		t.Errorf("Ancestors(ST) = %v, want [T1 S E]", got)
	}
	if got := ids(s.Descendants("E")); !equal(got, []string{"S", "T1", "ST", "T2"}) {
		t.Errorf("Descendants(E) = %v, want [S T1 ST T2] (depth first)", got)
	}
	if s.Get("missing") != nil {
		t.Error("Get(missing) must be nil")
	}
}

func ids(xs []*Item) []string {
	out := make([]string, len(xs))
	for i, x := range xs {
		out[i] = x.ID
	}
	return out
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestCompareIDsNatural(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"KB-9", "KB-10", -1},
		{"KB-10", "KB-9", 1},
		{"KB-3", "KB-3", 0},
		{"AB-5", "KB-1", -1},
		{"T1", "T2", -1},
	}
	for _, c := range cases {
		if got := CompareIDs(c.a, c.b); got != c.want {
			t.Errorf("CompareIDs(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}
