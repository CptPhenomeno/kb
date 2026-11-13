package core

import "testing"

// pts builds a task-like item with story points.
func pts(id string, typ Type, status Status, parent string, points int) *Item {
	x := it(id, typ, status, parent)
	x.Points = points
	return x
}

func TestWorkIndexBlockedSetAgreesWithBlockers(t *testing.T) {
	s := board(items(
		it("E", TypeEpic, StatusTodo, ""),
		it("S1", TypeStory, StatusTodo, "E"),
		it("S2", TypeStory, StatusTodo, "E"),
		it("T1", TypeTask, StatusTodo, "S1"),
		it("T2", TypeTask, StatusTodo, "S1"),
		it("T3", TypeTask, StatusDone, "S2"),
		it("T4", TypeTask, StatusTodo, "S2"),
		it("ST", TypeSubtask, StatusTodo, "T4"),
		it("B", TypeBug, StatusTodo, ""),
	),
		blocks("T1", "T2"), blocks("T3", "T4"), blocks("T1", "S2"),
		blocks("B", "E"), blocks("T2", "B"), blocks("ghost", "T1"),
		Relation{Kind: RelRelates, From: "T4", To: "T1"},
	)
	w := newWorkIndex(s)
	for _, x := range s.Items() {
		want, _ := Blockers(s, x.ID)
		if got := w.isBlocked(x.ID); got != want {
			t.Errorf("isBlocked(%s) = %v, Blockers says %v", x.ID, got, want)
		}
	}
}

func TestWorkIndexAncestorTargetBlocks(t *testing.T) {
	s := board(items(
		it("S", TypeStory, StatusTodo, ""),
		it("T", TypeTask, StatusTodo, "S"),
		it("ST", TypeSubtask, StatusTodo, "T"),
		it("X", TypeTask, StatusTodo, ""),
	), blocks("X", "S"))
	w := newWorkIndex(s)
	for _, id := range []string{"S", "T", "ST"} {
		if !w.isBlocked(id) {
			t.Errorf("%s must be blocked through S", id)
		}
	}
	if w.isBlocked("X") {
		t.Error("X blocks but is not blocked")
	}
}

func TestWorkIndexDoneSourceLeavesTargetUnblocked(t *testing.T) {
	s := board(items(
		it("A", TypeTask, StatusDone, ""),
		it("B", TypeTask, StatusTodo, ""),
	), blocks("A", "B"))
	w := newWorkIndex(s)
	if w.isBlocked("B") {
		t.Error("B must not be blocked by a done source")
	}
	if got := w.cbsp("A"); got != 0 {
		t.Errorf("cbsp(done A) = %d, want 0", got)
	}
}

func TestCBSPNoBlocksIsZero(t *testing.T) {
	w := newWorkIndex(board(items(pts("A", TypeTask, StatusTodo, "", 5))))
	if got := w.cbsp("A"); got != 0 {
		t.Errorf("cbsp = %d, want 0", got)
	}
}

func TestCBSPChainSumsPoints(t *testing.T) {
	w := newWorkIndex(board(items(
		pts("A", TypeTask, StatusTodo, "", 8),
		pts("B", TypeTask, StatusTodo, "", 3),
		pts("C", TypeTask, StatusTodo, "", 5),
	), blocks("A", "B"), blocks("B", "C")))
	if got := w.cbsp("A"); got != 8 {
		t.Errorf("cbsp(A) = %d, want 3+5 = 8 (own points excluded)", got)
	}
	if got := w.cbsp("B"); got != 5 {
		t.Errorf("cbsp(B) = %d, want 5", got)
	}
}

func TestCBSPForkTakesHeavierBranch(t *testing.T) {
	w := newWorkIndex(board(items(
		pts("A", TypeTask, StatusTodo, "", 1),
		pts("B", TypeTask, StatusTodo, "", 2),
		pts("C", TypeTask, StatusTodo, "", 3),
		pts("D", TypeTask, StatusTodo, "", 8),
	), blocks("A", "B"), blocks("A", "C"), blocks("C", "D")))
	if got := w.cbsp("A"); got != 11 {
		t.Errorf("cbsp(A) = %d, want max(2, 3+8) = 11", got)
	}
}

