package core

import (
	"testing"
	"time"
)

func TestNextOrderReviewBeforeDoingAndTodo(t *testing.T) {
	s := board(items(
		it("A", TypeTask, StatusTodo, ""),
		it("B", TypeTask, StatusDoing, ""),
		it("C", TypeTask, StatusReview, ""),
	))
	if got := availableIDs(s, NextFilter{}); !equal(got, []string{"C", "B", "A"}) {
		t.Fatalf("Available = %v, want [C B A]", got)
	}
}

func TestNextOrderStaleDoingBeforeTodo(t *testing.T) {
	s := board(items(
		it("A", TypeTask, StatusTodo, ""),
		claimed(it("B", TypeTask, StatusDoing, ""), "agent-1", now.Add(-time.Minute)),
	))
	if got := availableIDs(s, NextFilter{}); !equal(got, []string{"B", "A"}) {
		t.Fatalf("Available = %v, want [B A]", got)
	}
}

func TestNextOrderPriorityWithinSameStatus(t *testing.T) {
	// The low item has the best cbsp, points and position: only priority decides.
	lo := pts("A", TypeTask, StatusTodo, "", 13)
	lo.Priority = PriorityLow
	hi := it("B", TypeTask, StatusTodo, "")
	hi.Priority = PriorityHigh
	md := it("C", TypeTask, StatusTodo, "")
	s := board(items(lo, md, hi, pts("Z", TypeTask, StatusTodo, "", 13)), blocks("A", "Z"))
	s.Get("Z").Status = StatusBacklog
	if got := availableIDs(s, NextFilter{}); !equal(got, []string{"B", "C", "A"}) {
		t.Fatalf("Available = %v, want [B C A]", got)
	}
}

func TestNextOrderCBSPWinsOverOwnPoints(t *testing.T) {
	s := board(items(
		pts("A", TypeTask, StatusTodo, "", 13),
		pts("B", TypeTask, StatusTodo, "", 1),
		pts("X", TypeTask, StatusTodo, "", 5),
	), blocks("B", "X"))
	got := availableIDs(s, NextFilter{})
	if !equal(got, []string{"B", "A"}) {
		t.Fatalf("Available = %v, want [B A]", got)
	}
}

func TestNextOrderOwnPointsAtEqualCBSP(t *testing.T) {
	s := board(items(
		pts("A", TypeTask, StatusTodo, "", 2),
		pts("B", TypeTask, StatusTodo, "", 8),
	))
	s.Get("B").Position = 5 // worse position, more points
	if got := availableIDs(s, NextFilter{}); !equal(got, []string{"B", "A"}) {
		t.Fatalf("Available = %v, want [B A]", got)
	}
}

func TestNextOrderItemWithoutPointsAfterOnePoint(t *testing.T) {
	s := board(items(
		it("A", TypeTask, StatusTodo, ""), // no points, best position and id
		pts("B", TypeTask, StatusTodo, "", 1),
	))
	s.Get("B").Position = 9
	if got := availableIDs(s, NextFilter{}); !equal(got, []string{"B", "A"}) {
		t.Fatalf("Available = %v, want [B A]", got)
	}
}

func TestNextOrderPositionAtEqualPoints(t *testing.T) {
	s := board(items(
		pts("A", TypeTask, StatusTodo, "", 3),
		pts("B", TypeTask, StatusTodo, "", 3),
	))
	s.Get("A").Position = 2
	s.Get("B").Position = 1
	if got := availableIDs(s, NextFilter{}); !equal(got, []string{"B", "A"}) {
		t.Fatalf("Available = %v, want [B A]", got)
	}
}

func TestNextOrderNaturalIDBreaksLastTie(t *testing.T) {
	s := board(items(
		it("KB-10", TypeTask, StatusTodo, ""),
		it("KB-9", TypeTask, StatusTodo, ""),
	))
	if got := availableIDs(s, NextFilter{}); !equal(got, []string{"KB-9", "KB-10"}) {
		t.Fatalf("Available = %v, want [KB-9 KB-10]", got)
	}
}

func TestNextOrderSpecExample(t *testing.T) {
	s := board(items(
		it("KB-1", TypeStory, StatusTodo, ""),
		pts("KB-2", TypeTask, StatusTodo, "KB-1", 3),
		pts("KB-3", TypeTask, StatusTodo, "KB-1", 5),
		pts("KB-4", TypeTask, StatusTodo, "", 2),
		pts("KB-5", TypeTask, StatusTodo, "", 13),
		pts("KB-6", TypeTask, StatusDone, "", 13),
		it("KB-7", TypeStory, StatusTodo, ""),
		pts("KB-8", TypeTask, StatusTodo, "KB-7", 5),
	), blocks("KB-2", "KB-4"), blocks("KB-2", "KB-5"), blocks("KB-3", "KB-7"), blocks("KB-6", "KB-4"))
	if got := availableIDs(s, NextFilter{}); len(got) == 0 || got[0] != "KB-2" {
		t.Errorf("kb next = %v, want KB-2 first", got)
	}
	if got := availableIDs(s, NextFilter{Types: []Type{TypeStory}}); len(got) == 0 || got[0] != "KB-1" {
		t.Errorf("kb next --type story = %v, want KB-1 first", got)
	}
}
