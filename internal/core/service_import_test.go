package core_test

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/CptPhenomeno/kb/internal/core"
	"github.com/CptPhenomeno/kb/internal/plan"
)

// parsePlan parses YAML into a plan, failing the test on error.
func parsePlan(t *testing.T, src string) *core.ImportPlan {
	t.Helper()
	p, err := plan.Parse(strings.NewReader(src), "plan.yaml")
	if err != nil {
		t.Fatalf("parse plan: %v", err)
	}
	return p
}

const specPlan = `
spec: SPEC.md
status: todo
items:
  - ref: auth
    type: epic
    title: Authentication
    children:
      - ref: auth-login
        type: story
        title: Token login
        points: 5
        acceptance: [expired token → 401, valid token → 200]
        children:
          - ref: auth-login-validate
            type: task
            title: Validate token in middleware
            priority: high
            points: 3
            tests: [rejects expired token, accepts valid token]
            blocks: [auth-login-refresh]
          - ref: auth-login-refresh
            type: task
            title: Token refresh
            points: 2
            tests: [refresh rotates the token]
`

func (h *harness) importPlan(src string, opts core.ImportOptions) (*core.ImportReport, error) {
	h.t.Helper()
	return h.svc.Import(ctx, "agent-plan", parsePlan(h.t, src), opts)
}

func (h *harness) mustImport(src string, opts core.ImportOptions) *core.ImportReport {
	h.t.Helper()
	rep, err := h.importPlan(src, opts)
	if err != nil {
		h.t.Fatalf("Import: %v", err)
	}
	return rep
}

func outcomes(rep *core.ImportReport) []string {
	var out []string
	for _, r := range rep.Items {
		out = append(out, r.Ref+"="+r.ID+":"+r.Outcome)
	}
	return out
}

func assertProblems(t *testing.T, err error, wantSubstrings ...string) {
	t.Helper()
	unmet := unmetOf(t, err)
	joined := strings.Join(unmet, "\n")
	for _, w := range wantSubstrings {
		if !strings.Contains(joined, w) {
			t.Errorf("problems lack %q:\n%s", w, joined)
		}
	}
}

// --- 5.2 validation and ref resolution ---

func TestDuplicateRefInFile(t *testing.T) {
	h := newHarness(t)
	_, err := h.importPlan("items:\n  - {ref: a, type: task, title: A}\n  - {ref: a, type: task, title: B}\n", core.ImportOptions{})
	assertProblems(t, err, `ref "a" is defined twice`)
}

func TestRefConflictsWithBoard(t *testing.T) {
	h := newHarness(t)
	h.add("@dev", core.TypeTask, "existing", "", withRef("a"))
	_, err := h.importPlan("items:\n  - {ref: a, type: task, title: A}\n", core.ImportOptions{})
	assertProblems(t, err, `ref "a" is already used by KB-1`, "--update")
}

func TestForwardRefResolved(t *testing.T) {
	h := newHarness(t)
	rep := h.mustImport(specPlan, core.ImportOptions{})
	_ = rep
	board, _ := h.svc.Board(ctx)
	if blocked, by := core.Blockers(board, "KB-4"); !blocked || !slices.Equal(by, []string{"KB-3"}) {
		t.Fatalf("KB-4 blockers = %v, want [KB-3] (forward ref)", by)
	}
}

func TestExistingIDResolved(t *testing.T) {
	h := newHarness(t)
	story := h.add("@dev", core.TypeStory, "existing story", "")
	other := h.add("@dev", core.TypeTask, "other", "", withRef("other"))
	h.mustImport("items:\n  - ref: t\n    type: task\n    title: T\n    parent: "+story.ID+"\n    relates: [other]\n    blocks: ["+other.ID+"]\n", core.ImportOptions{})
	got, err := h.svc.GetByRef(ctx, "t")
	if err != nil || got.Parent != story.ID {
		t.Fatalf("imported task = %+v, %v", got, err)
	}
	board, _ := h.svc.Board(ctx)
	if blocked, _ := core.Blockers(board, other.ID); !blocked {
		t.Fatal("blocks to an existing ID not applied")
	}
}

func TestUnknownRefRejected(t *testing.T) {
	h := newHarness(t)
	_, err := h.importPlan("items:\n  - {ref: a, type: task, title: A, blocks: [nowhere]}\n  - {ref: b, type: task, title: B, parent: KB-99}\n", core.ImportOptions{})
	assertProblems(t, err, `plan.yaml:2 (a): unknown item "nowhere"`, `plan.yaml:3 (b): unknown parent "KB-99"`)
}

