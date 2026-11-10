package core_test

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/CptPhenomeno/kb/internal/core"
)

// --- 3.2 Create ---

func TestCreateDefaultsBacklogMedium(t *testing.T) {
	h := newHarness(t)
	it := h.add("@dev", core.TypeTask, "Token refresh", "")
	if it.ID != "KB-1" || it.Status != core.StatusBacklog || it.Priority != core.PriorityMedium || it.Version != 1 {
		t.Fatalf("created item = %+v", it)
	}
	if it.CreatedBy != "@dev" || it.UpdatedBy != "@dev" || !it.CreatedAt.Equal(h.now) {
		t.Fatalf("audit fields = %s/%s/%v", it.CreatedBy, it.UpdatedBy, it.CreatedAt)
	}
	if second := h.add("@dev", core.TypeTask, "b", ""); second.Position != it.Position+1 {
		t.Fatalf("second item position = %d, want %d (bottom of column)", second.Position, it.Position+1)
	}
}

func TestCreateInvalidParentRejected(t *testing.T) {
	h := newHarness(t)
	epic := h.add("@dev", core.TypeEpic, "e", "")
	_, err := h.svc.Create(ctx, "@dev", core.Item{Type: core.TypeTask, Title: "t", Parent: epic.ID})
	assertErr(t, err, core.ErrRule)
	_, err = h.svc.Create(ctx, "@dev", core.Item{Type: core.TypeSubtask, Title: "st"})
	assertErr(t, err, core.ErrRule)
	_, err = h.svc.Create(ctx, "@dev", core.Item{Type: core.TypeStory, Title: "s", Parent: "KB-99"})
	assertErr(t, err, core.ErrNotFound)
	_, err = h.svc.Create(ctx, "@dev", core.Item{Type: core.TypeTask, Title: " "})
	assertErr(t, err, core.ErrRule)
}

func TestCreateWithStatusChecksRequirements(t *testing.T) {
	h := newHarness(t)
	_, err := h.svc.Create(ctx, "@dev", core.Item{Type: core.TypeTask, Title: "t", Status: core.StatusDone,
		Tests: []core.Test{{Name: "a"}}})
	if got := unmetOf(t, err); !slices.Equal(got, []string{`test 1 "a" is pending`}) {
		t.Fatalf("unmet = %q", got)
	}
	blocker := h.add("@dev", core.TypeTask, "blocker", "")
	story := h.add("@dev", core.TypeStory, "s", "")
	if err := h.svc.Link(ctx, "@dev", core.Relation{Kind: core.RelBlocks, From: blocker.ID, To: story.ID}); err != nil {
		t.Fatal(err)
	}
	_, err = h.svc.Create(ctx, "@dev", core.Item{Type: core.TypeTask, Title: "t", Parent: story.ID, Status: core.StatusDoing})
	if got := unmetOf(t, err); !slices.Equal(got, []string{"blocked by " + blocker.ID}) {
		t.Fatalf("unmet = %q", got)
	}
	todo := h.add("@dev", core.TypeTask, "ready", "", withStatus(core.StatusTodo))
	if todo.Status != core.StatusTodo {
		t.Fatalf("status = %s, want todo", todo.Status)
	}
}

func TestCreateEmitsEvent(t *testing.T) {
	h := newHarness(t)
	it := h.add("agent-plan", core.TypeTask, "t", "")
	ev := lastEvent(t, h, it.ID)
	if ev.Kind != core.EventCreated || ev.Actor != "agent-plan" || !ev.At.Equal(h.now) {
		t.Fatalf("event = %+v", ev)
	}
}

func TestCreateChildRegressesDoneParent(t *testing.T) {
	h := newHarness(t)
	story := h.add("@dev", core.TypeStory, "s", "")
	h.add("@dev", core.TypeTask, "a", story.ID, withStatus(core.StatusDone))
	assertStatus(t, h, story.ID, core.StatusDone)
	h.add("@dev", core.TypeTask, "new", story.ID)
	assertStatus(t, h, story.ID, core.StatusDoing)
	ev := lastEvent(t, h, story.ID)
	if ev.Kind != core.EventMoved || ev.Data["derived"] != true {
		t.Fatalf("last story event = %+v, want a derived move", ev)
	}
}

func TestCreateDuplicateRefRejected(t *testing.T) {
	h := newHarness(t)
	h.add("@dev", core.TypeTask, "a", "", withRef("auth"))
	_, err := h.svc.Create(ctx, "@dev", core.Item{Type: core.TypeTask, Title: "b", Ref: "auth"})
	assertErr(t, err, core.ErrRule)
	byRef, err := h.svc.GetByRef(ctx, "auth")
	if err != nil || byRef.Title != "a" {
		t.Fatalf("GetByRef = %+v, %v", byRef, err)
	}
}

