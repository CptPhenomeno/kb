package store

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/CptPhenomeno/kb/internal/core"
)

func appendEvent(t *testing.T, db *DB, item string, kind core.EventKind) *core.Event {
	t.Helper()
	e := &core.Event{At: t0, Actor: "agent-1", Item: item, Kind: kind, Data: map[string]any{"to": "doing"}}
	inTx(t, db, func(tx *Tx) error { return tx.AppendEvent(e) })
	return e
}

func TestEventAppendedInSameTx(t *testing.T) {
	db := openTemp(t)
	boom := errors.New("boom")
	err := db.Tx(context.Background(), func(tx *Tx) error {
		if err := tx.AppendEvent(&core.Event{At: t0, Actor: "a", Item: "KB-1", Kind: core.EventNote}); err != nil {
			return err
		}
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatal(err)
	}
	var evs []core.Event
	inTx(t, db, func(tx *Tx) (err error) { evs, err = tx.EventsSince(0, 0); return err })
	if len(evs) != 0 {
		t.Fatalf("event survived rollback: %v", evs)
	}
}

func TestEventsSinceOrdered(t *testing.T) {
	db := openTemp(t)
	e1 := appendEvent(t, db, "KB-1", core.EventCreated)
	e2 := appendEvent(t, db, "KB-2", core.EventMoved)
	e3 := appendEvent(t, db, "KB-1", core.EventNote)
	if !(e1.ID < e2.ID && e2.ID < e3.ID) {
		t.Fatalf("event IDs not increasing: %d %d %d", e1.ID, e2.ID, e3.ID)
	}
	var evs []core.Event
	inTx(t, db, func(tx *Tx) (err error) { evs, err = tx.EventsSince(e1.ID, 0); return err })
	if len(evs) != 2 || evs[0].ID != e2.ID || evs[1].ID != e3.ID {
		t.Fatalf("EventsSince(e1) = %v", evs)
	}
	if !reflect.DeepEqual(evs[0], *e2) {
		t.Fatalf("event round trip:\n got %+v\nwant %+v", evs[0], *e2)
	}
	inTx(t, db, func(tx *Tx) (err error) { evs, err = tx.EventsSince(0, 1); return err })
	if len(evs) != 1 || evs[0].ID != e1.ID {
		t.Fatalf("EventsSince with limit = %v", evs)
	}
}

func TestEventsForItem(t *testing.T) {
	db := openTemp(t)
	appendEvent(t, db, "KB-1", core.EventCreated)
	appendEvent(t, db, "KB-2", core.EventCreated)
	appendEvent(t, db, "KB-1", core.EventMoved)
	var evs []core.Event
	inTx(t, db, func(tx *Tx) (err error) { evs, err = tx.EventsFor("KB-1"); return err })
	if len(evs) != 2 || evs[0].Kind != core.EventCreated || evs[1].Kind != core.EventMoved {
		t.Fatalf("EventsFor(KB-1) = %v", evs)
	}
}

func TestDataVersionChangesOnOtherConnectionWrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kb.db")
	watcher, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer watcher.Close()
	writer, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()

	ctx := context.Background()
	before, err := watcher.DataVersion(ctx)
	if err != nil {
		t.Fatal(err)
	}
	same, _ := watcher.DataVersion(ctx)
	if same != before {
		t.Fatalf("data_version changed without writes: %d → %d", before, same)
	}
	appendEvent(t, writer, "KB-1", core.EventNote)
	after, err := watcher.DataVersion(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if after == before {
		t.Fatal("data_version did not change after another connection committed")
	}
}