func TestHierarchyViolationInPlan(t *testing.T) {
	h := newHarness(t)
	src := "items:\n  - ref: e\n    type: epic\n    title: E\n    children:\n      - {ref: t, type: task, title: T}\n"
	_, err := h.importPlan(src, core.ImportOptions{})
	assertProblems(t, err, "plan.yaml:6 (t): a task cannot be placed under a epic")
	_, err = h.importPlan("items:\n  - {ref: x, type: task, title: X, points: 4}\n  - {ref: y, title: Y}\n", core.ImportOptions{})
	assertProblems(t, err, "(x): points must be one of", "(y): type is required")
}

func TestCycleAcrossPlanAndBoard(t *testing.T) {
	h := newHarness(t)
	a := h.add("@dev", core.TypeTask, "a", "", withRef("a"))
	b := h.add("@dev", core.TypeTask, "b", "", withRef("b"))
	if err := h.svc.Link(ctx, "@dev", core.Relation{Kind: core.RelBlocks, From: a.ID, To: b.ID}); err != nil {
		t.Fatal(err)
	}
	// The board has a → b; the plan adds b → c and c → a, closing a cycle.
	src := "items:\n  - {ref: b, blocks: [c]}\n  - {ref: c, type: task, title: C, blocks: [a]}\n" +
		"  - {ref: e, type: task, title: E, blocks: [e]}\n"
	_, err := h.importPlan(src, core.ImportOptions{Update: true})
	assertProblems(t, err, "would create a cycle of blocks", "(e): e cannot be linked to itself")
}

func TestAllErrorsReported(t *testing.T) {
	h := newHarness(t)
	src := "items:\n  - {ref: a, type: task, title: A, points: 4}\n  - {ref: a, type: task, title: dup}\n  - {ref: c, type: task, title: C, blocks: [zzz]}\n"
	_, err := h.importPlan(src, core.ImportOptions{})
	if got := unmetOf(t, err); len(got) != 3 {
		t.Fatalf("problems = %q, want 3", got)
	}
}

// --- 5.3 applying in one transaction ---

func TestImportCreatesTree(t *testing.T) {
	h := newHarness(t)
	h.mustImport(specPlan, core.ImportOptions{})
	board, _ := h.svc.Board(ctx)
	if n := len(board.Items()); n != 4 {
		t.Fatalf("%d items, want 4", n)
	}
	task, _ := h.svc.GetByRef(ctx, "auth-login-validate")
	story, _ := h.svc.GetByRef(ctx, "auth-login")
	if task.Parent != story.ID || task.Status != core.StatusTodo || task.Priority != core.PriorityHigh ||
		len(task.Tests) != 2 || task.Spec != "SPEC.md" || task.CreatedBy != "agent-plan" {
		t.Fatalf("task = %+v", task)
	}
	if len(story.Acceptance) != 2 || story.Points != 5 || story.Status != core.StatusTodo {
		t.Fatalf("story = %+v", story)
	}
	if ev := lastEvent(t, h, task.ID); ev.Kind != core.EventLinked && ev.Kind != core.EventCreated {
		t.Fatalf("task events end with %s", ev.Kind)
	}
}

func TestImportAtomicOnFailure(t *testing.T) {
	h := newHarness(t)
	src := specPlan + "  - {ref: broken, type: task, title: X, points: 4}\n"
	if _, err := h.importPlan(src, core.ImportOptions{}); !errors.Is(err, core.ErrRule) {
		t.Fatalf("err = %v, want ErrRule", err)
	}
	board, _ := h.svc.Board(ctx)
	if n := len(board.Items()); n != 0 {
		t.Fatalf("%d items written by a failed import", n)
	}
	if evs, _ := h.svc.Log(ctx, core.LogFilter{}); len(evs) != 0 {
		t.Fatalf("%d events written by a failed import", len(evs))
	}
	// IDs reserved by the failed import are not consumed.
	if it := h.add("@dev", core.TypeTask, "t", ""); it.ID != "KB-1" {
		t.Fatalf("next ID = %s, want KB-1", it.ID)
	}
}

