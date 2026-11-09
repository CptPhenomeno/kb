package store

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strconv"

	"github.com/CptPhenomeno/kb/internal/core"
	_ "modernc.org/sqlite" // Pure Go SQLite driver.
)

// busyTimeoutMS is how long a connection waits for a lock before failing.
const busyTimeoutMS = 5000

var errNoRows = sql.ErrNoRows

// DB is an open kb database.
type DB struct {
	sql   *sql.DB
	path  string
	watch watcher
}

// Open opens (creating it if needed) the database at path, configures it
// for concurrent use by several processes and applies pending migrations.
func Open(path string) (*DB, error) {
	q := url.Values{}
	q.Add("_pragma", "journal_mode(WAL)")
	q.Add("_pragma", fmt.Sprintf("busy_timeout(%d)", busyTimeoutMS))
	q.Add("_pragma", "foreign_keys(1)")
	q.Add("_pragma", "synchronous(NORMAL)")
	// Every transaction starts with BEGIN IMMEDIATE, so a writer takes the
	// lock up front and concurrent writers queue instead of deadlocking.
	q.Set("_txlock", "immediate")
	dsn := (&url.URL{Scheme: "file", Path: path, RawQuery: q.Encode()}).String()

	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	db := &DB{sql: sqlDB, path: path}
	if err := migrate(context.Background(), sqlDB, migrationsFS); err != nil {
		sqlDB.Close()
		return nil, err
	}
	return db, nil
}

// Close closes the database.
func (d *DB) Close() error {
	d.watch.mu.Lock()
	if d.watch.conn != nil {
		d.watch.conn.Close()
		d.watch.conn = nil
	}
	d.watch.mu.Unlock()
	return d.sql.Close()
}

// Path returns the path of the database file.
func (d *DB) Path() string { return d.path }

// Tx runs fn inside a single BEGIN IMMEDIATE transaction. The transaction
// is committed if fn returns nil and rolled back if it returns an error or
// panics; a panic is re-raised after the rollback.
func (d *DB) Tx(ctx context.Context, fn func(*Tx) error) (err error) {
	sqlTx, err := d.sql.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		if p := recover(); p != nil {
			_ = sqlTx.Rollback()
			panic(p)
		}
		if err != nil {
			_ = sqlTx.Rollback()
			return
		}
		if cerr := sqlTx.Commit(); cerr != nil {
			err = fmt.Errorf("commit transaction: %w", cerr)
		}
	}()
	return fn(&Tx{sql: sqlTx, ctx: ctx})
}

// Tx is an open transaction. Every read and write of the repository goes
// through a Tx, so callers see a consistent board.
type Tx struct {
	sql *sql.Tx
	ctx context.Context
}

func itoa(i int) string { return strconv.Itoa(i) }

// Transact runs fn in a transaction, exposing the transaction through the
// core.Tx interface the service works on.
func (d *DB) Transact(ctx context.Context, fn func(core.Tx) error) error {
	return d.Tx(ctx, func(tx *Tx) error { return fn(tx) })
}
