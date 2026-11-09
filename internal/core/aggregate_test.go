package core

import "testing"

func TestProgressParent(t *testing.T) {
	s := board(items(
		it("S", TypeStory, StatusDoing, ""),
		it("A", TypeTask, StatusDone, "S"),
		it("B", TypeTask, StatusReview, "S"),
		it("C", TypeBug, StatusDone, "S"),
	))
	done, total := Progress(s, s.Get("S"))
	if done != 2 || total != 3 {
		t.Fatalf("Progress(S) = %d/%d, want 2/3", done, total)
	}
}

func TestProgressLeafTests(t *testing.T) {
	task := it("T", TypeTask, StatusDoing, "")
	task.Tests = []Test{{Name: "a", State: TestPassing}, {Name: "b", State: TestFailing}}
	s := board(items(task, it("ST", TypeSubtask, StatusDone, "T")))
	// Tasks report tests even when they have subtasks.
	done, total := Progress(s, task)
	if done != 1 || total != 2 {
		t.Fatalf("Progress(T) = %d/%d, want 1/2", done, total)
	}
}

func TestProgressEmpty(t *testing.T) {
	s := board(items(it("S", TypeStory, StatusTodo, ""), it("T", TypeTask, StatusTodo, "")))
	for _, id := range []string{"S", "T"} {
		if done, total := Progress(s, s.Get(id)); done != 0 || total != 0 {
			t.Errorf("Progress(%s) = %d/%d, want 0/0", id, done, total)
		}
	}
}

func TestPointsSumIgnoresEpicAndSubtask(t *testing.T) {
	e := it("E", TypeEpic, StatusTodo, "")
	st := it("S", TypeStory, StatusTodo, "E")
	t1 := it("T1", TypeTask, StatusTodo, "S")
	b1 := it("B1", TypeBug, StatusDone, "S")
	sub := it("ST", TypeSubtask, StatusTodo, "T1")
	st.Points, t1.Points, b1.Points = 8, 3, 2
	s := board(items(e, st, t1, b1, sub))
	// Work points are the points of tasks and bugs; story points are an
	// estimate of the same work and are not added twice.
	if got := PointsSum(s, "E"); got != 5 {
		t.Errorf("PointsSum(E) = %d, want 5", got)
	}
	if got := PointsSum(s, "S"); got != 5 {
		t.Errorf("PointsSum(S) = %d, want 5", got)
	}
	if got := PointsSum(s, "T1"); got != 0 {
		t.Errorf("PointsSum(T1) = %d, want 0 (subtasks have no points)", got)
	}
}
