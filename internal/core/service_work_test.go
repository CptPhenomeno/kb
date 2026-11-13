package core_test

import (
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/CptPhenomeno/kb/internal/core"
	"github.com/CptPhenomeno/kb/internal/store"
)

// --- 3.6 Claim, release, touch ---

func TestClaimMovesToDoingAndPropagates(t *testing.T) {
	h := newHarness(t)
	story := h.add("@dev", core.TypeStory, "s", "")
	task := h.add("@dev", core.TypeTask, "t", story.ID, withStatus(core.StatusTodo))
	got, err := h.svc.Claim(ctx, "agent-1", task.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != core.StatusDoing || got.Assignee != "agent-1" || got.Claim == nil ||
		!got.Claim.ExpiresAt.Equal(h.now.Add(ttl)) {
		t.Fatalf("claimed item = %+v (claim %+v)", got, got.Claim)
	}
	assertStatus(t, h, story.ID, core.StatusDoing)
	kinds := eventKinds(h.history(task.ID))
	if !slices.Equal(kinds[len(kinds)-2:], []core.EventKind{core.EventMoved, core.EventClaimed}) {
		t.Fatalf("events = %v, want ... moved, claimed", kinds)
	}
}

func eventKinds(evs []core.Event) []core.EventKind {
	out := make([]core.EventKind, len(evs))
	for i, e := range evs {
		out[i] = e.Kind
	}
	return out
}

func TestClaimByOtherActiveFails(t *testing.T) {
	h := newHarness(t)
	task := h.add("@dev", core.TypeTask, "t", "", withStatus(core.StatusTodo))
	if _, err := h.svc.Claim(ctx, "agent-1", task.ID, 0); err != nil {
		t.Fatal(err)
	}
	h.advance(ttl - time.Second)
	_, err := h.svc.Claim(ctx, "agent-2", task.ID, 0)
	assertErr(t, err, core.ErrClaimed)
}

func TestClaimStaleTakeoverEmitsExpired(t *testing.T) {
	h := newHarness(t)
	task := h.add("@dev", core.TypeTask, "t", "", withStatus(core.StatusTodo))
	if _, err := h.svc.Claim(ctx, "agent-1", task.ID, 0); err != nil {
		t.Fatal(err)
	}
	h.advance(ttl + time.Minute)
	got, err := h.svc.Claim(ctx, "agent-2", task.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got.Claim.Actor != "agent-2" {
		t.Fatalf("claim holder = %s, want agent-2", got.Claim.Actor)
	}
	evs := h.history(task.ID)
	expired, claimedEv := evs[len(evs)-2], evs[len(evs)-1]
	if expired.Kind != core.EventClaimExpired || expired.Data["actor"] != "agent-1" || claimedEv.Kind != core.EventClaimed {
		t.Fatalf("events = %+v / %+v, want claim_expired(agent-1) then claimed", expired, claimedEv)
	}
}

func TestReleaseBackToTodo(t *testing.T) {
	h := newHarness(t)
	task := h.add("@dev", core.TypeTask, "t", "", withStatus(core.StatusTodo))
	if _, err := h.svc.Claim(ctx, "agent-1", task.ID, 0); err != nil {
		t.Fatal(err)
	}
	got, err := h.svc.Release(ctx, "agent-1", task.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != core.StatusTodo || got.Claim != nil || got.Assignee != "" {
		t.Fatalf("released item = %+v", got)
	}
	if ev := lastEvent(t, h, task.ID); ev.Kind != core.EventReleased {
		t.Fatalf("last event = %s, want released", ev.Kind)
	}
}

func TestReleaseStaysDoingWhenSubtasksStarted(t *testing.T) {
	h := newHarness(t)
	task := h.add("@dev", core.TypeTask, "t", "", withStatus(core.StatusTodo))
	if _, err := h.svc.Claim(ctx, "agent-1", task.ID, 0); err != nil {
		t.Fatal(err)
	}
	h.add("agent-1", core.TypeSubtask, "st", task.ID, withStatus(core.StatusDoing))
	got, err := h.svc.Release(ctx, "agent-1", task.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != core.StatusDoing || got.Claim != nil {
		t.Fatalf("released item = %s, claim %+v; want doing without claim", got.Status, got.Claim)
	}
}

func TestReleaseOthersActiveClaim(t *testing.T) {
	h := newHarness(t)
	task := h.add("@dev", core.TypeTask, "t", "", withStatus(core.StatusTodo))
	if _, err := h.svc.Claim(ctx, "agent-1", task.ID, 0); err != nil {
		t.Fatal(err)
	}
	_, err := h.svc.Release(ctx, "agent-2", task.ID, 0)
	assertErr(t, err, core.ErrClaimed)
	// People can always release a claim, e.g. from the Activity view.
	if _, err := h.svc.Release(ctx, "@dev", task.ID, 0); err != nil {
		t.Fatalf("human release: %v", err)
	}
}

func TestTouchRenews(t *testing.T) {
	h := newHarness(t)
	task := h.add("@dev", core.TypeTask, "t", "", withStatus(core.StatusTodo))
	if _, err := h.svc.Claim(ctx, "agent-1", task.ID, 0); err != nil {
		t.Fatal(err)
	}
	h.advance(10 * time.Minute)
	got, err := h.svc.Touch(ctx, "agent-1", task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Claim.ExpiresAt.Equal(h.now.Add(ttl)) {
		t.Fatalf("expires at %v, want %v", got.Claim.ExpiresAt, h.now.Add(ttl))
	}
	_, err = h.svc.Touch(ctx, "agent-2", task.ID)
	assertErr(t, err, core.ErrClaimed)
}

func TestWriteByHolderRenews(t *testing.T) {
	h := newHarness(t)
	task := h.add("@dev", core.TypeTask, "t", "", withStatus(core.StatusTodo), withTests("a"))
	if _, err := h.svc.Claim(ctx, "agent-1", task.ID, 0); err != nil {
		t.Fatal(err)
	}
	h.advance(14 * time.Minute)
	if _, err := h.svc.SetPhase(ctx, "agent-1", task.ID, core.PhaseRed, 0); err != nil {
		t.Fatal(err)
	}
	if got := h.get(task.ID).Claim.ExpiresAt; !got.Equal(h.now.Add(ttl)) {
		t.Fatalf("claim expires at %v, want renewed to %v", got, h.now.Add(ttl))
	}
	h.advance(14 * time.Minute)
	if err := h.svc.Note(ctx, "agent-1", task.ID, "still on it"); err != nil {
		t.Fatal(err)
	}
	if got := h.get(task.ID).Claim.ExpiresAt; !got.Equal(h.now.Add(ttl)) {
		t.Fatalf("a note must renew the claim: expires at %v", got)
	}
}

func TestWriteOnSubtaskRenewsParentClaim(t *testing.T) {
	h := newHarness(t)
	task := h.add("@dev", core.TypeTask, "t", "", withStatus(core.StatusTodo))
	sub := h.add("@dev", core.TypeSubtask, "st", task.ID)
	if _, err := h.svc.Claim(ctx, "agent-1", task.ID, 0); err != nil {
		t.Fatal(err)
	}
	h.advance(10 * time.Minute)
	h.mustMove("agent-1", sub.ID, core.StatusTodo)
	if got := h.get(task.ID).Claim.ExpiresAt; !got.Equal(h.now.Add(ttl)) {
		t.Fatalf("parent claim expires at %v, want %v", got, h.now.Add(ttl))
	}
}

func TestWriteByOtherDoesNotRenew(t *testing.T) {
	h := newHarness(t)
	task := h.add("@dev", core.TypeTask, "t", "", withStatus(core.StatusTodo))
	if _, err := h.svc.Claim(ctx, "agent-1", task.ID, 0); err != nil {
		t.Fatal(err)
	}
	expires := h.get(task.ID).Claim.ExpiresAt
	h.advance(5 * time.Minute)
	if err := h.svc.Note(ctx, "@dev", task.ID, "looks good"); err != nil {
		t.Fatal(err)
	}
	if got := h.get(task.ID).Claim.ExpiresAt; !got.Equal(expires) {
		t.Fatalf("claim renewed by another actor: %v → %v", expires, got)
	}
}

// --- 3.7 Next ---

func TestNextOrdering(t *testing.T) {
	h := newHarness(t)
	h.add("@dev", core.TypeTask, "low", "", withStatus(core.StatusTodo), func(i *core.Item) { i.Priority = core.PriorityLow })
	high := h.add("@dev", core.TypeBug, "high", "", withStatus(core.StatusTodo), func(i *core.Item) { i.Priority = core.PriorityHigh })
	got, err := h.svc.Next(ctx, "agent-1", core.NextFilter{}, false)
	if err != nil || got.ID != high.ID {
		t.Fatalf("Next = %v, %v; want %s", got, err, high.ID)
	}
	if got.Claim != nil {
		t.Fatal("Next without claim must not claim")
	}
}

func TestNextEmpty(t *testing.T) {
	h := newHarness(t)
	h.add("@dev", core.TypeTask, "in backlog", "")
	_, err := h.svc.Next(ctx, "agent-1", core.NextFilter{}, true)
	assertErr(t, err, core.ErrNotFound)
}

func TestNextWithClaimAtomic(t *testing.T) {
	h := newHarness(t)
	task := h.add("@dev", core.TypeTask, "t", "", withStatus(core.StatusTodo))
	got, err := h.svc.Next(ctx, "agent-1", core.NextFilter{}, true)
	if err != nil || got.ID != task.ID || got.Claim == nil || got.Status != core.StatusDoing {
		t.Fatalf("Next --claim = %+v, %v", got, err)
	}
	_, err = h.svc.Next(ctx, "agent-2", core.NextFilter{}, true)
	assertErr(t, err, core.ErrNotFound)
}

// TestNextConcurrentNoDuplicates runs many agents, each with its own
// database connection, claiming concurrently: no item may be handed out
// twice.
func TestNextConcurrentNoDuplicates(t *testing.T) {
	h := newHarness(t)
	const items, agents = 12, 20
	for i := range items {
		h.add("@dev", core.TypeTask, "t"+string(rune('a'+i)), "", withStatus(core.StatusTodo))
	}
	var (
		mu     sync.Mutex
		got    = map[string]string{}
		misses int
		wg     sync.WaitGroup
	)
	for a := range agents {
		db, err := store.Open(h.db.Path())
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { db.Close() })
		svc := h.service(db)
		actor := "agent-" + string(rune('A'+a))
		wg.Add(1)
		go func() {
			defer wg.Done()
			it, err := svc.Next(ctx, actor, core.NextFilter{}, true)
			mu.Lock()
			defer mu.Unlock()
			switch {
			case errors.Is(err, core.ErrNotFound):
				misses++
			case err != nil:
				t.Errorf("%s: %v", actor, err)
			default:
				if prev, dup := got[it.ID]; dup {
					t.Errorf("%s handed to both %s and %s", it.ID, prev, actor)
				}
				got[it.ID] = actor
			}
		}()
	}
	wg.Wait()
	if len(got) != items || misses != agents-items {
		t.Fatalf("claimed %d items with %d misses, want %d and %d", len(got), misses, items, agents-items)
	}
}

// --- 3.8 TDD phase, tests and acceptance criteria ---

func TestSetPhaseOnlyLeaf(t *testing.T) {
	h := newHarness(t)
	story := h.add("@dev", core.TypeStory, "s", "")
	_, err := h.svc.SetPhase(ctx, "@dev", story.ID, core.PhaseRed, 0)
	assertErr(t, err, core.ErrRule)
	task := h.add("@dev", core.TypeTask, "t", "")
	got, err := h.svc.SetPhase(ctx, "@dev", task.ID, core.PhaseGreen, 0)
	if err != nil || got.Phase != core.PhaseGreen {
		t.Fatalf("SetPhase = %v, %v", got, err)
	}
}

func TestSetTestStateByIndexAndName(t *testing.T) {
	h := newHarness(t)
	task := h.add("@dev", core.TypeTask, "t", "", withTests("rotates", "revokes"))
	if _, err := h.svc.SetTestState(ctx, "@dev", task.ID, "2", core.TestFailing, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := h.svc.SetTestState(ctx, "@dev", task.ID, "rotates", core.TestPassing, 0); err != nil {
		t.Fatal(err)
	}
	got := h.get(task.ID)
	if got.Tests[0].State != core.TestPassing || got.Tests[1].State != core.TestFailing {
		t.Fatalf("tests = %+v", got.Tests)
	}
	added, err := h.svc.AddTest(ctx, "@dev", task.ID, "third", 0)
	if err != nil || len(added.Tests) != 3 || added.Tests[2].State != core.TestPending {
		t.Fatalf("AddTest = %+v, %v", added, err)
	}
	_, err = h.svc.AddTest(ctx, "@dev", task.ID, "third", 0)
	assertErr(t, err, core.ErrRule)
}

func TestSetTestStateUnknown(t *testing.T) {
	h := newHarness(t)
	task := h.add("@dev", core.TypeTask, "t", "", withTests("a"))
	for _, sel := range []string{"0", "2", "missing"} {
		_, err := h.svc.SetTestState(ctx, "@dev", task.ID, sel, core.TestPassing, 0)
		assertErr(t, err, core.ErrNotFound)
	}
}

func TestCheckCriterionTriggersPropagation(t *testing.T) {
	h := newHarness(t)
	story := h.add("@dev", core.TypeStory, "s", "", withCriteria("a", "b"))
	h.add("@dev", core.TypeTask, "t", story.ID, withStatus(core.StatusDone))
	assertStatus(t, h, story.ID, core.StatusReview)
	if _, err := h.svc.SetCriterion(ctx, "@dev", story.ID, 1, true, 0); err != nil {
		t.Fatal(err)
	}
	assertStatus(t, h, story.ID, core.StatusReview)
	if _, err := h.svc.SetCriterion(ctx, "@dev", story.ID, 2, true, 0); err != nil {
		t.Fatal(err)
	}
	assertStatus(t, h, story.ID, core.StatusDone)
	_, err := h.svc.SetCriterion(ctx, "@dev", story.ID, 3, true, 0)
	assertErr(t, err, core.ErrNotFound)
}

func TestEventsForPhaseTestAcceptance(t *testing.T) {
	h := newHarness(t)
	task := h.add("@dev", core.TypeTask, "t", "", withTests("a"))
	if _, err := h.svc.SetPhase(ctx, "@dev", task.ID, core.PhaseRed, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := h.svc.SetTestState(ctx, "@dev", task.ID, "1", core.TestFailing, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := h.svc.AddCriterion(ctx, "@dev", task.ID, "works", 0); err != nil {
		t.Fatal(err)
	}
	kinds := eventKinds(h.history(task.ID))
	want := []core.EventKind{core.EventCreated, core.EventPhase, core.EventTest, core.EventAcceptance}
	if !slices.Equal(kinds, want) {
		t.Fatalf("events = %v, want %v", kinds, want)
	}
}

// --- 3.9 Note, remove, log ---

func TestNoteEvent(t *testing.T) {
	h := newHarness(t)
	task := h.add("@dev", core.TypeTask, "t", "")
	if err := h.svc.Note(ctx, "agent-1", task.ID, "validation moved"); err != nil {
		t.Fatal(err)
	}
	ev := lastEvent(t, h, task.ID)
	if ev.Kind != core.EventNote || ev.Data["text"] != "validation moved" || ev.Actor != "agent-1" {
		t.Fatalf("event = %+v", ev)
	}
	if h.get(task.ID).Version != task.Version {
		t.Fatal("a note must not change the item version")
	}
	assertErr(t, h.svc.Note(ctx, "@dev", task.ID, "  "), core.ErrRule)
}

func TestRemoveLeaf(t *testing.T) {
	h := newHarness(t)
	task := h.add("@dev", core.TypeTask, "t", "")
	removed, err := h.svc.Remove(ctx, "@dev", task.ID, false)
	if err != nil || !slices.Equal(removed, []string{task.ID}) {
		t.Fatalf("Remove = %v, %v", removed, err)
	}
	_, err = h.svc.Get(ctx, task.ID)
	assertErr(t, err, core.ErrNotFound)
	if ev := lastEvent(t, h, task.ID); ev.Kind != core.EventDeleted {
		t.Fatalf("last event = %s, want deleted", ev.Kind)
	}
}

func TestRemoveWithChildrenNeedsRecursive(t *testing.T) {
	h := newHarness(t)
	story := h.add("@dev", core.TypeStory, "s", "")
	h.add("@dev", core.TypeTask, "t", story.ID)
	_, err := h.svc.Remove(ctx, "@dev", story.ID, false)
	assertErr(t, err, core.ErrRule)
}

func TestRemoveRecursive(t *testing.T) {
	h := newHarness(t)
	story := h.add("@dev", core.TypeStory, "s", "")
	task := h.add("@dev", core.TypeTask, "t", story.ID)
	sub := h.add("@dev", core.TypeSubtask, "st", task.ID)
	removed, err := h.svc.Remove(ctx, "@dev", story.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{sub.ID, task.ID, story.ID}; !slices.Equal(removed, want) {
		t.Fatalf("removed = %v, want %v (deepest first)", removed, want)
	}
}

func TestRemoveCleansRelations(t *testing.T) {
	h := newHarness(t)
	a := h.add("@dev", core.TypeTask, "a", "")
	b := h.add("@dev", core.TypeTask, "b", "", withStatus(core.StatusTodo))
	if err := h.svc.Link(ctx, "@dev", core.Relation{Kind: core.RelBlocks, From: a.ID, To: b.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := h.svc.Remove(ctx, "@dev", a.ID, false); err != nil {
		t.Fatal(err)
	}
	board, _ := h.svc.Board(ctx)
	if len(board.Relations) != 0 {
		t.Fatalf("relations left = %v", board.Relations)
	}
	h.mustMove("@dev", b.ID, core.StatusDoing)
}

func TestRemovePropagatesParent(t *testing.T) {
	h := newHarness(t)
	story := h.add("@dev", core.TypeStory, "s", "")
	h.add("@dev", core.TypeTask, "done", story.ID, withStatus(core.StatusDone))
	open := h.add("@dev", core.TypeTask, "open", story.ID, withStatus(core.StatusTodo))
	assertStatus(t, h, story.ID, core.StatusDoing)
	if _, err := h.svc.Remove(ctx, "@dev", open.ID, false); err != nil {
		t.Fatal(err)
	}
	assertStatus(t, h, story.ID, core.StatusDone)
}

func TestLogFilters(t *testing.T) {
	h := newHarness(t)
	a := h.add("@dev", core.TypeTask, "a", "")
	h.advance(time.Hour)
	b := h.add("agent-1", core.TypeTask, "b", "")
	if err := h.svc.Note(ctx, "agent-1", a.ID, "n"); err != nil {
		t.Fatal(err)
	}
	all, err := h.svc.Log(ctx, core.LogFilter{})
	if err != nil || len(all) != 3 {
		t.Fatalf("all events = %d, %v", len(all), err)
	}
	byItem, _ := h.svc.Log(ctx, core.LogFilter{Item: a.ID})
	if len(byItem) != 2 {
		t.Fatalf("events of %s = %d, want 2", a.ID, len(byItem))
	}
	since, _ := h.svc.Log(ctx, core.LogFilter{Since: h.now})
	if len(since) != 2 || since[0].Item != b.ID {
		t.Fatalf("events since = %+v", since)
	}
	after, _ := h.svc.Log(ctx, core.LogFilter{AfterID: all[1].ID})
	if len(after) != 1 || after[0].ID != all[2].ID {
		t.Fatalf("events after = %+v", after)
	}
	limited, _ := h.svc.Log(ctx, core.LogFilter{Limit: 2})
	if len(limited) != 2 || limited[1].ID != all[2].ID {
		t.Fatalf("limit must keep the newest events: %+v", limited)
	}
}

// --- NEXT-1 allowed types ---

func nextFor(h *harness, types ...core.Type) (*core.Item, error) {
	return h.svc.Next(ctx, "agent-1", core.NextFilter{Types: types}, false)
}

func TestNextWithTypeStoryReturnsUnblockedStoryInTodo(t *testing.T) {
	h := newHarness(t)
	h.add("@dev", core.TypeTask, "a task", "", withStatus(core.StatusTodo))
	story := h.add("@dev", core.TypeStory, "a story", "", withStatus(core.StatusTodo))
	got, err := nextFor(h, core.TypeStory)
	if err != nil || got.ID != story.ID {
		t.Fatalf("Next(story) = %v, %v; want %s", got, err, story.ID)
	}
}

func TestNextWithTypeEpicReturnsUnblockedEpicInTodo(t *testing.T) {
	h := newHarness(t)
	h.add("@dev", core.TypeStory, "a story", "", withStatus(core.StatusTodo))
	epic := h.add("@dev", core.TypeEpic, "an epic", "", withStatus(core.StatusTodo))
	got, err := nextFor(h, core.TypeEpic)
	if err != nil || got.ID != epic.ID {
		t.Fatalf("Next(epic) = %v, %v; want %s", got, err, epic.ID)
	}
}

func TestNextWithTwoTypesConsidersBoth(t *testing.T) {
	h := newHarness(t)
	story := h.add("@dev", core.TypeStory, "a story", "", withStatus(core.StatusTodo), func(i *core.Item) { i.Priority = core.PriorityHigh })
	task := h.add("@dev", core.TypeTask, "a task", "", withStatus(core.StatusTodo), func(i *core.Item) { i.Priority = core.PriorityLow })
	got, err := nextFor(h, core.TypeStory, core.TypeTask)
	if err != nil || got.ID != story.ID {
		t.Fatalf("Next(story,task) = %v, %v; want %s", got, err, story.ID)
	}
	got, err = nextFor(h, core.TypeTask, core.TypeBug)
	if err != nil || got.ID != task.ID {
		t.Fatalf("Next(task,bug) = %v, %v; want %s", got, err, task.ID)
	}
}

func TestNextWithTypeSubtaskFailsWithRuleError(t *testing.T) {
	h := newHarness(t)
	task := h.add("@dev", core.TypeTask, "a task", "", withStatus(core.StatusTodo))
	h.add("@dev", core.TypeSubtask, "a subtask", task.ID, withStatus(core.StatusTodo))
	_, err := nextFor(h, core.TypeSubtask)
	assertErr(t, err, core.ErrRule)
	if !strings.Contains(err.Error(), "subtask") {
		t.Fatalf("err = %v, want it to name subtasks", err)
	}
}

func TestNextWithSubtaskAmongOtherTypesFailsWithRuleError(t *testing.T) {
	h := newHarness(t)
	h.add("@dev", core.TypeStory, "a story", "", withStatus(core.StatusTodo))
	_, err := nextFor(h, core.TypeStory, core.TypeSubtask)
	assertErr(t, err, core.ErrRule)
}

func TestNextWithoutTypeIgnoresStoriesAndEpics(t *testing.T) {
	h := newHarness(t)
	h.add("@dev", core.TypeEpic, "an epic", "", withStatus(core.StatusTodo))
	h.add("@dev", core.TypeStory, "a story", "", withStatus(core.StatusTodo))
	_, err := nextFor(h)
	assertErr(t, err, core.ErrNotFound)
}

// --- NEXT-5 --claim only takes tasks and bugs ---

func TestNextClaimWithOnlyStoryTypeFailsWithRuleError(t *testing.T) {
	h := newHarness(t)
	h.add("@dev", core.TypeStory, "a story", "", withStatus(core.StatusTodo))
	_, err := h.svc.Next(ctx, "agent-1", core.NextFilter{Types: []core.Type{core.TypeStory}}, true)
	assertErr(t, err, core.ErrRule)
	if !strings.Contains(err.Error(), "stories and epics") {
		t.Fatalf("err = %v, want it to explain stories and epics are not claimed", err)
	}
	_, err = h.svc.Next(ctx, "agent-1", core.NextFilter{Types: []core.Type{core.TypeEpic, core.TypeStory}}, true)
	assertErr(t, err, core.ErrRule)
}

func TestNextClaimWithStoryAndTaskTypesClaimsATask(t *testing.T) {
	h := newHarness(t)
	h.add("@dev", core.TypeStory, "a story", "", withStatus(core.StatusTodo), func(i *core.Item) { i.Priority = core.PriorityHigh })
	task := h.add("@dev", core.TypeTask, "a task", "", withStatus(core.StatusTodo), func(i *core.Item) { i.Priority = core.PriorityLow })
	got, err := h.svc.Next(ctx, "agent-1", core.NextFilter{Types: []core.Type{core.TypeStory, core.TypeTask}}, true)
	if err != nil || got.ID != task.ID || got.Claim == nil {
		t.Fatalf("Next(story,task) --claim = %+v, %v; want claimed %s", got, err, task.ID)
	}
}

func TestNextClaimOfAReviewTaskKeepsItInReview(t *testing.T) {
	h := newHarness(t)
	task := h.add("@dev", core.TypeTask, "a task", "", withStatus(core.StatusReview))
	got, err := h.svc.Next(ctx, "agent-1", core.NextFilter{}, true)
	if err != nil || got.ID != task.ID || got.Claim == nil || got.Status != core.StatusReview {
		t.Fatalf("Next --claim = %+v, %v; want %s claimed and still in review", got, err, task.ID)
	}
}

func TestNextClaimOfATodoTaskMovesItToDoing(t *testing.T) {
	h := newHarness(t)
	task := h.add("@dev", core.TypeTask, "a task", "", withStatus(core.StatusTodo))
	got, err := h.svc.Next(ctx, "agent-1", core.NextFilter{}, true)
	if err != nil || got.ID != task.ID || got.Status != core.StatusDoing {
		t.Fatalf("Next --claim = %+v, %v; want %s in doing", got, err, task.ID)
	}
}
