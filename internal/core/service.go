package core

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Tx is the transactional view of the board the service works on. The
// store package implements it on SQLite.
type Tx interface {
	NextID(prefix string) (string, error)
	NextPosition(status Status) (int, error)
	InsertItem(it *Item) error
	UpdateItem(it *Item, expected int) error
	DeleteItem(id string) error
	GetItem(id string) (*Item, error)
	GetItemByRef(ref string) (*Item, error)
	Snapshot() (*Snapshot, error)
	AddRelation(r Relation) error
	RemoveRelation(r Relation) error
	AppendEvent(e *Event) error
	EventsSince(afterID int64, limit int) ([]Event, error)
	EventsFor(item string) ([]Event, error)
}

// Store runs a function inside one atomic, serialized transaction.
type Store interface {
	Transact(ctx context.Context, fn func(Tx) error) error
}

// Service implements every operation of the board. Each write runs in a
// single transaction that checks the expected version, enforces the rules,
// renews the actor's claims, propagates status changes to parents and
// appends the events.
type Service struct {
	Store    Store
	Now      func() time.Time
	Prefix   string
	ClaimTTL time.Duration
}

// Patch lists the fields Edit changes; nil fields are left untouched.
type Patch struct {
	Title       *string
	Description *string
	Ref         *string
	Parent      *string // "" moves the item to the root.
	Priority    *Priority
	Points      *int
	Spec        *string
	Labels      *[]string
	Links       *[]string
}

// MoveResult is the outcome of a move: the item and the parents that moved
// with it.
type MoveResult struct {
	Item    *Item
	Derived []DerivedMove
}

// LogFilter selects events in Log. Zero fields do not filter.
type LogFilter struct {
	Item    string
	AfterID int64
	Since   time.Time
	Limit   int // Keep only the newest Limit events.
}

// op is the state of one write operation inside its transaction.
type op struct {
	s     *Service
	tx    Tx
	actor string
	now   time.Time
}

func (s *Service) write(ctx context.Context, actor string, fn func(*op) error) error {
	return s.Store.Transact(ctx, func(tx Tx) error {
		return fn(&op{s: s, tx: tx, actor: actor, now: s.Now().UTC()})
	})
}

// ---- reads ----

// Get returns an item by ID.
func (s *Service) Get(ctx context.Context, id string) (*Item, error) {
	var it *Item
	err := s.Store.Transact(ctx, func(tx Tx) (err error) { it, err = tx.GetItem(id); return err })
	return it, err
}

// GetByRef returns an item by ref.
func (s *Service) GetByRef(ctx context.Context, ref string) (*Item, error) {
	var it *Item
	err := s.Store.Transact(ctx, func(tx Tx) (err error) { it, err = tx.GetItemByRef(ref); return err })
	return it, err
}

// Board returns a snapshot of the whole board.
func (s *Service) Board(ctx context.Context) (*Snapshot, error) {
	var snap *Snapshot
	err := s.Store.Transact(ctx, func(tx Tx) (err error) { snap, err = tx.Snapshot(); return err })
	return snap, err
}

// History returns the events of one item, oldest first.
func (s *Service) History(ctx context.Context, id string) ([]Event, error) {
	var evs []Event
	err := s.Store.Transact(ctx, func(tx Tx) (err error) { evs, err = tx.EventsFor(id); return err })
	return evs, err
}

// Log returns the events matching f, oldest first.
func (s *Service) Log(ctx context.Context, f LogFilter) ([]Event, error) {
	var all []Event
	err := s.Store.Transact(ctx, func(tx Tx) (err error) { all, err = tx.EventsSince(f.AfterID, 0); return err })
	if err != nil {
		return nil, err
	}
	out := all[:0]
	for _, e := range all {
		if (f.Item == "" || e.Item == f.Item) && (f.Since.IsZero() || !e.At.Before(f.Since)) {
			out = append(out, e)
		}
	}
	if f.Limit > 0 && len(out) > f.Limit {
		out = out[len(out)-f.Limit:]
	}
	return out, nil
}

