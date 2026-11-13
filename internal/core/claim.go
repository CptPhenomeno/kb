package core

import (
	"cmp"
	"fmt"
	"slices"
	"time"
)

// ClaimStatus describes the claim on an item at a given time.
type ClaimStatus int

// Claim statuses. A stale claim has expired but has not been taken over
// yet: expiry is computed lazily, with no background process (SPEC.md §4.7).
const (
	ClaimNone ClaimStatus = iota
	ClaimActive
	ClaimStale
)

func (c ClaimStatus) String() string {
	switch c {
	case ClaimActive:
		return "active"
	case ClaimStale:
		return "stale"
	default:
		return "none"
	}
}

// ClaimState returns the state of the item's claim at time now. A claim
// expiring exactly at now is stale.
func ClaimState(item *Item, now time.Time) ClaimStatus {
	switch {
	case item.Claim == nil:
		return ClaimNone
	case now.Before(item.Claim.ExpiresAt):
		return ClaimActive
	default:
		return ClaimStale
	}
}

// CanClaim checks whether actor may claim item at time now (SPEC.md §4.3).
// It returns ErrClaimed when another actor holds an active claim, and a
// RuleError when the type cannot be claimed or the item cannot enter doing.
func CanClaim(s *Snapshot, item *Item, actor string, now time.Time) error {
	if !item.Type.IsClaimable() {
		return NewRuleError(fmt.Sprintf("cannot claim %s: only tasks and bugs can be claimed, %s is a %s", item.ID, item.ID, item.Type))
	}
	if ClaimState(item, now) == ClaimActive && item.Claim.Actor != actor {
		return wrapf(ErrClaimed, "%s is claimed by %s until %s", item.ID, item.Claim.Actor, item.Claim.ExpiresAt.UTC().Format(time.RFC3339))
	}
	if item.Status.Before(StatusDoing) {
		if unmet := CheckRequirements(s, item, StatusDoing); len(unmet) > 0 {
			return NewRuleError(fmt.Sprintf("cannot claim %s", item.ID), unmet...)
		}
	}
	return nil
}

// NextFilter restricts the candidates of `kb next`.
type NextFilter struct {
	Types  []Type // Empty means tasks and bugs; epic and story may be asked for.
	Label  string
	Parent string // Includes every descendant of Parent.
}

// Available returns the items `kb next` may hand out, best first
// (SPEC.md NEXT-1, NEXT-2): of a requested type (tasks and bugs by default),
// in todo, doing or review and not blocked; tasks and bugs also need no
// active claim, even one held by the caller, while stories and epics have no
// claim condition. Ordered by NEXT-3: status (review, doing, todo), priority, CBSP, story
// points (none last), position and natural ID.
func Available(s *Snapshot, f NextFilter, now time.Time) []*Item {
	idx := newWorkIndex(s)
	var scope map[string]bool
	if f.Parent != "" {
		scope = map[string]bool{}
		for _, id := range idx.subtree(f.Parent) {
			scope[id] = true
		}
	}
	var out []*Item
	for _, it := range s.Items() {
		switch {
		case len(f.Types) == 0 && !it.Type.IsClaimable(),
			len(f.Types) > 0 && !slices.Contains(f.Types, it.Type),
			f.Label != "" && !slices.Contains(it.Labels, f.Label),
			scope != nil && !scope[it.ID],
			!isWorkable(it.Status),
			it.Type.IsClaimable() && ClaimState(it, now) == ClaimActive:
			continue
		}
		if idx.isBlocked(it.ID) {
			continue
		}
		out = append(out, it)
	}
	// CBSP and points are computed once per candidate, not per comparison.
	crit := make(map[string]int, len(out))
	own := make(map[string]int, len(out))
	for _, it := range out {
		crit[it.ID] = idx.cbsp(it.ID)
		own[it.ID] = idx.storyPoints(it.ID)
	}
	slices.SortFunc(out, func(a, b *Item) int {
		if c := cmp.Compare(b.Status, a.Status); c != 0 { // review, doing, todo
			return c
		}
		if c := cmp.Compare(b.Priority, a.Priority); c != 0 {
			return c
		}
		if c := cmp.Compare(crit[b.ID], crit[a.ID]); c != 0 {
			return c
		}
		if c := cmp.Compare(own[b.ID], own[a.ID]); c != 0 { // no points sort last
			return c
		}
		return byPosition(a, b)
	})
	return out
}

// IsStale reports whether an item is stale: work in progress whose claim
// has expired, i.e. an agent took it and went silent. Items in review or
// done keep their last claim but are never stale.
func IsStale(item *Item, now time.Time) bool {
	return item.Status == StatusDoing && ClaimState(item, now) == ClaimStale
}

// isWorkable reports whether an item in status st may be proposed by
// `kb next`: backlog and done never are.
func isWorkable(st Status) bool {
	return st == StatusTodo || st == StatusDoing || st == StatusReview
}
