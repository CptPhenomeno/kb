package core

import (
	"slices"
	"testing"
)

func move(id string, from, to Status) DerivedMove { return DerivedMove{ID: id, From: from, To: to} }

func assertMoves(t *testing.T, got []DerivedMove, want ...DerivedMove) {
	t.Helper()
	if len(got) == 0 && len(want) == 0 {
		return
	}
	if !slices.Equal(got, want) {
		t.Fatalf("moves = %v, want %v", got, want)
	}
}

func TestIndicatedStatus(t *testing.T) {
	cases := []struct {
		name     string
		children []Status
		want     Status
	}{
		{"all done", []Status{StatusDone, StatusDone}, StatusDone},
		{"review or done", []Status{StatusReview, StatusDone}, StatusReview},
		{"one started", []Status{StatusDoing, StatusTodo, StatusBacklog}, StatusDoing},
		{"one done among todo", []Status{StatusDone, StatusTodo}, StatusDoing},
		{"one todo", []Status{StatusTodo, StatusBacklog}, StatusTodo},
		{"all backlog", []Status{StatusBacklog}, StatusBacklog},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			kids := make([]*Item, len(c.children))
			for i, st := range c.children {
				kids[i] = &Item{Status: st}
			}
			if got := IndicatedStatus(kids); got != c.want {
				t.Fatalf("IndicatedStatus = %v, want %v", got, c.want)
			}
		})
	}
}

func TestAdvanceToDoingWhenChildStarts(t *testing.T) {
	s := board(items(
		it("S", TypeStory, StatusTodo, ""),
		it("A", TypeTask, StatusDoing, "S"),
		it("B", TypeTask, StatusTodo, "S"),
	))
	assertMoves(t, Propagate(s, "S"), move("S", StatusTodo, StatusDoing))
}

func TestAdvanceToReviewWhenAllChildrenReview(t *testing.T) {
	s := board(items(
		it("S", TypeStory, StatusDoing, ""),
		it("A", TypeTask, StatusReview, "S"),
		it("B", TypeTask, StatusDone, "S"),
	))
	assertMoves(t, Propagate(s, "S"), move("S", StatusDoing, StatusReview))
}

func TestStopsAtReviewWhenAcceptanceUnchecked(t *testing.T) {
	s := board(items(
		it("S", TypeStory, StatusDoing, ""),
		it("A", TypeTask, StatusDone, "S"),
	))
	s.Get("S").Acceptance = []Criterion{{Text: "x"}}
	assertMoves(t, Propagate(s, "S"), move("S", StatusDoing, StatusReview))
}

func TestAdvanceToDoneAfterLastAcceptanceChecked(t *testing.T) {
	s := board(items(
		it("S", TypeStory, StatusReview, ""),
		it("A", TypeTask, StatusDone, "S"),
	))
	s.Get("S").Acceptance = []Criterion{{Text: "x", Done: true}}
	assertMoves(t, Propagate(s, "S"), move("S", StatusReview, StatusDone))
}

func TestNeverMovesBackOnAdvanceRule(t *testing.T) {
	// The children indicate todo while the story is already in doing. The
	// advance rule never moves an item back, and doing's requirements still
	// hold, so nothing happens.
	s := board(items(
		it("S", TypeStory, StatusDoing, ""),
		it("A", TypeTask, StatusTodo, "S"),
	))
	assertMoves(t, Propagate(s, "S"))
}

func TestRegressFromReviewWhenChildReopened(t *testing.T) {
	s := board(items(
		it("S", TypeStory, StatusReview, ""),
		it("A", TypeTask, StatusDoing, "S"),
		it("B", TypeTask, StatusReview, "S"),
	))
	assertMoves(t, Propagate(s, "S"), move("S", StatusReview, StatusDoing))
}

func TestRegressFromDoneWhenNewChildAdded(t *testing.T) {
	s := board(items(
		it("S", TypeStory, StatusDone, ""),
		it("A", TypeTask, StatusDone, "S"),
		it("NEW", TypeTask, StatusBacklog, "S"),
	))
	assertMoves(t, Propagate(s, "S"), move("S", StatusDone, StatusDoing))
}

func TestNoValidStateStays(t *testing.T) {
	// The story is in review, a child is reopened, and the story is
	// blocked: doing is not enterable, todo and backlog are not valid with
	// a started child. With no valid previous state the story stays.
	s := board(items(
		it("X", TypeTask, StatusTodo, ""),
		it("S", TypeStory, StatusReview, ""),
		it("A", TypeTask, StatusDoing, "S"),
	), blocks("X", "S"))
	assertMoves(t, Propagate(s, "S"))
}

func TestBlockedParentStaysInDoing(t *testing.T) {
	// Requirements are checked on entry only: a blocks relation added to a
	// story already in doing does not move it back.
	s := board(items(
		it("X", TypeTask, StatusTodo, ""),
		it("S", TypeStory, StatusDoing, ""),
		it("A", TypeTask, StatusDoing, "S"),
	), blocks("X", "S"))
	assertMoves(t, Propagate(s, "S"))
}

func TestPropagatesUpThreeLevels(t *testing.T) {
	s := board(items(
		it("E", TypeEpic, StatusTodo, ""),
		it("S", TypeStory, StatusTodo, "E"),
		it("T", TypeTask, StatusTodo, "S"),
		it("ST", TypeSubtask, StatusDoing, "T"),
	))
	assertMoves(t, Propagate(s, "T"),
		move("T", StatusTodo, StatusDoing),
		move("S", StatusTodo, StatusDoing),
		move("E", StatusTodo, StatusDoing))
}

func TestStopsWhenParentUnchanged(t *testing.T) {
	s := board(items(
		it("E", TypeEpic, StatusBacklog, ""),
		it("S", TypeStory, StatusDoing, "E"),
		it("A", TypeTask, StatusDoing, "S"),
		it("B", TypeTask, StatusReview, "S"),
	))
	// S stays in doing, so E (inconsistent on purpose) is not re-evaluated.
	assertMoves(t, Propagate(s, "S"))
}

func TestManualMoveOfParentWithinRequirements(t *testing.T) {
	// A story moved by hand to doing while its tasks are still in todo
	// stays there: the children indicate todo, which is behind doing, and
	// doing's requirements hold.
	s := board(items(
		it("S", TypeStory, StatusDoing, ""),
		it("A", TypeTask, StatusTodo, "S"),
		it("B", TypeTask, StatusBacklog, "S"),
	))
	if unmet := CheckRequirements(s, s.Get("S"), StatusDoing); len(unmet) > 0 {
		t.Fatalf("manual move to doing should be allowed, unmet: %v", unmet)
	}
	assertMoves(t, Propagate(s, "S"))
}

func TestPropagateIsPure(t *testing.T) {
	s := board(items(
		it("S", TypeStory, StatusTodo, ""),
		it("A", TypeTask, StatusDoing, "S"),
	))
	Propagate(s, "S")
	if got := s.Get("S").Status; got != StatusTodo {
		t.Fatalf("Propagate mutated the snapshot: S is %v", got)
	}
}

func TestLeafWithoutChildrenNeverMoves(t *testing.T) {
	s := board(items(it("T", TypeTask, StatusReview, "")))
	s.Get("T").Tests = []Test{{Name: "a", State: TestFailing}}
	assertMoves(t, Propagate(s, "T"))
}