func TestCBSPDiamondCountsSharedNodeOnce(t *testing.T) {
	w := newWorkIndex(board(items(
		pts("A", TypeTask, StatusTodo, "", 1),
		pts("B", TypeTask, StatusTodo, "", 2),
		pts("C", TypeTask, StatusTodo, "", 2),
		pts("D", TypeTask, StatusTodo, "", 8),
	), blocks("A", "B"), blocks("A", "C"), blocks("B", "D"), blocks("C", "D")))
	if got := w.cbsp("A"); got != 10 {
		t.Errorf("cbsp(A) = %d, want 2+8 = 10, not 20", got)
	}
}

func TestCBSPStoryInheritsTaskBlocks(t *testing.T) {
	w := newWorkIndex(board(items(
		it("S", TypeStory, StatusTodo, ""),
		pts("T", TypeTask, StatusTodo, "S", 2),
		pts("Y", TypeTask, StatusTodo, "", 5),
	), blocks("T", "Y")))
	if got := w.cbsp("S"); got != 5 {
		t.Errorf("cbsp(S) = %d, want 5", got)
	}
}

func TestCBSPEpicInheritsStoryTaskBlocks(t *testing.T) {
	w := newWorkIndex(board(items(
		it("E", TypeEpic, StatusTodo, ""),
		it("S", TypeStory, StatusTodo, "E"),
		pts("T", TypeTask, StatusTodo, "S", 2),
		pts("Y", TypeBug, StatusTodo, "", 13),
	), blocks("T", "Y")))
	if got := w.cbsp("E"); got != 13 {
		t.Errorf("cbsp(E) = %d, want 13", got)
	}
}

func TestCBSPSubtaskBlocksCountUpTheTree(t *testing.T) {
	w := newWorkIndex(board(items(
		it("E", TypeEpic, StatusTodo, ""),
		it("S", TypeStory, StatusTodo, "E"),
		pts("T", TypeTask, StatusTodo, "S", 2),
		it("ST", TypeSubtask, StatusTodo, "T"),
		pts("Y", TypeTask, StatusTodo, "", 3),
	), blocks("ST", "Y")))
	for _, id := range []string{"ST", "T", "S", "E"} {
		if got := w.cbsp(id); got != 3 {
			t.Errorf("cbsp(%s) = %d, want 3", id, got)
		}
	}
}

func TestCBSPDoneDescendantDoesNotPassOnBlocks(t *testing.T) {
	w := newWorkIndex(board(items(
		it("S", TypeStory, StatusTodo, ""),
		pts("T", TypeTask, StatusDone, "S", 2),
		pts("Y", TypeTask, StatusTodo, "", 3),
	), blocks("T", "Y")))
	if got := w.cbsp("S"); got != 0 {
		t.Errorf("cbsp(S) = %d, want 0", got)
	}
}

func TestCBSPStoryTargetWeighsNonDoneTasks(t *testing.T) {
	w := newWorkIndex(board(items(
		pts("X", TypeTask, StatusTodo, "", 1),
		it("S", TypeStory, StatusTodo, ""),
		pts("T1", TypeTask, StatusTodo, "S", 3),
		pts("T2", TypeBug, StatusDoing, "S", 5),
		pts("T3", TypeTask, StatusDone, "S", 13),
		pts("ST", TypeSubtask, StatusTodo, "T1", 8),
	), blocks("X", "S")))
	if got := w.storyPoints("S"); got != 8 {
		t.Errorf("storyPoints(S) = %d, want 3+5 = 8", got)
	}
	if got := w.cbsp("X"); got != 8 {
		t.Errorf("cbsp(X) = %d, want 8", got)
	}
}

func TestWorkIndexSubtreeMatchesDescendants(t *testing.T) {
	s := board(items(
		it("E", TypeEpic, StatusTodo, ""),
		it("S1", TypeStory, StatusTodo, "E"),
		it("S2", TypeStory, StatusTodo, "E"),
		it("T1", TypeTask, StatusTodo, "S1"),
		it("T2", TypeTask, StatusTodo, "S2"),
		it("ST", TypeSubtask, StatusTodo, "T2"),
		it("O", TypeTask, StatusTodo, ""),
	))
	w := newWorkIndex(s)
	for _, root := range []string{"E", "S1", "S2", "T2", "O", "T1"} {
		want := map[string]bool{}
		for _, d := range s.Descendants(root) {
			want[d.ID] = true
		}
		got := w.subtree(root)
		if len(got) != len(want) {
			t.Errorf("subtree(%s) = %v, want %d items", root, got, len(want))
		}
		for _, id := range got {
			if !want[id] {
				t.Errorf("subtree(%s) has unexpected %s", root, id)
			}
		}
	}
	// Available with --parent agrees at depth.
	got := availableIDs(s, NextFilter{Parent: "E"})
	if !equal(got, []string{"T1", "T2"}) {
		t.Errorf("Available(parent E) = %v, want [T1 T2]", got)
	}
}