// Column returns the items of a column in board order.
func Column(s *Snapshot, status Status) []*Item {
	var out []*Item
	for _, it := range s.Items() {
		if it.Status == status {
			out = append(out, it)
		}
	}
	slices.SortFunc(out, byPosition)
	return out
}

// ---- items ----

// Create adds a new item. Status defaults to backlog; any other initial
// status must meet its entry requirements.
func (s *Service) Create(ctx context.Context, actor string, in Item) (*Item, error) {
	var out *Item
	err := s.write(ctx, actor, func(o *op) error {
		it := in
		it.Title = strings.TrimSpace(it.Title)
		it.Claim, it.Assignee, it.Version = nil, "", 0
		if err := ValidateItem(&it); err != nil {
			return err
		}
		snap, err := o.tx.Snapshot()
		if err != nil {
			return err
		}
		var parentType *Type
		if it.Parent != "" {
			p := snap.Get(it.Parent)
			if p == nil {
				return wrapf(ErrNotFound, "parent %s", it.Parent)
			}
			parentType = &p.Type
		}
		if err := ValidateParent(it.Type, parentType); err != nil {
			return err
		}
		if it.ID, err = o.tx.NextID(s.Prefix); err != nil {
			return err
		}
		// Requirements are checked against the board as it will be, so an
		// item under a blocked story cannot start in doing.
		if it.Status != StatusBacklog {
			probe := it
			trial := NewSnapshot(append(snap.Items(), &probe), snap.Relations)
			if unmet := CheckRequirements(trial, &probe, it.Status); len(unmet) > 0 {
				return NewRuleError(fmt.Sprintf("cannot create %s in %s", it.Type, it.Status), unmet...)
			}
		}
		if it.Position, err = o.tx.NextPosition(it.Status); err != nil {
			return err
		}
		it.CreatedAt, it.CreatedBy, it.UpdatedAt, it.UpdatedBy = o.now, actor, o.now, actor
		if err := o.tx.InsertItem(&it); err != nil {
			return err
		}
		if err := o.event(it.ID, EventCreated, map[string]any{"type": it.Type.String(), "title": it.Title, "status": it.Status.String()}); err != nil {
			return err
		}
		if err := o.renewClaims(it.Parent); err != nil {
			return err
		}
		if _, err := o.propagate(it.Parent); err != nil {
			return err
		}
		out, err = o.tx.GetItem(it.ID)
		return err
	})
	return out, err
}

// Edit changes the fields set in p.
func (s *Service) Edit(ctx context.Context, actor, id string, p Patch, ifVersion int) (*Item, error) {
	var out *Item
	err := s.write(ctx, actor, func(o *op) error {
		it, err := o.load(id, ifVersion)
		if err != nil {
			return err
		}
		before := *it
		changes := map[string]any{}
		setField(changes, "title", &it.Title, p.Title)
		setField(changes, "description", &it.Description, p.Description)
		setField(changes, "ref", &it.Ref, p.Ref)
		setField(changes, "spec", &it.Spec, p.Spec)
		setField(changes, "points", &it.Points, p.Points)
		if p.Priority != nil && *p.Priority != it.Priority {
			changes["priority"] = []any{it.Priority.String(), p.Priority.String()}
			it.Priority = *p.Priority
		}
		setList(changes, "labels", &it.Labels, p.Labels)
		setList(changes, "links", &it.Links, p.Links)
		it.Title = strings.TrimSpace(it.Title)
		if err := ValidateItem(it); err != nil {
			return err
		}
		if p.Parent != nil && *p.Parent != it.Parent {
			snap, err := o.tx.Snapshot()
			if err != nil {
				return err
			}
			if err := ValidateReparent(snap, id, *p.Parent); err != nil {
				return err
			}
			changes["parent"] = []any{it.Parent, *p.Parent}
			it.Parent = *p.Parent
		}
		if len(changes) == 0 {
			out = it
			return nil
		}
		if err := o.save(it); err != nil {
			return err
		}
		if err := o.event(id, EventUpdated, map[string]any{"changes": changes}); err != nil {
			return err
		}
		if err := o.afterWrite(it); err != nil {
			return err
		}
		if before.Parent != it.Parent {
			// The moved item itself does not change, so propagation from it
			// would stop at once: re-evaluate both parents explicitly.
			for _, p := range []string{before.Parent, it.Parent} {
				if _, err := o.propagate(p); err != nil {
					return err
				}
			}
		}
		out, err = o.tx.GetItem(id)
		return err
	})
	return out, err
}

