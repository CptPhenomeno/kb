package core

import "slices"

// DerivedMove is a status change computed by propagation rather than
// requested by an actor. It is recorded as a moved event with derived: true.
type DerivedMove struct {
	ID   string
	From Status
	To   Status
}

// IndicatedStatus is the status the children of an item point to
// (SPEC.md §4.6). The caller guarantees children is not empty.
func IndicatedStatus(children []*Item) Status {
	allDone, allReviewOrDone, anyStarted, anyTodo := true, true, false, false
	for _, c := range children {
		allDone = allDone && c.Status == StatusDone
		allReviewOrDone = allReviewOrDone && !c.Status.Before(StatusReview)
		anyStarted = anyStarted || !c.Status.Before(StatusDoing)
		anyTodo = anyTodo || c.Status == StatusTodo
	}
	switch {
	case allDone:
		return StatusDone
	case allReviewOrDone:
		return StatusReview
	case anyStarted:
		return StatusDoing
	case anyTodo:
		return StatusTodo
	default:
		return StatusBacklog
	}
}

// Propagate re-evaluates start and then its ancestors, and returns the
// derived moves to apply, nearest first. It stops at the first item whose
// status does not change. The snapshot is not modified.
//
// For every item with children:
//   - advance: if the children indicate a later status, the item moves to
//     the most advanced status up to the indicated one whose entry
//     requirements hold;
//   - regress: otherwise, if the item no longer meets the requirements of
//     its current status, it moves to the most advanced earlier status whose
//     entry requirements hold, or stays if there is none.
//
// Requirements are checked on entry only, so being blocked never makes an
// item leave doing (SPEC.md §4.7).
func Propagate(s *Snapshot, start string) []DerivedMove {
	sim := s.clone()
	var moves []DerivedMove
	for id := start; id != ""; {
		item := sim.Get(id)
		if item == nil {
			break
		}
		to, ok := nextStatus(sim, item)
		if !ok {
			break
		}
		moves = append(moves, DerivedMove{ID: id, From: item.Status, To: to})
		item.Status = to
		id = item.Parent
	}
	return moves
}

// nextStatus returns the status item should move to, if any.
func nextStatus(s *Snapshot, item *Item) (Status, bool) {
	children := s.Children(item.ID)
	if len(children) == 0 {
		return item.Status, false
	}
	if indicated := IndicatedStatus(children); item.Status.Before(indicated) {
		for st := indicated; item.Status.Before(st); st-- {
			if len(CheckRequirements(s, item, st)) == 0 {
				return st, true
			}
		}
		return item.Status, false
	}
	if stillValid(s, item) {
		return item.Status, false
	}
	for st := item.Status - 1; st >= StatusBacklog; st-- {
		if len(CheckRequirements(s, item, st)) == 0 {
			return st, true
		}
	}
	return item.Status, false
}

// stillValid reports whether item still meets the requirements of its
// current status. Being blocked is ignored: it only matters on entry.
func stillValid(s *Snapshot, item *Item) bool {
	if item.Status == StatusDoing {
		return true
	}
	return len(CheckRequirements(s, item, item.Status)) == 0
}

// clone returns a copy of the snapshot whose items can be mutated without
// affecting the original. Slices inside items are shared and must not be
// modified through the clone.
func (s *Snapshot) clone() *Snapshot {
	items := make([]*Item, 0, len(s.items))
	for _, it := range s.items {
		cp := *it
		items = append(items, &cp)
	}
	return NewSnapshot(items, slices.Clone(s.Relations))
}
