package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/CptPhenomeno/kb/internal/core"
)

// AppendEvent adds an event to the log and sets its ID. Call it in the same
// transaction as the change it records, so both commit or neither does.
func (tx *Tx) AppendEvent(e *core.Event) error {
	data := "{}"
	if len(e.Data) > 0 {
		b, err := json.Marshal(e.Data)
		if err != nil {
			return fmt.Errorf("encode event data: %w", err)
		}
		data = string(b)
	}
	res, err := tx.exec("INSERT INTO events(at, actor, item, kind, data) VALUES (?, ?, ?, ?, ?)",
		formatTime(e.At), e.Actor, e.Item, string(e.Kind), data)
	if err != nil {
		return fmt.Errorf("append event: %w", err)
	}
	e.ID, err = res.LastInsertId()
	return err
}

// EventsSince returns the events with an ID greater than afterID, oldest
// first. A positive limit caps the number of events returned.
func (tx *Tx) EventsSince(afterID int64, limit int) ([]core.Event, error) {
	q := "SELECT id, at, actor, item, kind, data FROM events WHERE id > ? ORDER BY id"
	args := []any{afterID}
	if limit > 0 {
		q += " LIMIT ?"
		args = append(args, limit)
	}
	return tx.events(q, args...)
}

// EventsFor returns the history of one item, oldest first.
func (tx *Tx) EventsFor(item string) ([]core.Event, error) {
	return tx.events("SELECT id, at, actor, item, kind, data FROM events WHERE item = ? ORDER BY id", item)
}

func (tx *Tx) events(q string, args ...any) ([]core.Event, error) {
	rows, err := tx.query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []core.Event
	for rows.Next() {
		var e core.Event
		var at, kind, data string
		if err := rows.Scan(&e.ID, &at, &e.Actor, &e.Item, &kind, &data); err != nil {
			return nil, err
		}
		if e.At, err = parseTime(at); err != nil {
			return nil, err
		}
		e.Kind = core.EventKind(kind)
		if data != "{}" {
			if err := json.Unmarshal([]byte(data), &e.Data); err != nil {
				return nil, fmt.Errorf("decode event %d data: %w", e.ID, err)
			}
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// watcher holds the connection used to read PRAGMA data_version. The value
// is per connection, so it must always be read on the same one.
type watcher struct {
	mu   sync.Mutex
	conn *sql.Conn
}

// DataVersion returns a number that changes whenever another connection
// commits a write to the database. The TUI and `kb log --follow` poll it to
// notice changes made by other processes.
func (d *DB) DataVersion(ctx context.Context) (int64, error) {
	d.watch.mu.Lock()
	defer d.watch.mu.Unlock()
	if d.watch.conn == nil {
		conn, err := d.sql.Conn(ctx)
		if err != nil {
			return 0, err
		}
		d.watch.conn = conn
	}
	var v int64
	err := d.watch.conn.QueryRowContext(ctx, "PRAGMA data_version").Scan(&v)
	return v, err
}