// Move puts an item in another column, if it meets the column's entry
// requirements, and propagates the change to its parents. Moving to the
// current column is a no-op.
func (s *Service) Move(ctx context.Context, actor, id string, to Status, ifVersion int) (*MoveResult, error) {
	var res MoveResult
	err := s.write(ctx, actor, func(o *op) error {
		it, err := o.load(id, ifVersion)
		if err != nil {
			return err
		}
		if it.Status == to {
			res.Item = it
			return nil
		}
		snap, err := o.tx.Snapshot()
		if err != nil {
			return err
		}
		if unmet := CheckRequirements(snap, snap.Get(id), to); len(unmet) > 0 {
			return NewRuleError(fmt.Sprintf("cannot move %s to %s", id, to), unmet...)
		}
		if err := o.moveTo(it, to, false); err != nil {
			return err
		}
		if err := o.renewClaims(it.Parent); err != nil {
			return err
		}
		if res.Derived, err = o.propagate(it.Parent); err != nil {
			return err
		}
		res.Item, err = o.tx.GetItem(id)
		return err
	})
	if err != nil {
		return nil, err
	}
	return &res, nil
}

// ArrangeColumn writes the order of a whole column: ids must list every
// item of the column exactly once, top first. Only positions change: no
// updated_at and no events, since the content of the items is untouched.
// The TUI uses it to fix a sorted column before a manual reorder.
func (s *Service) ArrangeColumn(ctx context.Context, actor string, status Status, ids []string) error {
	return s.write(ctx, actor, func(o *op) error {
		snap, err := o.tx.Snapshot()
		if err != nil {
			return err
		}
		col := Column(snap, status)
		want := map[string]bool{}
		for _, it := range col {
			want[it.ID] = true
		}
		seen := map[string]bool{}
		for _, id := range ids {
			if !want[id] || seen[id] {
				return NewRuleError(fmt.Sprintf("%s is not in column %s, or is listed twice", id, status))
			}
			seen[id] = true
		}
		if len(ids) != len(col) {
			return NewRuleError(fmt.Sprintf("column %s has %d items, %d given", status, len(col), len(ids)))
		}
		for pos, id := range ids {
			if snap.Get(id).Position == pos {
				continue
			}
			cur, err := o.tx.GetItem(id)
			if err != nil {
				return err
			}
			cur.Position = pos
			if err := o.tx.UpdateItem(cur, cur.Version); err != nil {
				return err
			}
		}
		return nil
	})
}

// Reorder moves an item to the given index of its column (0 is the top).
// Reordering is always allowed.
func (s *Service) Reorder(ctx context.Context, actor, id string, index int, ifVersion int) (*Item, error) {
	var out *Item
	err := s.write(ctx, actor, func(o *op) error {
		it, err := o.load(id, ifVersion)
		if err != nil {
			return err
		}
		snap, err := o.tx.Snapshot()
		if err != nil {
			return err
		}
		col := slices.DeleteFunc(Column(snap, it.Status), func(x *Item) bool { return x.ID == id })
		index = max(0, min(index, len(col)))
		col = slices.Insert(col, index, snap.Get(id))
		for pos, x := range col {
			if x.Position == pos {
				continue
			}
			cur, err := o.tx.GetItem(x.ID)
			if err != nil {
				return err
			}
			cur.Position = pos
			if x.ID == id {
				if err := o.save(cur); err != nil {
					return err
				}
				continue
			}
			if err := o.tx.UpdateItem(cur, cur.Version); err != nil {
				return err
			}
		}
		if err := o.event(id, EventUpdated, map[string]any{"changes": map[string]any{"position": []any{it.Position, index}}}); err != nil {
			return err
		}
		if err := o.renewClaims(id); err != nil {
			return err
		}
		out, err = o.tx.GetItem(id)
		return err
	})
	return out, err
}

