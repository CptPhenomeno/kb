package core

import (
	"slices"
	"testing"
)

func check(t *testing.T, s *Snapshot, id string, target Status, want ...string) {
	t.Helper()
	got := CheckRequirements(s, s.Get(id), target)
	if len(want) == 0 && len(got) == 0 {
		return
	}
	if !slices.Equal(got, want) {
		t.Fatalf("CheckRequirements(%s → %s) =\n  %q\nwant\n  %q", id, target, got, want)
	}
}

func TestReqBacklog(t *testing.T) {
	s := board(items(
		it("S", TypeStory, StatusTodo, ""),
		it("A", TypeTask, StatusBacklog, "S"),
		it("B", TypeTask, StatusBacklog, "S"),
	))
	check(t, s, "S", StatusBacklog)
	s.Get("B").Status = StatusTodo
	check(t, s, "S", StatusBacklog, "child B is todo")
}

func TestReqTodo(t *testing.T) {
	s := board(items(
		it("S", TypeStory, StatusBacklog, ""),
		it("A", TypeTask, StatusTodo, "S"),
	))
	check(t, s, "S", StatusTodo)
	s.Get("A").Status = StatusDoing
	check(t, s, "S", StatusTodo, "child A is doing")
}

func TestReqDoing(t *testing.T) {
	s := board(items(
		it("X", TypeTask, StatusDoing, ""),
		it("Y", TypeTask, StatusTodo, ""),
		it("S", TypeStory, StatusTodo, ""),
		it("T", TypeTask, StatusTodo, "S"),
	), blocks("X", "Y"), blocks("X", "S"))
	check(t, s, "Y", StatusDoing, "blocked by X")
	check(t, s, "T", StatusDoing, "blocked by X")
	s.Get("X").Status = StatusDone
	check(t, s, "Y", StatusDoing)
	check(t, s, "T", StatusDoing)
}

func TestReqReview(t *testing.T) {
	s := board(items(
		it("S", TypeStory, StatusDoing, ""),
		it("A", TypeTask, StatusReview, "S"),
		it("B", TypeTask, StatusDoing, "S"),
		it("L", TypeTask, StatusDoing, ""),
		it("E", TypeTask, StatusDoing, ""),
	))
	check(t, s, "S", StatusReview, "child B is doing")
	s.Get("B").Status = StatusDone
	check(t, s, "S", StatusReview)

	s.Get("L").Tests = []Test{{Name: "a", State: TestPassing}, {Name: "b", State: TestPending}, {Name: "c", State: TestFailing}}
	check(t, s, "L", StatusReview, `test 2 "b" is pending`, `test 3 "c" is failing`)

	// An item without tests or children can always enter review.
	check(t, s, "E", StatusReview)
}

func TestReqDone(t *testing.T) {
	s := board(items(
		it("S", TypeStory, StatusReview, ""),
		it("A", TypeTask, StatusReview, "S"),
	))
	s.Get("S").Acceptance = []Criterion{{Text: "x", Done: true}, {Text: "y"}}
	check(t, s, "S", StatusDone, "child A is review", "acceptance 1/2 checked")
	s.Get("A").Status = StatusDone
	s.Get("S").Acceptance[1].Done = true
	check(t, s, "S", StatusDone)
}

func TestReqListsAllUnmet(t *testing.T) {
	s := board(items(
		it("T", TypeTask, StatusReview, ""),
		it("ST", TypeSubtask, StatusDoing, "T"),
	))
	tk := s.Get("T")
	tk.Tests = []Test{{Name: "rotates token", State: TestFailing}}
	tk.Acceptance = []Criterion{{Text: "returns new pair"}}
	got := CheckRequirements(s, tk, StatusDone)
	if len(got) != 3 {
		t.Fatalf("got %d unmet requirements %q, want 3", len(got), got)
	}
}

// TestUnmetMessagesStable pins the exact wording of every message: agents
// parse it from --json output, so changing it is a breaking change.
func TestUnmetMessagesStable(t *testing.T) {
	s := board(items(
		it("KB-1", TypeTask, StatusDoing, ""),
		it("KB-2", TypeStory, StatusDoing, ""),
		it("KB-3", TypeTask, StatusReview, "KB-2"),
		it("KB-4", TypeTask, StatusDoing, "KB-2"),
		it("KB-5", TypeBug, StatusTodo, "KB-2"),
		it("KB-6", TypeTask, StatusTodo, ""),
	), blocks("KB-1", "KB-6"))
	s.Get("KB-2").Acceptance = []Criterion{{Text: "a"}, {Text: "b"}}
	s.Get("KB-4").Tests = []Test{{Name: "refresh rotates the token", State: TestFailing}}

	check(t, s, "KB-2", StatusDone,
		"child KB-3 is review",
		"child KB-4 is doing",
		"child KB-5 is todo",
		"acceptance 0/2 checked")
	check(t, s, "KB-4", StatusReview, `test 1 "refresh rotates the token" is failing`)
	check(t, s, "KB-6", StatusDoing, "blocked by KB-1")
}
