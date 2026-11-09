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
	Types  []Type // Empty means tasks and bugs.
	Label  string
	Parent string // Includes every descendant of Parent.
}

// Available returns the items `kb next` may hand out, best first
// (SPEC.md §4.5): claimable, in todo or doing, without an active claim and
// not blocked; ordered by priority, position and ID.
func Available(s *Snapshot, f NextFilter, now time.Time) []*Item {
	var scope map[string]bool
	if f.Parent != "" {
		scope = map[string]bool{}
		for _, d := range s.Descendants(f.Parent) {
			scope[d.ID] = true
		}
	}
	var out []*Item
	for _, it := range s.Items() {
		switch {
		case !it.Type.IsClaimable(),
			len(f.Types) > 0 && !slices.Contains(f.Types, it.Type),
			f.Label != "" && !slices.Contains(it.Labels, f.Label),
			scope != nil && !scope[it.ID],
			it.Status != StatusTodo && it.Status != StatusDoing,
			ClaimState(it, now) == ClaimActive:
			continue
		}
		if blocked, _ := Blockers(s, it.ID); blocked {
			continue
		}
		out = append(out, it)
	}
	slices.SortFunc(out, func(a, b *Item) int {
		if c := cmp.Compare(b.Priority, a.Priority); c != 0 {
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