// Remove deletes an item; with recursive it also deletes its descendants,
// deepest first. It returns the deleted IDs in deletion order.
func (s *Service) Remove(ctx context.Context, actor, id string, recursive bool) ([]string, error) {
	var removed []string
	err := s.write(ctx, actor, func(o *op) error {
		snap, err := o.tx.Snapshot()
		if err != nil {
			return err
		}
		it := snap.Get(id)
		if it == nil {
			return wrapf(ErrNotFound, "item %s", id)
		}
		desc := snap.Descendants(id)
		if len(desc) > 0 && !recursive {
			return NewRuleError(fmt.Sprintf("%s has %d descendants: remove them first or use --recursive", id, len(desc)))
		}
		slices.Reverse(desc)
		for _, x := range append(desc, it) {
			if err := o.tx.DeleteItem(x.ID); err != nil {
				return err
			}
			if err := o.event(x.ID, EventDeleted, map[string]any{"title": x.Title, "type": x.Type.String()}); err != nil {
				return err
			}
			removed = append(removed, x.ID)
		}
		if err := o.renewClaims(it.Parent); err != nil {
			return err
		}
		_, err = o.propagate(it.Parent)
		return err
	})
	return removed, err
}

// ---- relations ----

// Link adds a relation. Adding an existing relation is a no-op.
func (s *Service) Link(ctx context.Context, actor string, r Relation) error {
	return s.write(ctx, actor, func(o *op) error {
		snap, err := o.tx.Snapshot()
		if err != nil {
			return err
		}
		if err := ValidateLink(snap, r); err != nil {
			return err
		}
		if HasRelation(snap.Relations, r) {
			return nil
		}
		if err := o.tx.AddRelation(r); err != nil {
			return err
		}
		return o.relationEvents(EventLinked, r)
	})
}

// Unlink removes a relation.
func (s *Service) Unlink(ctx context.Context, actor string, r Relation) error {
	return s.write(ctx, actor, func(o *op) error {
		if err := o.tx.RemoveRelation(r); err != nil {
			return err
		}
		return o.relationEvents(EventUnlinked, r)
	})
}

func (o *op) relationEvents(kind EventKind, r Relation) error {
	data := map[string]any{"kind": r.Kind.String(), "from": r.From, "to": r.To}
	for _, id := range []string{r.From, r.To} {
		if err := o.event(id, kind, data); err != nil {
			return err
		}
		if err := o.renewClaims(id); err != nil {
			return err
		}
	}
	return nil
}

// ---- claims ----

// Claim gives the item to actor for ClaimTTL, moving it to doing if it was
// in backlog or todo (SPEC.md §4.3).
func (s *Service) Claim(ctx context.Context, actor, id string, ifVersion int) (*Item, error) {
	var out *Item
	err := s.write(ctx, actor, func(o *op) (err error) {
		if _, err := o.load(id, ifVersion); err != nil {
			return err
		}
		out, err = o.claim(id)
		return err
	})
	return out, err
}