// --- 3.3 Edit ---

func ptr[T any](v T) *T { return &v }

func TestEditPartialFields(t *testing.T) {
	h := newHarness(t)
	it := h.add("@dev", core.TypeTask, "old", "", func(i *core.Item) { i.Description = "keep me" })
	h.advance(time.Minute)
	got, err := h.svc.Edit(ctx, "agent-1", it.ID, core.Patch{Title: ptr("new"), Priority: ptr(core.PriorityHigh),
		Points: ptr(3), Labels: ptr([]string{"api"})}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "new" || got.Description != "keep me" || got.Priority != core.PriorityHigh ||
		got.Points != 3 || !slices.Equal(got.Labels, []string{"api"}) {
		t.Fatalf("edited item = %+v", got)
	}
	if got.UpdatedBy != "agent-1" || !got.UpdatedAt.Equal(h.now) || got.Version != 2 {
		t.Fatalf("audit = %s %v v%d", got.UpdatedBy, got.UpdatedAt, got.Version)
	}
}

func TestEditVersionConflict(t *testing.T) {
	h := newHarness(t)
	it := h.add("@dev", core.TypeTask, "t", "")
	_, err := h.svc.Edit(ctx, "@dev", it.ID, core.Patch{Title: ptr("x")}, 5)
	assertErr(t, err, core.ErrConflict)
	if _, err := h.svc.Edit(ctx, "@dev", it.ID, core.Patch{Title: ptr("x")}, 1); err != nil {
		t.Fatalf("edit at the right version: %v", err)
	}
}

func TestEditReparentPropagatesBoth(t *testing.T) {
	h := newHarness(t)
	oldStory := h.add("@dev", core.TypeStory, "old", "")
	newStory := h.add("@dev", core.TypeStory, "new", "")
	h.add("@dev", core.TypeTask, "done", oldStory.ID, withStatus(core.StatusDone))
	moving := h.add("@dev", core.TypeTask, "moving", oldStory.ID, withStatus(core.StatusTodo))
	h.add("@dev", core.TypeTask, "done2", newStory.ID, withStatus(core.StatusDone))
	assertStatus(t, h, oldStory.ID, core.StatusDoing)
	assertStatus(t, h, newStory.ID, core.StatusDone)

	if _, err := h.svc.Edit(ctx, "@dev", moving.ID, core.Patch{Parent: ptr(newStory.ID)}, 0); err != nil {
		t.Fatal(err)
	}
	assertStatus(t, h, oldStory.ID, core.StatusDone)  // only done children left
	assertStatus(t, h, newStory.ID, core.StatusDoing) // gained a todo child
}

func TestEditInvalidPointsRejected(t *testing.T) {
	h := newHarness(t)
	it := h.add("@dev", core.TypeTask, "t", "")
	_, err := h.svc.Edit(ctx, "@dev", it.ID, core.Patch{Points: ptr(4)}, 0)
	assertErr(t, err, core.ErrRule)
	_, err = h.svc.Edit(ctx, "@dev", it.ID, core.Patch{Parent: ptr("KB-404")}, 0)
	assertErr(t, err, core.ErrNotFound)
}

func TestEditEventHasDiff(t *testing.T) {
	h := newHarness(t)
	it := h.add("@dev", core.TypeTask, "old", "")
	if _, err := h.svc.Edit(ctx, "@dev", it.ID, core.Patch{Title: ptr("new"), Description: ptr("")}, 0); err != nil {
		t.Fatal(err)
	}
	ev := lastEvent(t, h, it.ID)
	changes, _ := ev.Data["changes"].(map[string]any)
	title, _ := changes["title"].([]any)
	if ev.Kind != core.EventUpdated || len(title) != 2 || title[0] != "old" || title[1] != "new" {
		t.Fatalf("event = %+v", ev)
	}
	if _, ok := changes["description"]; ok {
		t.Fatal("unchanged fields must not appear in the diff")
	}
}

// --- 3.4 Move and reorder ---

func TestMoveAllowed(t *testing.T) {
	h := newHarness(t)
	it := h.add("@dev", core.TypeTask, "t", "")
	h.add("@dev", core.TypeTask, "already in todo", "", withStatus(core.StatusTodo))
	res := h.mustMove("@dev", it.ID, core.StatusTodo)
	if res.Item.Status != core.StatusTodo || res.Item.Position != 1 {
		t.Fatalf("moved item = %s at %d, want todo at the bottom (1)", res.Item.Status, res.Item.Position)
	}
	ev := lastEvent(t, h, it.ID)
	if ev.Kind != core.EventMoved || ev.Data["from"] != "backlog" || ev.Data["to"] != "todo" {
		t.Fatalf("event = %+v", ev)
	}
}

func TestMoveRejectedListsUnmet(t *testing.T) {
	h := newHarness(t)
	it := h.add("@dev", core.TypeTask, "t", "", withTests("a", "b"))
	_, err := h.svc.Move(ctx, "@dev", it.ID, core.StatusReview, 0)
	if got := unmetOf(t, err); len(got) != 2 {
		t.Fatalf("unmet = %q, want two pending tests", got)
	}
	assertStatus(t, h, it.ID, core.StatusBacklog)
}

func TestMoveDerivedEventsAttributedToActor(t *testing.T) {
	h := newHarness(t)
	epic := h.add("@dev", core.TypeEpic, "e", "")
	story := h.add("@dev", core.TypeStory, "s", epic.ID)
	task := h.add("@dev", core.TypeTask, "t", story.ID, withStatus(core.StatusTodo))
	res := h.mustMove("agent-1", task.ID, core.StatusDoing)
	if len(res.Derived) != 2 {
		t.Fatalf("derived = %v, want story and epic", res.Derived)
	}
	for _, id := range []string{story.ID, epic.ID} {
		ev := lastEvent(t, h, id)
		if ev.Actor != "agent-1" || ev.Data["derived"] != true || ev.Data["to"] != "doing" {
			t.Errorf("%s event = %+v", id, ev)
		}
	}
}

func TestMoveParentManuallyWithinRequirements(t *testing.T) {
	h := newHarness(t)
	story := h.add("@dev", core.TypeStory, "s", "")
	h.add("@dev", core.TypeTask, "t", story.ID, withStatus(core.StatusTodo))
	assertStatus(t, h, story.ID, core.StatusTodo)
	h.mustMove("@dev", story.ID, core.StatusDoing)
	assertStatus(t, h, story.ID, core.StatusDoing)
}

func TestMoveParentRejectedOutsideRequirements(t *testing.T) {
	h := newHarness(t)
	story := h.add("@dev", core.TypeStory, "s", "")
	task := h.add("@dev", core.TypeTask, "t", story.ID, withStatus(core.StatusTodo))
	_, err := h.svc.Move(ctx, "@dev", story.ID, core.StatusDone, 0)
	if got := unmetOf(t, err); !slices.Equal(got, []string{"child " + task.ID + " is todo"}) {
		t.Fatalf("unmet = %q", got)
	}
}

func TestReorderAlwaysAllowed(t *testing.T) {
	h := newHarness(t)
	a := h.add("@dev", core.TypeTask, "a", "")
	b := h.add("@dev", core.TypeTask, "b", "")
	c := h.add("@dev", core.TypeTask, "c", "")
	if _, err := h.svc.Reorder(ctx, "@dev", c.ID, 0, 0); err != nil {
		t.Fatal(err)
	}
	board, err := h.svc.Board(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var order []string
	for _, it := range core.Column(board, core.StatusBacklog) {
		order = append(order, it.ID)
	}
	if want := []string{c.ID, a.ID, b.ID}; !slices.Equal(order, want) {
		t.Fatalf("column order = %v, want %v", order, want)
	}
}

func TestMoveSameStatusNoop(t *testing.T) {
	h := newHarness(t)
	it := h.add("@dev", core.TypeTask, "t", "")
	res := h.mustMove("@dev", it.ID, core.StatusBacklog)
	if res.Item.Version != it.Version || len(h.history(it.ID)) != 1 {
		t.Fatalf("no-op move wrote: version %d, %d events", res.Item.Version, len(h.history(it.ID)))
	}
}

// --- 3.5 Link and unlink ---

func TestLinkBlocks(t *testing.T) {
	h := newHarness(t)
	a := h.add("@dev", core.TypeTask, "a", "", withStatus(core.StatusTodo))
	b := h.add("@dev", core.TypeTask, "b", "", withStatus(core.StatusTodo))
	if err := h.svc.Link(ctx, "@dev", core.Relation{Kind: core.RelBlocks, From: a.ID, To: b.ID}); err != nil {
		t.Fatal(err)
	}
	_, err := h.svc.Move(ctx, "@dev", b.ID, core.StatusDoing, 0)
	if got := unmetOf(t, err); !slices.Equal(got, []string{"blocked by " + a.ID}) {
		t.Fatalf("unmet = %q", got)
	}
	for _, id := range []string{a.ID, b.ID} {
		if ev := lastEvent(t, h, id); ev.Kind != core.EventLinked {
			t.Errorf("%s last event = %s, want linked", id, ev.Kind)
		}
	}
}

func TestLinkCycleRejected(t *testing.T) {
	h := newHarness(t)
	a := h.add("@dev", core.TypeTask, "a", "")
	b := h.add("@dev", core.TypeTask, "b", "")
	if err := h.svc.Link(ctx, "@dev", core.Relation{Kind: core.RelBlocks, From: a.ID, To: b.ID}); err != nil {
		t.Fatal(err)
	}
	assertErr(t, h.svc.Link(ctx, "@dev", core.Relation{Kind: core.RelBlocks, From: b.ID, To: a.ID}), core.ErrRule)
}

func TestLinkRelatesIdempotent(t *testing.T) {
	h := newHarness(t)
	a := h.add("@dev", core.TypeTask, "a", "")
	b := h.add("@dev", core.TypeTask, "b", "")
	for _, r := range []core.Relation{{Kind: core.RelRelates, From: a.ID, To: b.ID}, {Kind: core.RelRelates, From: b.ID, To: a.ID}} {
		if err := h.svc.Link(ctx, "@dev", r); err != nil {
			t.Fatal(err)
		}
	}
	board, _ := h.svc.Board(ctx)
	if len(board.Relations) != 1 {
		t.Fatalf("relations = %v, want one", board.Relations)
	}
	if n := len(h.history(a.ID)); n != 2 {
		t.Fatalf("%d events on %s, want created + one linked", n, a.ID)
	}
}

func TestUnlinkMissingIsNotFound(t *testing.T) {
	h := newHarness(t)
	a := h.add("@dev", core.TypeTask, "a", "")
	b := h.add("@dev", core.TypeTask, "b", "")
	assertErr(t, h.svc.Unlink(ctx, "@dev", core.Relation{Kind: core.RelBlocks, From: a.ID, To: b.ID}), core.ErrNotFound)
	if err := h.svc.Link(ctx, "@dev", core.Relation{Kind: core.RelBlocks, From: a.ID, To: b.ID}); err != nil {
		t.Fatal(err)
	}
	if err := h.svc.Unlink(ctx, "@dev", core.Relation{Kind: core.RelBlocks, From: a.ID, To: b.ID}); err != nil {
		t.Fatal(err)
	}
	if ev := lastEvent(t, h, a.ID); ev.Kind != core.EventUnlinked {
		t.Fatalf("last event = %s, want unlinked", ev.Kind)
	}
}

func TestBlockingDoneUnblocksTarget(t *testing.T) {
	h := newHarness(t)
	a := h.add("@dev", core.TypeTask, "a", "", withStatus(core.StatusReview))
	b := h.add("@dev", core.TypeTask, "b", "", withStatus(core.StatusTodo))
	if err := h.svc.Link(ctx, "@dev", core.Relation{Kind: core.RelBlocks, From: a.ID, To: b.ID}); err != nil {
		t.Fatal(err)
	}
	h.mustMove("@dev", a.ID, core.StatusDone)
	h.mustMove("@dev", b.ID, core.StatusDoing)
}

// ArrangeColumn writes the order of a whole column at once (the TUI fixes
// a sorted column before a manual reorder): positions only, no change of
// updated_at and no events.
func TestArrangeColumn(t *testing.T) {
	h := newHarness(t)
	a := h.add("@dev", core.TypeTask, "a", "")
	b := h.add("@dev", core.TypeTask, "b", "")
	c := h.add("@dev", core.TypeTask, "c", "")
	h.advance(time.Hour)
	if err := h.svc.ArrangeColumn(ctx, "@dev", core.StatusBacklog, []string{c.ID, a.ID, b.ID}); err != nil {
		t.Fatal(err)
	}
	board, err := h.svc.Board(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var order []string
	for _, it := range core.Column(board, core.StatusBacklog) {
		order = append(order, it.ID)
		if !it.UpdatedAt.Equal(a.UpdatedAt) {
			t.Errorf("%s updated_at changed to %v", it.ID, it.UpdatedAt)
		}
		if n := len(h.history(it.ID)); n != 1 {
			t.Errorf("%s has %d events, want only created", it.ID, n)
		}
	}
	if want := []string{c.ID, a.ID, b.ID}; !slices.Equal(order, want) {
		t.Fatalf("column order = %v, want %v", order, want)
	}
}

func TestArrangeColumnNeedsTheWholeColumn(t *testing.T) {
	h := newHarness(t)
	a := h.add("@dev", core.TypeTask, "a", "")
	b := h.add("@dev", core.TypeTask, "b", "")
	todo := h.add("@dev", core.TypeTask, "t", "", withStatus(core.StatusTodo))
	for _, ids := range [][]string{{a.ID}, {a.ID, b.ID, todo.ID}, {a.ID, a.ID}} {
		if err := h.svc.ArrangeColumn(ctx, "@dev", core.StatusBacklog, ids); !errors.Is(err, core.ErrRule) {
			t.Errorf("ArrangeColumn(%v) = %v, want a rule error", ids, err)
		}
	}
}
