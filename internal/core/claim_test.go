package core

import (
	"errors"
	"testing"
	"time"
)

var now = time.Date(2026, 11, 9, 14, 0, 0, 0, time.UTC)

func claimed(item *Item, actor string, expires time.Time) *Item {
	item.Claim = &Claim{Actor: actor, ExpiresAt: expires}
	item.Assignee = actor
	return item
}

func TestClaimStateNoneActiveStale(t *testing.T) {
	free := it("A", TypeTask, StatusTodo, "")
	if got := ClaimState(free, now); got != ClaimNone {
		t.Errorf("free item: %v, want none", got)
	}
	active := claimed(it("B", TypeTask, StatusDoing, ""), "agent-1", now.Add(time.Minute))
	if got := ClaimState(active, now); got != ClaimActive {
		t.Errorf("claim expiring in 1m: %v, want active", got)
	}
	stale := claimed(it("C", TypeTask, StatusDoing, ""), "agent-1", now.Add(-time.Second))
	if got := ClaimState(stale, now); got != ClaimStale {
		t.Errorf("expired claim: %v, want stale", got)
	}
	edge := claimed(it("D", TypeTask, StatusDoing, ""), "agent-1", now)
	if got := ClaimState(edge, now); got != ClaimStale {
		t.Errorf("claim expiring exactly now: %v, want stale", got)
	}
}

func TestClaimOnlyTaskAndBug(t *testing.T) {
	s := board(items(
		it("E", TypeEpic, StatusTodo, ""),
		it("S", TypeStory, StatusTodo, ""),
		it("T", TypeTask, StatusTodo, ""),
		it("B", TypeBug, StatusTodo, ""),
		it("ST", TypeSubtask, StatusTodo, "T"),
	))
	for _, id := range []string{"E", "S", "ST"} {
		if err := CanClaim(s, s.Get(id), "agent-1", now); !errors.Is(err, ErrRule) {
			t.Errorf("claim %s: err = %v, want ErrRule", id, err)
		}
	}
	for _, id := range []string{"T", "B"} {
		if err := CanClaim(s, s.Get(id), "agent-1", now); err != nil {
			t.Errorf("claim %s: err = %v, want nil", id, err)
		}
	}
}

func TestClaimRejectedWhenClaimedByOther(t *testing.T) {
	s := board(items(claimed(it("T", TypeTask, StatusDoing, ""), "agent-1", now.Add(5*time.Minute))))
	err := CanClaim(s, s.Get("T"), "agent-2", now)
	if !errors.Is(err, ErrClaimed) {
		t.Fatalf("err = %v, want ErrClaimed", err)
	}
}

func TestReclaimBySameActorRenews(t *testing.T) {
	s := board(items(claimed(it("T", TypeTask, StatusDoing, ""), "agent-1", now.Add(5*time.Minute))))
	if err := CanClaim(s, s.Get("T"), "agent-1", now); err != nil {
		t.Fatalf("same actor reclaim: err = %v, want nil", err)
	}
}

func TestClaimStaleByOtherAllowed(t *testing.T) {
	s := board(items(claimed(it("T", TypeTask, StatusDoing, ""), "agent-1", now.Add(-time.Minute))))
	if err := CanClaim(s, s.Get("T"), "agent-2", now); err != nil {
		t.Fatalf("stale takeover: err = %v, want nil", err)
	}
}

func TestClaimBlockedRejected(t *testing.T) {
	s := board(items(
		it("X", TypeTask, StatusDoing, ""),
		it("T", TypeTask, StatusTodo, ""),
	), blocks("X", "T"))
	err := CanClaim(s, s.Get("T"), "agent-1", now)
	var re *RuleError
	if !errors.As(err, &re) || len(re.Unmet) != 1 || re.Unmet[0] != "blocked by X" {
		t.Fatalf("err = %v, want rule error with unmet [blocked by X]", err)
	}
}

func availableIDs(s *Snapshot, f NextFilter) []string { return ids(Available(s, f, now)) }

func TestAvailableExcludesActiveClaim(t *testing.T) {
	s := board(items(
		claimed(it("A", TypeTask, StatusDoing, ""), "agent-1", now.Add(time.Minute)),
		it("B", TypeTask, StatusTodo, ""),
	))
	if got := availableIDs(s, NextFilter{}); !equal(got, []string{"B"}) {
		t.Fatalf("Available = %v, want [B]", got)
	}
}