func (o *op) claim(id string) (*Item, error) {
	snap, err := o.tx.Snapshot()
	if err != nil {
		return nil, err
	}
	it := snap.Get(id)
	if it == nil {
		return nil, wrapf(ErrNotFound, "item %s", id)
	}
	if err := CanClaim(snap, it, o.actor, o.now); err != nil {
		return nil, err
	}
	if ClaimState(it, o.now) == ClaimStale && it.Claim.Actor != o.actor {
		if err := o.event(id, EventClaimExpired, map[string]any{"actor": it.Claim.Actor, "expired_at": it.Claim.ExpiresAt.Format(time.RFC3339)}); err != nil {
			return nil, err
		}
	}
	it.Claim = &Claim{Actor: o.actor, ExpiresAt: o.now.Add(o.s.ClaimTTL)}
	it.Assignee = o.actor
	if it.Status.Before(StatusDoing) {
		if err := o.moveTo(it, StatusDoing, false); err != nil {
			return nil, err
		}
	} else if err := o.save(it); err != nil {
		return nil, err
	}
	if err := o.event(id, EventClaimed, map[string]any{"expires_at": it.Claim.ExpiresAt.Format(time.RFC3339)}); err != nil {
		return nil, err
	}
	if _, err := o.propagate(it.Parent); err != nil {
		return nil, err
	}
	return o.tx.GetItem(id)
}

// Release drops the claim and moves the item back to todo when its
// requirements allow it; otherwise it stays in doing, free for kb next.
// An agent cannot release another agent's active claim; people can.
func (s *Service) Release(ctx context.Context, actor, id string, ifVersion int) (*Item, error) {
	var out *Item
	err := s.write(ctx, actor, func(o *op) error {
		it, err := o.load(id, ifVersion)
		if err != nil {
			return err
		}
		if it.Claim == nil {
			return NewRuleError(fmt.Sprintf("%s is not claimed", id))
		}
		if ClaimState(it, o.now) == ClaimActive && it.Claim.Actor != actor && !IsHumanActor(actor) {
			return wrapf(ErrClaimed, "%s is claimed by %s", id, it.Claim.Actor)
		}
		holder := it.Claim.Actor
		it.Claim, it.Assignee = nil, ""
		snap, err := o.tx.Snapshot()
		if err != nil {
			return err
		}
		if it.Status == StatusDoing && len(CheckRequirements(snap, snap.Get(id), StatusTodo)) == 0 {
			if err := o.moveTo(it, StatusTodo, false); err != nil {
				return err
			}
		} else if err := o.save(it); err != nil {
			return err
		}
		if err := o.event(id, EventReleased, map[string]any{"actor": holder}); err != nil {
			return err
		}
		if _, err := o.propagate(it.Parent); err != nil {
			return err
		}
		out, err = o.tx.GetItem(id)
		return err
	})
	return out, err
}

// Touch renews the actor's claim on the item.
func (s *Service) Touch(ctx context.Context, actor, id string) (*Item, error) {
	var out *Item
	err := s.write(ctx, actor, func(o *op) error {
		it, err := o.tx.GetItem(id)
		if err != nil {
			return err
		}
		if it.Claim == nil {
			return NewRuleError(fmt.Sprintf("%s is not claimed", id))
		}
		if it.Claim.Actor != actor {
			return wrapf(ErrClaimed, "%s is claimed by %s", id, it.Claim.Actor)
		}
		if err := o.renewClaims(id); err != nil {
			return err
		}
		out, err = o.tx.GetItem(id)
		return err
	})
	return out, err
}

// Next returns the best available item (SPEC.md §4.5); with claim it also
// claims it in the same transaction, so concurrent agents never get the
// same item. It returns ErrNotFound when nothing is available.
func (s *Service) Next(ctx context.Context, actor string, f NextFilter, claim bool) (*Item, error) {
	var out *Item
	err := s.write(ctx, actor, func(o *op) error {
		snap, err := o.tx.Snapshot()
		if err != nil {
			return err
		}
		avail := Available(snap, f, o.now)
		if len(avail) == 0 {
			return wrapf(ErrNotFound, "no available item")
		}
		if !claim {
			out = avail[0]
			return nil
		}
		out, err = o.claim(avail[0].ID)
		return err
	})
	return out, err
}

// ---- TDD phase, tests, acceptance, notes ----

