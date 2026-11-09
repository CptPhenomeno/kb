package core_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/CptPhenomeno/kb/internal/core"
	"github.com/CptPhenomeno/kb/internal/store"
)

const ttl = 15 * time.Minute

var ctx = context.Background()

// harness is a service backed by a real SQLite database and a fake clock.
type harness struct {
	t   *testing.T
	db  *store.DB
	svc *core.Service
	now time.Time
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	path := filepath.Join(t.TempDir(), "kb.db")
	db, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	h := &harness{t: t, db: db, now: time.Date(2026, 11, 9, 14, 0, 0, 0, time.UTC)}
	h.svc = h.service(db)
	return h
}

// service returns a service sharing the harness clock over db.
func (h *harness) service(db *store.DB) *core.Service {
	return &core.Service{Store: db, Now: func() time.Time { return h.now }, Prefix: "KB", ClaimTTL: ttl}
}

func (h *harness) advance(d time.Duration) { h.now = h.now.Add(d) }

// add creates an item and fails the test on error.
func (h *harness) add(actor string, typ core.Type, title, parent string, opts ...func(*core.Item)) *core.Item {
	h.t.Helper()
	it := core.Item{Type: typ, Title: title, Parent: parent}
	for _, o := range opts {
		o(&it)
	}
	created, err := h.svc.Create(ctx, actor, it)
	if err != nil {
		h.t.Fatalf("Create %q: %v", title, err)
	}
	return created
}

func withStatus(s core.Status) func(*core.Item) { return func(it *core.Item) { it.Status = s } }
func withRef(r string) func(*core.Item)         { return func(it *core.Item) { it.Ref = r } }
func withTests(names ...string) func(*core.Item) {
	return func(it *core.Item) {
		for _, n := range names {
			it.Tests = append(it.Tests, core.Test{Name: n})
		}
	}
}
func withCriteria(texts ...string) func(*core.Item) {
	return func(it *core.Item) {
		for _, x := range texts {
			it.Acceptance = append(it.Acceptance, core.Criterion{Text: x})
		}
	}
}

func (h *harness) get(id string) *core.Item {
	h.t.Helper()
	it, err := h.svc.Get(ctx, id)
	if err != nil {
		h.t.Fatalf("Get %s: %v", id, err)
	}
	return it
}

func (h *harness) status(id string) core.Status { h.t.Helper(); return h.get(id).Status }

func (h *harness) history(id string) []core.Event {
	h.t.Helper()
	evs, err := h.svc.History(ctx, id)
	if err != nil {
		h.t.Fatalf("History %s: %v", id, err)
	}
	return evs
}

// mustMove moves an item and fails the test on error.
func (h *harness) mustMove(actor, id string, to core.Status) *core.MoveResult {
	h.t.Helper()
	res, err := h.svc.Move(ctx, actor, id, to, 0)
	if err != nil {
		h.t.Fatalf("Move %s → %s: %v", id, to, err)
	}
	return res
}

func assertStatus(t *testing.T, h *harness, id string, want core.Status) {
	t.Helper()
	if got := h.status(id); got != want {
		t.Fatalf("%s is %s, want %s", id, got, want)
	}
}

func assertErr(t *testing.T, err, want error) {
	t.Helper()
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want %v", err, want)
	}
}

// unmetOf returns the unmet requirements of a rule error.
func unmetOf(t *testing.T, err error) []string {
	t.Helper()
	var re *core.RuleError
	if !errors.As(err, &re) {
		t.Fatalf("err = %v, want a RuleError", err)
	}
	return re.Unmet
}

// lastEvent returns the most recent event of an item.
func lastEvent(t *testing.T, h *harness, id string) core.Event {
	t.Helper()
	evs := h.history(id)
	if len(evs) == 0 {
		t.Fatalf("%s has no events", id)
	}
	return evs[len(evs)-1]
}