func TestAvailableIncludesStaleDoing(t *testing.T) {
	s := board(items(
		claimed(it("A", TypeTask, StatusDoing, ""), "agent-1", now.Add(-time.Minute)),
		it("B", TypeTask, StatusDoing, ""), // released but kept in doing
	))
	if got := availableIDs(s, NextFilter{}); !equal(got, []string{"A", "B"}) {
		t.Fatalf("Available = %v, want [A B]", got)
	}
}

func TestAvailableExcludesBlocked(t *testing.T) {
	s := board(items(
		it("X", TypeTask, StatusTodo, ""),
		it("Y", TypeTask, StatusTodo, ""),
	), blocks("X", "Y"))
	if got := availableIDs(s, NextFilter{}); !equal(got, []string{"X"}) {
		t.Fatalf("Available = %v, want [X]", got)
	}
}

func TestAvailableExcludesBacklogAndReview(t *testing.T) {
	s := board(items(
		it("A", TypeTask, StatusBacklog, ""),
		it("B", TypeTask, StatusReview, ""),
		it("C", TypeTask, StatusDone, ""),
		it("D", TypeTask, StatusTodo, ""),
		it("S", TypeStory, StatusTodo, ""),
	))
	if got := availableIDs(s, NextFilter{}); !equal(got, []string{"D"}) {
		t.Fatalf("Available = %v, want [D]", got)
	}
}

func TestAvailableOrdering(t *testing.T) {
	a := it("KB-10", TypeTask, StatusTodo, "")
	b := it("KB-9", TypeTask, StatusTodo, "")
	c := it("KB-3", TypeBug, StatusTodo, "")
	d := it("KB-4", TypeTask, StatusTodo, "")
	a.Priority, b.Priority, c.Priority, d.Priority = PriorityHigh, PriorityHigh, PriorityMedium, PriorityHigh
	a.Position, b.Position, d.Position = 1, 1, 0
	s := board(items(a, b, c, d))
	// high first; among high, position 0 before 1; equal position by ID.
	if got := availableIDs(s, NextFilter{}); !equal(got, []string{"KB-4", "KB-9", "KB-10", "KB-3"}) {
		t.Fatalf("Available = %v, want [KB-4 KB-9 KB-10 KB-3]", got)
	}
}

func TestAvailableFilterTypeLabelParent(t *testing.T) {
	e := it("E", TypeEpic, StatusTodo, "")
	st := it("S", TypeStory, StatusTodo, "E")
	t1 := it("T1", TypeTask, StatusTodo, "S")
	b1 := it("B1", TypeBug, StatusTodo, "S")
	t2 := it("T2", TypeTask, StatusTodo, "")
	t1.Labels = []string{"api"}
	t2.Labels = []string{"api", "docs"}
	s := board(items(e, st, t1, b1, t2))

	if got := availableIDs(s, NextFilter{Types: []Type{TypeBug}}); !equal(got, []string{"B1"}) {
		t.Errorf("--type bug = %v, want [B1]", got)
	}
	if got := availableIDs(s, NextFilter{Label: "api"}); !equal(got, []string{"T1", "T2"}) {
		t.Errorf("--label api = %v, want [T1 T2]", got)
	}
	// --parent includes every descendant, not only direct children.
	if got := availableIDs(s, NextFilter{Parent: "E"}); !equal(got, []string{"B1", "T1"}) {
		t.Errorf("--parent E = %v, want [B1 T1]", got)
	}
}

func TestOnlyItemsInProgressCanBeStale(t *testing.T) {
	for _, st := range allStatuses() {
		item := claimed(it("A", TypeTask, st, ""), "agent-1", now.Add(-time.Hour))
		if got, want := IsStale(item, now), st == StatusDoing; got != want {
			t.Errorf("IsStale(%s with an expired claim) = %v, want %v", st, got, want)
		}
	}
	if IsStale(it("B", TypeTask, StatusDoing, ""), now) {
		t.Error("an item without a claim is not stale")
	}
}

func allStatuses() []Status { return Statuses() }
