package core_test

import (
	"testing"

	"github.com/CptPhenomeno/kb/internal/core"
)

func testNames(it *core.Item) []string {
	var out []string
	for _, t := range it.Tests {
		out = append(out, t.Name)
	}
	return out
}

func passAll(t *testing.T, h *harness, id string, sels ...string) {
	t.Helper()
	for _, s := range sels {
		if _, err := h.svc.SetTestState(ctx, "@dev", id, s, core.TestPassing, 0); err != nil {
			t.Fatal(err)
		}
	}
}

func TestEditTestRenamesByPositionAndSetsPending(t *testing.T) {
	h := newHarness(t)
	task := h.add("@dev", core.TypeTask, "t", "", withTests("a", "b"))
	passAll(t, h, task.ID, "2")
	got, err := h.svc.EditTest(ctx, "@dev", task.ID, "2", "  better b  ", 0)
	if err != nil {
		t.Fatal(err)
	}
	if got.Tests[0].Name != "a" || got.Tests[1].Name != "better b" || got.Tests[1].State != core.TestPending {
		t.Fatalf("tests = %+v", got.Tests)
	}
}

func TestEditTestRenamesByName(t *testing.T) {
	h := newHarness(t)
	task := h.add("@dev", core.TypeTask, "t", "", withTests("a", "b"))
	got, err := h.svc.EditTest(ctx, "@dev", task.ID, "a", "z", 0)
	if err != nil {
		t.Fatal(err)
	}
	if n := testNames(got); n[0] != "z" || n[1] != "b" {
		t.Fatalf("tests = %v", n)
	}
}

func TestEditTestEmptyOrDuplicateNameIsRuleError(t *testing.T) {
	h := newHarness(t)
	task := h.add("@dev", core.TypeTask, "t", "", withTests("a", "b"))
	for _, name := range []string{"", "   ", "b", " b "} {
		_, err := h.svc.EditTest(ctx, "@dev", task.ID, "1", name, 0)
		assertErr(t, err, core.ErrRule)
	}
}

func TestEditTestKeepingOwnNameIsAllowed(t *testing.T) {
	h := newHarness(t)
	task := h.add("@dev", core.TypeTask, "t", "", withTests("a"))
	passAll(t, h, task.ID, "1")
	got, err := h.svc.EditTest(ctx, "@dev", task.ID, "1", "a", 0)
	if err != nil {
		t.Fatal(err)
	}
	if got.Tests[0].State != core.TestPending {
		t.Fatalf("tests = %+v", got.Tests)
	}
}

func TestRemoveTestByPositionShiftsLaterOnesUp(t *testing.T) {
	h := newHarness(t)
	task := h.add("@dev", core.TypeTask, "t", "", withTests("a", "b", "c"))
	got, err := h.svc.RemoveTest(ctx, "@dev", task.ID, "2", 0)
	if err != nil {
		t.Fatal(err)
	}
	if n := testNames(got); len(n) != 2 || n[0] != "a" || n[1] != "c" {
		t.Fatalf("tests = %v", n)
	}
}

func TestRemoveTestByName(t *testing.T) {
	h := newHarness(t)
	task := h.add("@dev", core.TypeTask, "t", "", withTests("a", "b", "c"))
	got, err := h.svc.RemoveTest(ctx, "@dev", task.ID, "a", 0)
	if err != nil {
		t.Fatal(err)
	}
	if n := testNames(got); len(n) != 2 || n[0] != "b" || n[1] != "c" {
		t.Fatalf("tests = %v", n)
	}
}

func TestEditOrRemoveMissingTestIsNotFound(t *testing.T) {
	h := newHarness(t)
	task := h.add("@dev", core.TypeTask, "t", "", withTests("a"))
	for _, sel := range []string{"0", "2", "-1", "nope"} {
		_, err := h.svc.EditTest(ctx, "@dev", task.ID, sel, "x", 0)
		assertErr(t, err, core.ErrNotFound)
		_, err = h.svc.RemoveTest(ctx, "@dev", task.ID, sel, 0)
		assertErr(t, err, core.ErrNotFound)
	}
}

func TestEditAndRemoveTestRecordTestEvents(t *testing.T) {
	h := newHarness(t)
	task := h.add("@dev", core.TypeTask, "t", "", withTests("old", "gone"))
	passAll(t, h, task.ID, "gone")
	if _, err := h.svc.EditTest(ctx, "agent-1", task.ID, "1", "new", 0); err != nil {
		t.Fatal(err)
	}
	ev := lastEvent(t, h, task.ID)
	if ev.Kind != core.EventTest || ev.Actor != "agent-1" || ev.Data["name"] != "new" ||
		ev.Data["state"] != "pending" || ev.Data["edited"] != true || ev.Data["old"] != "old" {
		t.Fatalf("edit event = %+v", ev)
	}
	if _, err := h.svc.RemoveTest(ctx, "agent-1", task.ID, "gone", 0); err != nil {
		t.Fatal(err)
	}
	ev = lastEvent(t, h, task.ID)
	if ev.Kind != core.EventTest || ev.Data["name"] != "gone" || ev.Data["state"] != "passing" || ev.Data["removed"] != true {
		t.Fatalf("remove event = %+v", ev)
	}
}

func TestItemInReviewStaysInReviewWhenATestGoesBackToPending(t *testing.T) {
	h := newHarness(t)
	task := h.add("@dev", core.TypeTask, "t", "", withTests("a"), withStatus(core.StatusDoing))
	passAll(t, h, task.ID, "1")
	if _, err := h.svc.Move(ctx, "@dev", task.ID, core.StatusReview, 0); err != nil {
		t.Fatal(err)
	}
	assertStatus(t, h, task.ID, core.StatusReview)
	if _, err := h.svc.EditTest(ctx, "@dev", task.ID, "1", "a2", 0); err != nil {
		t.Fatal(err)
	}
	assertStatus(t, h, task.ID, core.StatusReview)
	if _, err := h.svc.RemoveTest(ctx, "@dev", task.ID, "1", 0); err != nil {
		t.Fatal(err)
	}
	assertStatus(t, h, task.ID, core.StatusReview)
}
