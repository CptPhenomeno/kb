package store

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/CptPhenomeno/kb/internal/core"
)

var t0 = time.Date(2026, 11, 9, 14, 0, 0, 0, time.UTC)

// inTx runs fn in a transaction and fails the test on error.
func inTx(t *testing.T, db *DB, fn func(tx *Tx) error) {
	t.Helper()
	if err := db.Tx(context.Background(), fn); err != nil {
		t.Fatalf("transaction failed: %v", err)
	}
}

// newItem inserts an item with a generated ID and returns it.
func newItem(t *testing.T, db *DB, typ core.Type, title, parent string) *core.Item {
	t.Helper()
	item := &core.Item{Type: typ, Title: title, Parent: parent, Status: core.StatusBacklog,
		CreatedAt: t0, CreatedBy: "@dev", UpdatedAt: t0, UpdatedBy: "@dev"}
	inTx(t, db, func(tx *Tx) error {
		id, err := tx.NextID("KB")
		if err != nil {
			return err
		}
		item.ID = id
		return tx.InsertItem(item)
	})
	return item
}

func TestInsertAssignsSequentialIDs(t *testing.T) {
	db := openTemp(t)
	for i, want := range []string{"KB-1", "KB-2", "KB-3"} {
		got := newItem(t, db, core.TypeTask, "t", "")
		if got.ID != want {
			t.Fatalf("item %d: ID = %s, want %s", i, got.ID, want)
		}
		if got.Version != 1 {
			t.Fatalf("new item version = %d, want 1", got.Version)
		}
	}
}

func TestIDsNotReusedAfterDelete(t *testing.T) {
	db := openTemp(t)
	newItem(t, db, core.TypeTask, "a", "")
	b := newItem(t, db, core.TypeTask, "b", "")
	inTx(t, db, func(tx *Tx) error { return tx.DeleteItem(b.ID) })
	if c := newItem(t, db, core.TypeTask, "c", ""); c.ID != "KB-3" {
		t.Fatalf("ID after delete = %s, want KB-3", c.ID)
	}
}

func TestItemRoundTrip(t *testing.T) {
	db := openTemp(t)
	story := newItem(t, db, core.TypeStory, "Token login", "")
	full := &core.Item{
		Ref: "auth-login-refresh", Type: core.TypeTask, Title: "Token refresh",
		Description: "Exchange a refresh token.\n\nMarkdown **body**.", Parent: story.ID,
		Status: core.StatusDoing, Priority: core.PriorityHigh, Points: 2, Spec: "SPEC.md#refresh",
		Labels: []string{"api", "auth"}, Links: []string{"PR #148"},
		Acceptance: []core.Criterion{{Text: "new pair", Done: true}, {Text: "reuse → 401"}},
		Tests:      []core.Test{{Name: "rotates", State: core.TestFailing}, {Name: "revokes"}},
		Phase:      core.PhaseRed, Assignee: "agent-auth",
		Claim:    &core.Claim{Actor: "agent-auth", ExpiresAt: t0.Add(15 * time.Minute)},
		Position: 4, CreatedAt: t0, CreatedBy: "agent-plan", UpdatedAt: t0.Add(time.Second), UpdatedBy: "agent-auth",
	}
	inTx(t, db, func(tx *Tx) error {
		full.ID = "KB-99"
		return tx.InsertItem(full)
	})
	var got *core.Item
	inTx(t, db, func(tx *Tx) (err error) { got, err = tx.GetItem("KB-99"); return err })
	if !reflect.DeepEqual(got, full) {
		t.Fatalf("round trip mismatch:\n got %+v\nwant %+v", got, full)
	}
	var byRef *core.Item
	inTx(t, db, func(tx *Tx) (err error) { byRef, err = tx.GetItemByRef("auth-login-refresh"); return err })
	if byRef.ID != "KB-99" {
		t.Fatalf("GetItemByRef = %s, want KB-99", byRef.ID)
	}
}

