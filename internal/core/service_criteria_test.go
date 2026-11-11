package core_test

import (
	"testing"

	"github.com/CptPhenomeno/kb/internal/core"
)

func TestEditCriterionReplacesTextAndUnchecks(t *testing.T) {
	h := newHarness(t)
	task := h.add("@dev", core.TypeTask, "t", "", withCriteria("a", "b"))
	if _, err := h.svc.SetCriterion(ctx, "@dev", task.ID, 2, true, 0); err != nil {
		t.Fatal(err)
	}
	got, err := h.svc.EditCriterion(ctx, "@dev", task.ID, 2, "  better b  ", 0)
	if err != nil {
		t.Fatal(err)
	}
	if got.Acceptance[0].Text != "a" || got.Acceptance[1].Text != "better b" || got.Acceptance[1].Done {
		t.Fatalf("acceptance = %+v", got.Acceptance)
	}
}

func TestEditCriterionRecordsAcceptanceEvent(t *testing.T) {
	h := newHarness(t)
	task := h.add("@dev", core.TypeTask, "t", "", withCriteria("old text"))
	if _, err := h.svc.EditCriterion(ctx, "agent-1", task.ID, 1, "new text", 0); err != nil {
		t.Fatal(err)
	}
	ev := lastEvent(t, h, task.ID)
	if ev.Kind != core.EventAcceptance || ev.Actor != "agent-1" ||
		ev.Data["text"] != "new text" || ev.Data["done"] != false ||
		ev.Data["edited"] != true || ev.Data["old"] != "old text" {
		t.Fatalf("event = %+v", ev)
	}
}

func TestEditCheckedCriterionMovesDoneStoryBack(t *testing.T) {
	h := newHarness(t)
	story := h.add("@dev", core.TypeStory, "s", "", withCriteria("a", "b"))
	h.add("@dev", core.TypeTask, "t", story.ID, withStatus(core.StatusDone))
	for n := 1; n <= 2; n++ {
		if _, err := h.svc.SetCriterion(ctx, "@dev", story.ID, n, true, 0); err != nil {
			t.Fatal(err)
		}
	}
	assertStatus(t, h, story.ID, core.StatusDone)
	if _, err := h.svc.EditCriterion(ctx, "@dev", story.ID, 1, "a, reworded", 0); err != nil {
		t.Fatal(err)
	}
	assertStatus(t, h, story.ID, core.StatusReview)
}

func TestEditCriterionPropagatesToParents(t *testing.T) {
	h := newHarness(t)
	epic := h.add("@dev", core.TypeEpic, "e", "")
	story := h.add("@dev", core.TypeStory, "s", epic.ID, withCriteria("a"))
	h.add("@dev", core.TypeTask, "t", story.ID, withStatus(core.StatusDone))
	if _, err := h.svc.SetCriterion(ctx, "@dev", story.ID, 1, true, 0); err != nil {
		t.Fatal(err)
	}
	assertStatus(t, h, story.ID, core.StatusDone)
	assertStatus(t, h, epic.ID, core.StatusDone)
	if _, err := h.svc.EditCriterion(ctx, "@dev", story.ID, 1, "a, reworded", 0); err != nil {
		t.Fatal(err)
	}
	assertStatus(t, h, story.ID, core.StatusReview)
	if got := h.status(epic.ID); got == core.StatusDone {
		t.Fatalf("epic status = %s, want it moved back", got)
	}
}

func TestEditCriterionOutOfRangeIsNotFound(t *testing.T) {
	h := newHarness(t)
	task := h.add("@dev", core.TypeTask, "t", "", withCriteria("a"))
	for _, n := range []int{0, 2, -1} {
		_, err := h.svc.EditCriterion(ctx, "@dev", task.ID, n, "x", 0)
		assertErr(t, err, core.ErrNotFound)
	}
}