func TestCBSPEpicTargetWeighsTasksAndBugsOfAllStories(t *testing.T) {
	w := newWorkIndex(board(items(
		pts("X", TypeTask, StatusTodo, "", 1),
		it("E", TypeEpic, StatusTodo, ""),
		it("S1", TypeStory, StatusTodo, "E"),
		it("S2", TypeStory, StatusTodo, "E"),
		pts("T1", TypeTask, StatusTodo, "S1", 2),
		pts("B1", TypeBug, StatusTodo, "S2", 3),
		pts("T2", TypeTask, StatusDone, "S2", 13),
	), blocks("X", "E")))
	if got := w.storyPoints("E"); got != 5 {
		t.Errorf("storyPoints(E) = %d, want 5", got)
	}
	if got := w.cbsp("X"); got != 5 {
		t.Errorf("cbsp(X) = %d, want 5", got)
	}
}

func TestCBSPDoneSourceHasEmptyBlockedSet(t *testing.T) {
	w := newWorkIndex(board(items(
		it("S", TypeStory, StatusDone, ""),
		pts("T", TypeTask, StatusTodo, "S", 2),
		pts("Y", TypeTask, StatusTodo, "", 8),
	), blocks("T", "Y")))
	if got := w.cbsp("S"); got != 0 {
		t.Errorf("cbsp(done S) = %d, want 0", got)
	}
}

func TestCBSPDoneTargetIgnored(t *testing.T) {
	w := newWorkIndex(board(items(
		pts("X", TypeTask, StatusTodo, "", 1),
		pts("Y", TypeTask, StatusDone, "", 13),
	), blocks("X", "Y")))
	if got := w.cbsp("X"); got != 0 {
		t.Errorf("cbsp(X) = %d, want 0", got)
	}
}

func TestCBSPRelatesIgnored(t *testing.T) {
	w := newWorkIndex(board(items(
		pts("X", TypeTask, StatusTodo, "", 1),
		pts("Y", TypeTask, StatusTodo, "", 13),
		pts("Z", TypeTask, StatusTodo, "", 8),
	),
		Relation{Kind: RelRelates, From: "X", To: "Y"},
		Relation{Kind: RelDuplicates, From: "X", To: "Z"}))
	if got := w.cbsp("X"); got != 0 {
		t.Errorf("cbsp(X) = %d, want 0", got)
	}
}

func TestCBSPItemsWithoutPointsWeighZero(t *testing.T) {
	w := newWorkIndex(board(items(
		pts("X", TypeTask, StatusTodo, "", 1),
		it("Y", TypeTask, StatusTodo, ""),
		pts("Z", TypeTask, StatusTodo, "", 3),
	), blocks("X", "Y"), blocks("Y", "Z")))
	if got := w.cbsp("X"); got != 3 {
		t.Errorf("cbsp(X) = %d, want 0+3 = 3", got)
	}
}

func TestCBSPSpecExample(t *testing.T) {
	w := newWorkIndex(board(items(
		it("KB-1", TypeStory, StatusTodo, ""),
		pts("KB-2", TypeTask, StatusTodo, "KB-1", 3),
		pts("KB-3", TypeTask, StatusTodo, "KB-1", 5),
		pts("KB-4", TypeTask, StatusTodo, "", 2),
		pts("KB-5", TypeTask, StatusTodo, "", 13),
		pts("KB-6", TypeTask, StatusDone, "", 13),
		it("KB-7", TypeStory, StatusTodo, ""),
		pts("KB-8", TypeTask, StatusTodo, "KB-7", 5),
	), blocks("KB-2", "KB-4"), blocks("KB-2", "KB-5"), blocks("KB-3", "KB-7"), blocks("KB-6", "KB-4")))
	for id, want := range map[string]int{
		"KB-1": 13, "KB-2": 13, "KB-3": 5, "KB-4": 0, "KB-5": 0, "KB-7": 0, "KB-8": 0,
	} {
		if got := w.cbsp(id); got != want {
			t.Errorf("cbsp(%s) = %d, want %d", id, got, want)
		}
	}
}