func TestGetNotFound(t *testing.T) {
	db := openTemp(t)
	err := db.Tx(context.Background(), func(tx *Tx) error {
		_, err := tx.GetItem("KB-404")
		return err
	})
	if !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
	err = db.Tx(context.Background(), func(tx *Tx) error {
		_, err := tx.GetItemByRef("nope")
		return err
	})
	if !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("by ref: err = %v, want ErrNotFound", err)
	}
	if err := db.Tx(context.Background(), func(tx *Tx) error { return tx.DeleteItem("KB-404") }); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("delete: err = %v, want ErrNotFound", err)
	}
}

func TestUpdateIncrementsVersion(t *testing.T) {
	db := openTemp(t)
	item := newItem(t, db, core.TypeTask, "a", "")
	item.Title = "renamed"
	item.Status = core.StatusTodo
	inTx(t, db, func(tx *Tx) error { return tx.UpdateItem(item, 1) })
	if item.Version != 2 {
		t.Fatalf("version after update = %d, want 2", item.Version)
	}
	var got *core.Item
	inTx(t, db, func(tx *Tx) (err error) { got, err = tx.GetItem(item.ID); return err })
	if got.Title != "renamed" || got.Status != core.StatusTodo || got.Version != 2 {
		t.Fatalf("stored item = %+v", got)
	}
}

func TestUpdateVersionConflict(t *testing.T) {
	db := openTemp(t)
	item := newItem(t, db, core.TypeTask, "a", "")
	err := db.Tx(context.Background(), func(tx *Tx) error { return tx.UpdateItem(item, 7) })
	if !errors.Is(err, core.ErrConflict) {
		t.Fatalf("err = %v, want ErrConflict", err)
	}
	ghost := &core.Item{ID: "KB-404", Type: core.TypeTask, Title: "x"}
	err = db.Tx(context.Background(), func(tx *Tx) error { return tx.UpdateItem(ghost, 1) })
	if !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("missing item: err = %v, want ErrNotFound", err)
	}
}

func TestListFilters(t *testing.T) {
	db := openTemp(t)
	s := newItem(t, db, core.TypeStory, "s", "")
	a := newItem(t, db, core.TypeTask, "a", s.ID)
	b := newItem(t, db, core.TypeBug, "b", s.ID)
	c := newItem(t, db, core.TypeTask, "c", "")
	a.Status, a.Assignee, a.Labels = core.StatusDoing, "agent-1", []string{"api"}
	c.Labels = []string{"docs", "api"}
	inTx(t, db, func(tx *Tx) error {
		if err := tx.UpdateItem(a, 1); err != nil {
			return err
		}
		return tx.UpdateItem(c, 1)
	})
	doing, task, bug := core.StatusDoing, core.TypeTask, core.TypeBug
	cases := []struct {
		name string
		f    ItemFilter
		want []string
	}{
		{"all", ItemFilter{}, []string{s.ID, a.ID, b.ID, c.ID}},
		{"status", ItemFilter{Status: &doing}, []string{a.ID}},
		{"type", ItemFilter{Type: &task}, []string{a.ID, c.ID}},
		{"type bug", ItemFilter{Type: &bug}, []string{b.ID}},
		{"assignee", ItemFilter{Assignee: "agent-1"}, []string{a.ID}},
		{"label", ItemFilter{Label: "api"}, []string{a.ID, c.ID}},
		{"parent", ItemFilter{Parent: s.ID}, []string{a.ID, b.ID}},
	}
	for _, tc := range cases {
		var got []*core.Item
		inTx(t, db, func(tx *Tx) (err error) { got, err = tx.ListItems(tc.f); return err })
		var ids []string
		for _, it := range got {
			ids = append(ids, it.ID)
		}
		if !reflect.DeepEqual(ids, tc.want) {
			t.Errorf("%s: got %v, want %v", tc.name, ids, tc.want)
		}
	}
}