func TestImportReportRefToID(t *testing.T) {
	h := newHarness(t)
	rep := h.mustImport(specPlan, core.ImportOptions{})
	want := []string{"auth=KB-1:created", "auth-login=KB-2:created", "auth-login-validate=KB-3:created", "auth-login-refresh=KB-4:created"}
	if got := outcomes(rep); !slices.Equal(got, want) {
		t.Fatalf("report = %v, want %v", got, want)
	}
	if rep.Created != 4 || rep.Updated != 0 || rep.Unchanged != 0 || rep.DryRun {
		t.Fatalf("totals = %+v", rep)
	}
}

func TestImportInitialStatusRequirements(t *testing.T) {
	h := newHarness(t)
	src := "items:\n  - {ref: a, type: task, title: A, status: done, tests: [x]}\n" +
		"  - {ref: b, type: task, title: B, status: todo}\n  - {ref: c, type: task, title: C, status: doing}\n" +
		"  - {ref: d, type: task, title: D, blocks: [c]}\n"
	_, err := h.importPlan(src, core.ImportOptions{})
	assertProblems(t, err, `(a): cannot start in done: test 1 "x" is pending`, "(c): cannot start in doing: blocked by")
}

func TestImportPropagatesOnce(t *testing.T) {
	h := newHarness(t)
	src := "items:\n  - ref: s\n    type: story\n    title: S\n    status: todo\n    children:\n      - {ref: t, type: task, title: T, status: doing}\n"
	h.mustImport(src, core.ImportOptions{})
	s, _ := h.svc.GetByRef(ctx, "s")
	if s.Status != core.StatusDoing {
		t.Fatalf("story = %s, want doing (derived from its child)", s.Status)
	}
}

// --- 5.4 --update ---

func TestUpdateOnlyPresentFields(t *testing.T) {
	h := newHarness(t)
	h.mustImport(specPlan, core.ImportOptions{})
	if _, err := h.svc.Edit(ctx, "@dev", "KB-4", core.Patch{Description: ptr("kept")}, 0); err != nil {
		t.Fatal(err)
	}
	h.mustImport("items:\n  - {ref: auth-login-refresh, title: Refresh tokens, points: 3}\n", core.ImportOptions{Update: true})
	got := h.get("KB-4")
	if got.Title != "Refresh tokens" || got.Points != 3 || got.Description != "kept" || got.Parent != "KB-2" {
		t.Fatalf("updated item = %+v", got)
	}
}

func TestUpdateIgnoresStatus(t *testing.T) {
	h := newHarness(t)
	h.mustImport(specPlan, core.ImportOptions{})
	if _, err := h.svc.Claim(ctx, "agent-auth", "KB-3", 0); err != nil {
		t.Fatal(err)
	}
	h.mustImport(specPlan, core.ImportOptions{Update: true}) // status: todo at plan level
	if got := h.status("KB-3"); got != core.StatusDoing {
		t.Fatalf("status = %s, want doing (kept)", got)
	}
}

func TestUpdateMergesTestsKeepsState(t *testing.T) {
	h := newHarness(t)
	h.mustImport(specPlan, core.ImportOptions{})
	if _, err := h.svc.SetTestState(ctx, "@dev", "KB-3", "1", core.TestPassing, 0); err != nil {
		t.Fatal(err)
	}
	src := "items:\n  - {ref: auth-login-validate, tests: [rejects expired token, rejects wrong audience]}\n"
	h.mustImport(src, core.ImportOptions{Update: true})
	tests := h.get("KB-3").Tests
	want := []core.Test{{Name: "rejects expired token", State: core.TestPassing}, {Name: "accepts valid token"},
		{Name: "rejects wrong audience"}}
	if !slices.Equal(tests, want) {
		t.Fatalf("tests = %+v, want %+v", tests, want)
	}
}

func TestUpdateMergesAcceptanceKeepsDone(t *testing.T) {
	h := newHarness(t)
	h.mustImport(specPlan, core.ImportOptions{})
	if _, err := h.svc.SetCriterion(ctx, "@dev", "KB-2", 1, true, 0); err != nil {
		t.Fatal(err)
	}
	h.mustImport("items:\n  - {ref: auth-login, acceptance: [expired token → 401, revoked token → 401]}\n", core.ImportOptions{Update: true})
	acc := h.get("KB-2").Acceptance
	if len(acc) != 3 || !acc[0].Done || acc[2].Text != "revoked token → 401" || acc[2].Done {
		t.Fatalf("acceptance = %+v", acc)
	}
}

