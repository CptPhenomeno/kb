package store

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"testing/fstest"
	"time"
)

// openTemp opens a fresh database file in a temporary directory.
func openTemp(t *testing.T) *DB {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "kb.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func pragma(t *testing.T, db *DB, name string) string {
	t.Helper()
	var v string
	if err := db.sql.QueryRow("PRAGMA " + name).Scan(&v); err != nil {
		t.Fatalf("PRAGMA %s: %v", name, err)
	}
	return v
}

func TestOpenCreatesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kb.db")
	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("database file not created: %v", err)
	}
}

func TestOpenSetsWAL(t *testing.T) {
	if got := pragma(t, openTemp(t), "journal_mode"); got != "wal" {
		t.Fatalf("journal_mode = %q, want wal", got)
	}
}

func TestOpenSetsBusyTimeout(t *testing.T) {
	if got := pragma(t, openTemp(t), "busy_timeout"); got != "5000" {
		t.Fatalf("busy_timeout = %q, want 5000", got)
	}
}

func TestForeignKeysEnabled(t *testing.T) {
	if got := pragma(t, openTemp(t), "foreign_keys"); got != "1" {
		t.Fatalf("foreign_keys = %q, want 1", got)
	}
}

func TestMigrateFreshDB(t *testing.T) {
	db := openTemp(t)
	if got := pragma(t, db, "user_version"); got != itoa(len(mustMigrations(t))) {
		t.Fatalf("user_version = %s, want %d", got, len(mustMigrations(t)))
	}
	for _, table := range []string{"meta", "items", "relations", "events"} {
		var n int
		err := db.sql.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&n)
		if err != nil || n != 1 {
			t.Errorf("table %s missing (n=%d, err=%v)", table, n, err)
		}
	}
}

func TestMigrateIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kb.db")
	for range 3 {
		db, err := Open(path)
		if err != nil {
			t.Fatalf("Open: %v", err)
		}
		db.Close()
	}
}

func TestMigrateFromV1ToLatest(t *testing.T) {
	fsys := fstest.MapFS{
		"migrations/0001_first.sql":  {Data: []byte("CREATE TABLE a (x INTEGER);")},
		"migrations/0002_second.sql": {Data: []byte("CREATE TABLE b (y INTEGER); INSERT INTO a VALUES (1);")},
	}
	db := openTemp(t)
	if _, err := db.sql.Exec("PRAGMA user_version = 0"); err != nil {
		t.Fatal(err)
	}
	// Apply only the first migration, then both: the second run must apply
	// 0002 alone.
	first := fstest.MapFS{"migrations/0001_first.sql": fsys["migrations/0001_first.sql"]}
	if err := migrate(context.Background(), db.sql, first); err != nil {
		t.Fatalf("migrate v1: %v", err)
	}
	if err := migrate(context.Background(), db.sql, fsys); err != nil {
		t.Fatalf("migrate v2: %v", err)
	}
	if got := pragma(t, db, "user_version"); got != "2" {
		t.Fatalf("user_version = %s, want 2", got)
	}
	var n int
	if err := db.sql.QueryRow("SELECT count(*) FROM a").Scan(&n); err != nil || n != 1 {
		t.Fatalf("migration 0002 not applied exactly once: n=%d err=%v", n, err)
	}
}

func TestRejectNewerSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kb.db")
	db, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.sql.Exec("PRAGMA user_version = 999"); err != nil {
		t.Fatal(err)
	}
	db.Close()
	if _, err := Open(path); !errors.Is(err, ErrSchemaTooNew) {
		t.Fatalf("Open newer schema: err = %v, want ErrSchemaTooNew", err)
	}
}

func TestTxCommit(t *testing.T) {
	db := openTemp(t)
	err := db.Tx(context.Background(), func(tx *Tx) error {
		_, err := tx.sql.Exec("INSERT INTO meta(key, value) VALUES ('k', 'v')")
		return err
	})
	if err != nil {
		t.Fatalf("Tx: %v", err)
	}
	if got := metaValue(t, db, "k"); got != "v" {
		t.Fatalf("meta k = %q, want v", got)
	}
}

func TestTxRollbackOnError(t *testing.T) {
	db := openTemp(t)
	boom := errors.New("boom")
	err := db.Tx(context.Background(), func(tx *Tx) error {
		if _, err := tx.sql.Exec("INSERT INTO meta(key, value) VALUES ('k', 'v')"); err != nil {
			return err
		}
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("Tx error = %v, want boom", err)
	}
	if got := metaValue(t, db, "k"); got != "" {
		t.Fatalf("rolled back row still present: %q", got)
	}
}

func TestTxRollbackOnPanic(t *testing.T) {
	db := openTemp(t)
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("panic was swallowed")
			}
		}()
		_ = db.Tx(context.Background(), func(tx *Tx) error {
			if _, err := tx.sql.Exec("INSERT INTO meta(key, value) VALUES ('k', 'v')"); err != nil {
				return err
			}
			panic("boom")
		})
	}()
	if got := metaValue(t, db, "k"); got != "" {
		t.Fatalf("row survived a panicking transaction: %q", got)
	}
}

// TestImmediateSerializesWriters checks that two connections writing
// concurrently do not fail with SQLITE_BUSY: BEGIN IMMEDIATE makes the
// second writer wait for the first.
func TestImmediateSerializesWriters(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kb.db")
	a, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	b, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()

	const n = 20
	var wg sync.WaitGroup
	errs := make(chan error, 2*n)
	for i := range n {
		for j, db := range []*DB{a, b} {
			wg.Add(1)
			go func() {
				defer wg.Done()
				errs <- db.Tx(context.Background(), func(tx *Tx) error {
					// Read-then-write: the pattern that deadlocks with
					// deferred transactions.
					var v string
					_ = tx.sql.QueryRow("SELECT value FROM meta WHERE key='counter'").Scan(&v)
					time.Sleep(time.Millisecond)
					_, err := tx.sql.Exec("INSERT INTO meta(key, value) VALUES (?, 'x')", "w"+itoa(i)+"-"+itoa(j))
					return err
				})
			}()
		}
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent writer failed: %v", err)
		}
	}
}

func metaValue(t *testing.T, db *DB, key string) string {
	t.Helper()
	var v string
	err := db.sql.QueryRow("SELECT value FROM meta WHERE key=?", key).Scan(&v)
	if err != nil && !errors.Is(err, errNoRows) {
		t.Fatalf("read meta %s: %v", key, err)
	}
	return v
}

func mustMigrations(t *testing.T) []string {
	t.Helper()
	names, err := migrationNames(migrationsFS)
	if err != nil {
		t.Fatal(err)
	}
	return names
}
