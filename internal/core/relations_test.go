package core

import (
	"errors"
	"slices"
	"testing"
)

func TestBlocksCycleDirect(t *testing.T) {
	rels := []Relation{blocks("A", "B")}
	if !CreatesBlocksCycle(rels, blocks("B", "A")) {
		t.Fatal("B blocks A after A blocks B must be a cycle")
	}
}

func TestBlocksCycleTransitive(t *testing.T) {
	rels := []Relation{blocks("A", "B"), blocks("B", "C")}
	if !CreatesBlocksCycle(rels, blocks("C", "A")) {
		t.Fatal("C blocks A after A→B→C must be a cycle")
	}
}

func TestBlocksNoCycle(t *testing.T) {
	rels := []Relation{blocks("A", "B"), blocks("B", "C"), {Kind: RelRelates, From: "C", To: "A"}}
	if CreatesBlocksCycle(rels, blocks("A", "C")) {
		t.Fatal("A blocks C is not a cycle")
	}
	if CreatesBlocksCycle(rels, Relation{Kind: RelRelates, From: "C", To: "A"}) {
		t.Fatal("relates never creates a blocks cycle")
	}
}

func TestBlockedByOpenBlocker(t *testing.T) {
	s := board(items(
		it("A", TypeTask, StatusDoing, ""),
		it("B", TypeTask, StatusTodo, ""),
	), blocks("A", "B"))
	blocked, by := Blockers(s, "B")
	if !blocked || !slices.Equal(by, []string{"A"}) {
		t.Fatalf("Blockers(B) = %v, %v; want true, [A]", blocked, by)
	}
	if blocked, _ := Blockers(s, "A"); blocked {
		t.Fatal("A is not blocked")
	}
}

func TestNotBlockedWhenBlockerDone(t *testing.T) {
	s := board(items(
		it("A", TypeTask, StatusDone, ""),
		it("B", TypeTask, StatusTodo, ""),
	), blocks("A", "B"))
	if blocked, _ := Blockers(s, "B"); blocked {
		t.Fatal("B must not be blocked once A is done")
	}
}

func TestBlockedInheritedFromAncestor(t *testing.T) {
	s := board(items(
		it("X", TypeTask, StatusTodo, ""),
		it("S", TypeStory, StatusTodo, ""),
		it("T", TypeTask, StatusTodo, "S"),
		it("ST", TypeSubtask, StatusTodo, "T"),
	), blocks("X", "S"))
	for _, id := range []string{"S", "T", "ST"} {
		blocked, by := Blockers(s, id)
		if !blocked || !slices.Equal(by, []string{"X"}) {
			t.Errorf("Blockers(%s) = %v, %v; want true, [X]", id, blocked, by)
		}
	}
}

func TestRelatesSymmetricNoDuplicate(t *testing.T) {
	rels := []Relation{{Kind: RelRelates, From: "A", To: "B"}}
	if !HasRelation(rels, Relation{Kind: RelRelates, From: "B", To: "A"}) {
		t.Fatal("relates must be symmetric")
	}
	if HasRelation(rels, blocks("B", "A")) {
		t.Fatal("blocks is directed and distinct from relates")
	}
	dir := []Relation{{Kind: RelDuplicates, From: "A", To: "B"}}
	if HasRelation(dir, Relation{Kind: RelDuplicates, From: "B", To: "A"}) {
		t.Fatal("duplicates is directed")
	}
}

func TestSelfLinkRejected(t *testing.T) {
	s := board(items(it("A", TypeTask, StatusTodo, "")))
	for _, k := range []RelationKind{RelBlocks, RelRelates, RelDuplicates} {
		err := ValidateLink(s, Relation{Kind: k, From: "A", To: "A"})
		if !errors.Is(err, ErrRule) {
			t.Errorf("%s self link: err = %v, want ErrRule", k, err)
		}
	}
}

func TestValidateLink(t *testing.T) {
	s := board(items(
		it("A", TypeTask, StatusTodo, ""),
		it("B", TypeTask, StatusTodo, ""),
	), blocks("A", "B"))
	if err := ValidateLink(s, blocks("B", "A")); !errors.Is(err, ErrRule) {
		t.Errorf("cycle: err = %v, want ErrRule", err)
	}
	if err := ValidateLink(s, blocks("A", "Z")); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown item: err = %v, want ErrNotFound", err)
	}
	if err := ValidateLink(s, Relation{Kind: RelRelates, From: "B", To: "A"}); err != nil {
		t.Errorf("relates B-A: err = %v, want nil", err)
	}
}