func TestSnapshotContainsAncestorsAndDescendants(t *testing.T) {
	db := openTemp(t)
	e := newItem(t, db, core.TypeEpic, "e", "")
	s := newItem(t, db, core.TypeStory, "s", e.ID)
	tk := newItem(t, db, core.TypeTask, "t", s.ID)
	other := newItem(t, db, core.TypeTask, "o", "")
	inTx(t, db, func(tx *Tx) error {
		return tx.AddRelation(core.Relation{Kind: core.RelBlocks, From: other.ID, To: s.ID})
	})
	var snap *core.Snapshot
	inTx(t, db, func(tx *Tx) (err error) { snap, err = tx.Snapshot(); return err })
	if got := len(snap.Ancestors(tk.ID)); got != 2 {
		t.Errorf("ancestors of task = %d, want 2", got)
	}
	if got := len(snap.Descendants(e.ID)); got != 2 {
		t.Errorf("descendants of epic = %d, want 2", got)
	}
	if blocked, _ := core.Blockers(snap, tk.ID); !blocked {
		t.Error("task must be blocked through its story")
	}
}

func TestRefUnique(t *testing.T) {
	db := openTemp(t)
	err := db.Tx(context.Background(), func(tx *Tx) error {
		for _, id := range []string{"KB-1", "KB-2"} {
			if err := tx.InsertItem(&core.Item{ID: id, Ref: "same", Type: core.TypeTask, Title: "x"}); err != nil {
				return err
			}
		}
		return nil
	})
	if !errors.Is(err, core.ErrRule) {
		t.Fatalf("duplicate ref: err = %v, want ErrRule", err)
	}
	// Items without a ref never collide.
	inTx(t, db, func(tx *Tx) error {
		for _, id := range []string{"KB-3", "KB-4"} {
			if err := tx.InsertItem(&core.Item{ID: id, Type: core.TypeTask, Title: "x"}); err != nil {
				return err
			}
		}
		return nil
	})
}

func TestPositionAppendAndReorder(t *testing.T) {
	db := openTemp(t)
	var p int
	inTx(t, db, func(tx *Tx) (err error) { p, err = tx.NextPosition(core.StatusTodo); return err })
	if p != 0 {
		t.Fatalf("first position in empty column = %d, want 0", p)
	}
	a := newItem(t, db, core.TypeTask, "a", "")
	a.Status, a.Position = core.StatusTodo, 0
	inTx(t, db, func(tx *Tx) error { return tx.UpdateItem(a, 1) })
	inTx(t, db, func(tx *Tx) (err error) { p, err = tx.NextPosition(core.StatusTodo); return err })
	if p != 1 {
		t.Fatalf("next position = %d, want 1", p)
	}
	inTx(t, db, func(tx *Tx) (err error) { p, err = tx.NextPosition(core.StatusDone); return err })
	if p != 0 {
		t.Fatalf("positions are per column: done = %d, want 0", p)
	}
}

func TestRelationsAddRemove(t *testing.T) {
	db := openTemp(t)
	a := newItem(t, db, core.TypeTask, "a", "")
	b := newItem(t, db, core.TypeTask, "b", "")
	rel := core.Relation{Kind: core.RelRelates, From: a.ID, To: b.ID}
	inTx(t, db, func(tx *Tx) error {
		if err := tx.AddRelation(rel); err != nil {
			return err
		}
		return tx.AddRelation(rel) // idempotent
	})
	var rels []core.Relation
	inTx(t, db, func(tx *Tx) (err error) { rels, err = tx.Relations(); return err })
	if len(rels) != 1 {
		t.Fatalf("relations = %v, want one", rels)
	}
	// relates is symmetric: removing B→A removes A→B.
	inTx(t, db, func(tx *Tx) error {
		return tx.RemoveRelation(core.Relation{Kind: core.RelRelates, From: b.ID, To: a.ID})
	})
	err := db.Tx(context.Background(), func(tx *Tx) error { return tx.RemoveRelation(rel) })
	if !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("remove missing relation: err = %v, want ErrNotFound", err)
	}
	// Deleting an item drops its relations.
	inTx(t, db, func(tx *Tx) error {
		return tx.AddRelation(core.Relation{Kind: core.RelBlocks, From: a.ID, To: b.ID})
	})
	inTx(t, db, func(tx *Tx) error { return tx.DeleteItem(a.ID) })
	inTx(t, db, func(tx *Tx) (err error) { rels, err = tx.Relations(); return err })
	if len(rels) != 0 {
		t.Fatalf("relations after delete = %v, want none", rels)
	}
}