func TestEditCriterionBlankTextIsRuleViolation(t *testing.T) {
	h := newHarness(t)
	task := h.add("@dev", core.TypeTask, "t", "", withCriteria("a"))
	for _, text := range []string{"", "   \t"} {
		_, err := h.svc.EditCriterion(ctx, "@dev", task.ID, 1, text, 0)
		assertErr(t, err, core.ErrRule)
	}
	if got := h.get(task.ID).Acceptance[0].Text; got != "a" {
		t.Fatalf("text = %q, want it unchanged", got)
	}
}

func TestAddCriterionBlankTextIsRuleViolation(t *testing.T) {
	h := newHarness(t)
	task := h.add("@dev", core.TypeTask, "t", "")
	for _, text := range []string{"", "  "} {
		_, err := h.svc.AddCriterion(ctx, "@dev", task.ID, text, 0)
		assertErr(t, err, core.ErrRule)
	}
	if n := len(h.get(task.ID).Acceptance); n != 0 {
		t.Fatalf("%d criteria stored, want 0", n)
	}
}

func TestEditCriterionDuplicateTextIsRuleViolation(t *testing.T) {
	h := newHarness(t)
	task := h.add("@dev", core.TypeTask, "t", "", withCriteria("a", "b"))
	_, err := h.svc.EditCriterion(ctx, "@dev", task.ID, 2, "a", 0)
	assertErr(t, err, core.ErrRule)
}

func TestRemoveCriterionShiftsFollowingOnes(t *testing.T) {
	h := newHarness(t)
	task := h.add("@dev", core.TypeTask, "t", "", withCriteria("a", "b", "c"))
	if _, err := h.svc.SetCriterion(ctx, "@dev", task.ID, 3, true, 0); err != nil {
		t.Fatal(err)
	}
	got, err := h.svc.RemoveCriterion(ctx, "@dev", task.ID, 2, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Acceptance) != 2 || got.Acceptance[0].Text != "a" || got.Acceptance[0].Done ||
		got.Acceptance[1].Text != "c" || !got.Acceptance[1].Done {
		t.Fatalf("acceptance = %+v", got.Acceptance)
	}
}

func TestRemoveCriterionRecordsAcceptanceEvent(t *testing.T) {
	h := newHarness(t)
	task := h.add("@dev", core.TypeTask, "t", "", withCriteria("a", "gone"))
	if _, err := h.svc.SetCriterion(ctx, "@dev", task.ID, 2, true, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := h.svc.RemoveCriterion(ctx, "agent-1", task.ID, 2, 0); err != nil {
		t.Fatal(err)
	}
	ev := lastEvent(t, h, task.ID)
	if ev.Kind != core.EventAcceptance || ev.Actor != "agent-1" ||
		ev.Data["text"] != "gone" || ev.Data["done"] != true || ev.Data["removed"] != true {
		t.Fatalf("event = %+v", ev)
	}
}

func TestRemoveLastUncheckedCriterionMovesStoryToDone(t *testing.T) {
	h := newHarness(t)
	story := h.add("@dev", core.TypeStory, "s", "", withCriteria("a", "b"))
	h.add("@dev", core.TypeTask, "t", story.ID, withStatus(core.StatusDone))
	if _, err := h.svc.SetCriterion(ctx, "@dev", story.ID, 1, true, 0); err != nil {
		t.Fatal(err)
	}
	assertStatus(t, h, story.ID, core.StatusReview)
	if _, err := h.svc.RemoveCriterion(ctx, "@dev", story.ID, 2, 0); err != nil {
		t.Fatal(err)
	}
	assertStatus(t, h, story.ID, core.StatusDone)
}

func TestRemoveCriterionOutOfRangeIsNotFound(t *testing.T) {
	h := newHarness(t)
	task := h.add("@dev", core.TypeTask, "t", "", withCriteria("a"))
	for _, n := range []int{0, 2, -1} {
		_, err := h.svc.RemoveCriterion(ctx, "@dev", task.ID, n, 0)
		assertErr(t, err, core.ErrNotFound)
	}
	if n := len(h.get(task.ID).Acceptance); n != 1 {
		t.Fatalf("%d criteria stored, want 1", n)
	}
}