// SetPhase sets the TDD phase of a task, subtask or bug.
func (s *Service) SetPhase(ctx context.Context, actor, id string, phase TDDPhase, ifVersion int) (*Item, error) {
	return s.update(ctx, actor, id, ifVersion, func(it *Item) (EventKind, map[string]any, error) {
		if !it.Type.IsLeafType() {
			return "", nil, NewRuleError(fmt.Sprintf("tdd phase is not allowed on a %s", it.Type))
		}
		from := it.Phase
		it.Phase = phase
		return EventPhase, map[string]any{"from": from.String(), "to": phase.String()}, nil
	})
}

// AddTest appends a pending test.
func (s *Service) AddTest(ctx context.Context, actor, id, name string, ifVersion int) (*Item, error) {
	return s.update(ctx, actor, id, ifVersion, func(it *Item) (EventKind, map[string]any, error) {
		name = strings.TrimSpace(name)
		it.Tests = append(it.Tests, Test{Name: name})
		return EventTest, map[string]any{"name": name, "state": TestPending.String(), "added": true}, nil
	})
}

// SetTestState sets the state of the test selected by its 1-based index or
// by its name.
func (s *Service) SetTestState(ctx context.Context, actor, id, selector string, state TestState, ifVersion int) (*Item, error) {
	return s.update(ctx, actor, id, ifVersion, func(it *Item) (EventKind, map[string]any, error) {
		i := -1
		if n, err := strconv.Atoi(selector); err == nil && n >= 1 && n <= len(it.Tests) {
			i = n - 1
		} else {
			i = slices.IndexFunc(it.Tests, func(t Test) bool { return t.Name == selector })
		}
		if i < 0 {
			return "", nil, wrapf(ErrNotFound, "test %q of %s", selector, id)
		}
		it.Tests[i].State = state
		return EventTest, map[string]any{"name": it.Tests[i].Name, "state": state.String()}, nil
	})
}

// AddCriterion appends an unchecked acceptance criterion.
func (s *Service) AddCriterion(ctx context.Context, actor, id, text string, ifVersion int) (*Item, error) {
	return s.update(ctx, actor, id, ifVersion, func(it *Item) (EventKind, map[string]any, error) {
		text = strings.TrimSpace(text)
		it.Acceptance = append(it.Acceptance, Criterion{Text: text})
		return EventAcceptance, map[string]any{"text": text, "done": false, "added": true}, nil
	})
}

// SetCriterion checks or unchecks the n-th (1-based) acceptance criterion.
func (s *Service) SetCriterion(ctx context.Context, actor, id string, n int, done bool, ifVersion int) (*Item, error) {
	return s.update(ctx, actor, id, ifVersion, func(it *Item) (EventKind, map[string]any, error) {
		if n < 1 || n > len(it.Acceptance) {
			return "", nil, wrapf(ErrNotFound, "acceptance criterion %d of %s", n, id)
		}
		it.Acceptance[n-1].Done = done
		return EventAcceptance, map[string]any{"text": it.Acceptance[n-1].Text, "done": done}, nil
	})
}

// update applies change to an item, validates it, saves it, records the
// event and re-evaluates the item and its parents: tests and criteria are
// requirements, so changing them can move the item's parents, or the item
// itself when it has children.
func (s *Service) update(ctx context.Context, actor, id string, ifVersion int,
	change func(*Item) (EventKind, map[string]any, error)) (*Item, error) {
	var out *Item
	err := s.write(ctx, actor, func(o *op) error {
		it, err := o.load(id, ifVersion)
		if err != nil {
			return err
		}
		kind, data, err := change(it)
		if err != nil {
			return err
		}
		if err := ValidateItem(it); err != nil {
			return err
		}
		if err := o.save(it); err != nil {
			return err
		}
		if err := o.event(id, kind, data); err != nil {
			return err
		}
		if err := o.afterWrite(it); err != nil {
			return err
		}
		out, err = o.tx.GetItem(id)
		return err
	})
	return out, err
}