func TestUpdateAddsRelations(t *testing.T) {
	h := newHarness(t)
	h.mustImport(specPlan, core.ImportOptions{})
	h.mustImport("items:\n  - {ref: auth-login-refresh, relates: [auth-login-validate]}\n", core.ImportOptions{Update: true})
	board, _ := h.svc.Board(ctx)
	if len(board.Relations) != 2 {
		t.Fatalf("relations = %v, want the original blocks plus relates", board.Relations)
	}
}

func TestUpdateLeavesMissingUntouched(t *testing.T) {
	h := newHarness(t)
	h.mustImport(specPlan, core.ImportOptions{})
	before := h.get("KB-1")
	rep := h.mustImport("items:\n  - {ref: auth-login, title: Token login}\n", core.ImportOptions{Update: true})
	if after := h.get("KB-1"); after.Version != before.Version {
		t.Fatal("an item missing from the plan was modified")
	}
	if len(rep.Items) != 1 {
		t.Fatalf("report = %v, want only the plan's item", outcomes(rep))
	}
}

func TestUpdateReparent(t *testing.T) {
	h := newHarness(t)
	h.mustImport(specPlan, core.ImportOptions{})
	src := "items:\n  - ref: auth-keys\n    type: story\n    title: API keys\n    children:\n      - {ref: auth-login-refresh}\n"
	h.mustImport(src, core.ImportOptions{Update: true})
	keys, _ := h.svc.GetByRef(ctx, "auth-keys")
	if got := h.get("KB-4").Parent; got != keys.ID {
		t.Fatalf("parent = %s, want %s", got, keys.ID)
	}
	_, err := h.importPlan("items:\n  - ref: auth\n    children:\n      - {ref: auth-login-refresh}\n", core.ImportOptions{Update: true})
	assertProblems(t, err, "a task cannot be placed under a epic")
}

func TestUpdateUnchangedReported(t *testing.T) {
	h := newHarness(t)
	h.mustImport(specPlan, core.ImportOptions{})
	rep := h.mustImport(specPlan, core.ImportOptions{Update: true})
	if rep.Unchanged != 4 || rep.Updated != 0 || rep.Created != 0 {
		t.Fatalf("re-import of the same plan = %v", outcomes(rep))
	}
	if evs, _ := h.svc.Log(ctx, core.LogFilter{AfterID: 0}); len(evs) == 0 {
		t.Fatal("no events")
	}
}

func TestUpdateOnClaimedItemRecorded(t *testing.T) {
	h := newHarness(t)
	h.mustImport(specPlan, core.ImportOptions{})
	if _, err := h.svc.Claim(ctx, "agent-auth", "KB-3", 0); err != nil {
		t.Fatal(err)
	}
	rep := h.mustImport("items:\n  - {ref: auth-login-validate, title: Validate JWT}\n", core.ImportOptions{Update: true})
	if got := outcomes(rep); !slices.Equal(got, []string{"auth-login-validate=KB-3:updated"}) {
		t.Fatalf("report = %v", got)
	}
	ev := lastEvent(t, h, "KB-3")
	if ev.Kind != core.EventUpdated || ev.Actor != "agent-plan" {
		t.Fatalf("event = %+v", ev)
	}
	if got := h.get("KB-3"); got.Claim == nil || got.Claim.Actor != "agent-auth" {
		t.Fatal("the claim must survive an import")
	}
}

func TestUpdateTypeChangeRejected(t *testing.T) {
	h := newHarness(t)
	h.mustImport(specPlan, core.ImportOptions{})
	_, err := h.importPlan("items:\n  - {ref: auth-login-refresh, type: bug}\n", core.ImportOptions{Update: true})
	assertProblems(t, err, "type cannot change")
}

// --- 5.5 dry run ---

func TestDryRunWritesNothing(t *testing.T) {
	h := newHarness(t)
	rep := h.mustImport(specPlan, core.ImportOptions{DryRun: true})
	if !rep.DryRun || rep.Created != 4 {
		t.Fatalf("dry run report = %+v", rep)
	}
	board, _ := h.svc.Board(ctx)
	if len(board.Items()) != 0 {
		t.Fatal("dry run wrote items")
	}
	if rep2 := h.mustImport(specPlan, core.ImportOptions{}); !slices.Equal(outcomes(rep2), outcomes(rep)) {
		t.Fatalf("dry run predicted %v, real import did %v", outcomes(rep), outcomes(rep2))
	}
}