// Note records a free-text note in the item's history.
func (s *Service) Note(ctx context.Context, actor, id, text string) error {
	text = strings.TrimSpace(text)
	if text == "" {
		return NewRuleError("note text is required")
	}
	return s.write(ctx, actor, func(o *op) error {
		if _, err := o.tx.GetItem(id); err != nil {
			return err
		}
		if err := o.event(id, EventNote, map[string]any{"text": text}); err != nil {
			return err
		}
		return o.renewClaims(id)
	})
}

// ---- helpers inside a transaction ----

// load reads an item and checks the expected version (0 skips the check).
func (o *op) load(id string, ifVersion int) (*Item, error) {
	it, err := o.tx.GetItem(id)
	if err != nil {
		return nil, err
	}
	if ifVersion != 0 && it.Version != ifVersion {
		return nil, wrapf(ErrConflict, "%s is at version %d, not %d", id, it.Version, ifVersion)
	}
	return it, nil
}

// save stores an item written by the actor, renewing the actor's claim on it.
func (o *op) save(it *Item) error {
	it.UpdatedAt, it.UpdatedBy = o.now, o.actor
	o.renew(it)
	return o.tx.UpdateItem(it, it.Version)
}

// renew extends the item's claim if the actor holds it.
func (o *op) renew(it *Item) {
	if it.Claim != nil && it.Claim.Actor == o.actor {
		it.Claim.ExpiresAt = o.now.Add(o.s.ClaimTTL)
	}
}

// renewClaims renews the actor's claim on id and on its ancestors (a write
// on a subtask renews the claim on its task).
func (o *op) renewClaims(id string) error {
	for id != "" {
		it, err := o.tx.GetItem(id)
		if err != nil {
			return err
		}
		if it.Claim != nil && it.Claim.Actor == o.actor {
			o.renew(it)
			if err := o.tx.UpdateItem(it, it.Version); err != nil {
				return err
			}
		}
		id = it.Parent
	}
	return nil
}

// afterWrite renews the ancestors' claims and propagates from the item
// itself, which covers both its own children-driven status and its parents.
func (o *op) afterWrite(it *Item) error {
	if err := o.renewClaims(it.Parent); err != nil {
		return err
	}
	_, err := o.propagate(it.ID)
	return err
}

// moveTo puts it at the bottom of column to, saves it and records the move.
func (o *op) moveTo(it *Item, to Status, derived bool) error {
	from := it.Status
	pos, err := o.tx.NextPosition(to)
	if err != nil {
		return err
	}
	it.Status, it.Position = to, pos
	if derived {
		it.UpdatedAt, it.UpdatedBy = o.now, o.actor
		err = o.tx.UpdateItem(it, it.Version)
	} else {
		err = o.save(it)
	}
	if err != nil {
		return err
	}
	data := map[string]any{"from": from.String(), "to": to.String()}
	if derived {
		data["derived"] = true
	}
	return o.event(it.ID, EventMoved, data)
}

// propagate applies the derived moves starting at id and returns them.
func (o *op) propagate(id string) ([]DerivedMove, error) {
	if id == "" {
		return nil, nil
	}
	snap, err := o.tx.Snapshot()
	if err != nil {
		return nil, err
	}
	moves := Propagate(snap, id)
	for _, m := range moves {
		it, err := o.tx.GetItem(m.ID)
		if err != nil {
			return nil, err
		}
		if err := o.moveTo(it, m.To, true); err != nil {
			return nil, err
		}
	}
	return moves, nil
}

func (o *op) event(item string, kind EventKind, data map[string]any) error {
	return o.tx.AppendEvent(&Event{At: o.now, Actor: o.actor, Item: item, Kind: kind, Data: data})
}

func setField[T comparable](changes map[string]any, name string, field *T, value *T) {
	if value != nil && *value != *field {
		changes[name] = []any{*field, *value}
		*field = *value
	}
}

func setList(changes map[string]any, name string, field *[]string, value *[]string) {
	if value != nil && !slices.Equal(*field, *value) {
		changes[name] = []any{*field, *value}
		*field = slices.Clone(*value)
	}
}
