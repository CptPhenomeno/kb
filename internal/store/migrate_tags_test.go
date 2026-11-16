package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"reflect"
	"testing"
	"testing/fstest"
)

// v1Item is a row to seed in a version 1 database.
type v1Item struct {
	id, typ, created string
	points           int
	labels           string
}

// newV1DB builds a database at user_version 1 (only migration 0001) holding
// items, and returns its path.
func newV1DB(t *testing.T, items []v1Item) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "kb.db")
	sqlDB, err := sql.Open("sqlite", "file:"+path+"?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	body, err := fs.ReadFile(migrationsFS, "migrations/0001_init.sql")
	if err != nil {
		t.Fatal(err)
	}
	only1 := fstest.MapFS{"migrations/0001_init.sql": {Data: body}}
	if err := migrate(context.Background(), sqlDB, only1, nil); err != nil {
		t.Fatalf("migrate v1: %v", err)
	}
	for i, it := range items {
		_, err := sqlDB.Exec(`INSERT INTO items(id, type, title, status, priority, points, labels,
			position, version, created_at, created_by, updated_at, updated_by)
			VALUES (?, ?, 't', 'todo', 'medium', ?, ?, ?, 1, ?, 'a', ?, 'a')`,
			it.id, it.typ, it.points, it.labels, i, it.created, it.created)
		if err != nil {
			t.Fatal(err)
		}
	}
	return path
}

type tagRow struct{ Name, Color string }

func tagsOf(t *testing.T, db *DB) []tagRow {
	t.Helper()
	rows, err := db.sql.Query(`SELECT name, color FROM tags ORDER BY key`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []tagRow
	for rows.Next() {
		var r tagRow
		if err := rows.Scan(&r.Name, &r.Color); err != nil {
			t.Fatal(err)
		}
		out = append(out, r)
	}
	return out
}

func cardTagsOf(t *testing.T, db *DB, item string) []string {
	t.Helper()
	rows, err := db.sql.Query(`SELECT t.name FROM card_tags ct JOIN tags t ON t.id = ct.tag
		WHERE ct.item = ? ORDER BY ct.position`, item)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			t.Fatal(err)
		}
		out = append(out, n)
	}
	return out
}

func openPath(t *testing.T, path string) *DB {
	t.Helper()
	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestMigrationTurnsEachDistinctLabelIntoATag(t *testing.T) {
	path := newV1DB(t, []v1Item{
		{"KB-1", "task", "2026-01-01T00:00:00Z", 2, `["store","api"]`},
		{"KB-2", "task", "2026-01-02T00:00:00Z", 1, `["api","ui"]`},
		{"KB-3", "task", "2026-01-03T00:00:00Z", 1, `[]`},
	})
	db := openPath(t, path)
	var names []string
	for _, r := range tagsOf(t, db) {
		names = append(names, r.Name)
	}
	if want := []string{"api", "store", "ui"}; !reflect.DeepEqual(names, want) {
		t.Fatalf("tags = %v, want %v", names, want)
	}
}

func TestMigrationMergesCaseVariantsWithFirstSpelling(t *testing.T) {
	// KB-2 is created first, so its "API" wins over KB-1's "api"; ties on
	// created_at are broken by id.
	path := newV1DB(t, []v1Item{
		{"KB-1", "task", "2026-01-05T00:00:00Z", 1, `["api"]`},
		{"KB-2", "task", "2026-01-01T00:00:00Z", 1, `["Ünï","API"]`},
		{"KB-3", "task", "2026-01-01T00:00:00Z", 1, `["ünï","Api"]`},
	})
	db := openPath(t, path)
	var names []string
	for _, r := range tagsOf(t, db) {
		names = append(names, r.Name)
	}
	if want := []string{"API", "Ünï"}; !reflect.DeepEqual(names, want) {
		t.Fatalf("tags = %v, want %v", names, want)
	}
	if got := cardTagsOf(t, db, "KB-1"); !reflect.DeepEqual(got, []string{"API"}) {
		t.Fatalf("KB-1 tags = %v", got)
	}
}

func TestMigratedTagColorsRotateAlphabetically(t *testing.T) {
	labels := `["pear","Apple","fig","kiwi","date","banana","grape","cherry","elder"]`
	path := newV1DB(t, []v1Item{{"KB-1", "task", "2026-01-01T00:00:00Z", 1, labels}})
	db := openPath(t, path)
	want := []tagRow{
		{"Apple", "red"}, {"banana", "orange"}, {"cherry", "yellow"}, {"date", "green"},
		{"elder", "cyan"}, {"fig", "blue"}, {"grape", "purple"}, {"kiwi", "pink"},
		{"pear", "red"},
	}
	if got := tagsOf(t, db); !reflect.DeepEqual(got, want) {
		t.Fatalf("tags = %v, want %v", got, want)
	}
}

func TestMigratedCardsKeepTagsInLabelOrder(t *testing.T) {
	path := newV1DB(t, []v1Item{
		{"KB-1", "task", "2026-01-01T00:00:00Z", 1, `["zeta","alpha","Mid"]`},
		{"KB-2", "bug", "2026-01-02T00:00:00Z", 1, `["MID","zeta","dup","DUP"]`},
	})
	db := openPath(t, path)
	if got, want := cardTagsOf(t, db, "KB-1"), []string{"zeta", "alpha", "Mid"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("KB-1 = %v, want %v", got, want)
	}
	if got, want := cardTagsOf(t, db, "KB-2"), []string{"Mid", "zeta", "dup"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("KB-2 = %v, want %v", got, want)
	}
}

func TestMigrationZeroesStoryPointsKeepsTaskPoints(t *testing.T) {
	path := newV1DB(t, []v1Item{
		{"KB-1", "story", "2026-01-01T00:00:00Z", 5, `[]`},
		{"KB-2", "task", "2026-01-02T00:00:00Z", 3, `[]`},
		{"KB-3", "bug", "2026-01-03T00:00:00Z", 2, `[]`},
	})
	db := openPath(t, path)
	for id, want := range map[string]int{"KB-1": 0, "KB-2": 3, "KB-3": 2} {
		var got int
		if err := db.sql.QueryRow(`SELECT points FROM items WHERE id = ?`, id).Scan(&got); err != nil || got != want {
			t.Errorf("%s points = %d (err %v), want %d", id, got, err, want)
		}
	}
	var events int
	if err := db.sql.QueryRow(`SELECT count(*) FROM events`).Scan(&events); err != nil || events != 0 {
		t.Errorf("events = %d (err %v), want 0", events, err)
	}
}

func TestMigrationRollsBackOnError(t *testing.T) {
	path := newV1DB(t, []v1Item{{"KB-1", "story", "2026-01-01T00:00:00Z", 5, `["a"]`}})
	sqlDB, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	boom := errors.New("boom")
	steps := map[int]goStep{2: func(ctx context.Context, tx *sql.Tx) error {
		// The SQL of 0002 has already run in this transaction.
		if _, err := tx.ExecContext(ctx, `UPDATE items SET points = 9`); err != nil {
			return err
		}
		return boom
	}}
	if err := migrate(context.Background(), sqlDB, migrationsFS, steps); !errors.Is(err, boom) {
		t.Fatalf("migrate err = %v, want boom", err)
	}
	var v, tables, points int
	sqlDB.QueryRow("PRAGMA user_version").Scan(&v)
	sqlDB.QueryRow(`SELECT count(*) FROM sqlite_master WHERE name IN ('tags','card_tags')`).Scan(&tables)
	sqlDB.QueryRow(`SELECT points FROM items`).Scan(&points)
	if v != 1 || tables != 0 || points != 5 {
		t.Fatalf("after failure: user_version=%d tag tables=%d points=%d, want 1, 0, 5", v, tables, points)
	}
	// A later run with the real steps succeeds.
	if err := migrate(context.Background(), sqlDB, migrationsFS, goSteps); err != nil {
		t.Fatalf("retry: %v", err)
	}
}

func schemaOf(t *testing.T, db *DB) map[string]string {
	t.Helper()
	rows, err := db.sql.Query(`SELECT name, coalesce(sql, '') FROM sqlite_master WHERE name NOT LIKE 'sqlite_%'`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var n, s string
		if err := rows.Scan(&n, &s); err != nil {
			t.Fatal(err)
		}
		out[n] = s
	}
	return out
}

func TestFreshSchemaMatchesMigratedV1Schema(t *testing.T) {
	fresh := openTemp(t)
	migrated := openPath(t, newV1DB(t, nil))
	if a, b := schemaOf(t, fresh), schemaOf(t, migrated); !reflect.DeepEqual(a, b) {
		t.Fatalf("schemas differ:\nfresh:    %v\nmigrated: %v", a, b)
	}
	for _, db := range []*DB{fresh, migrated} {
		if got := pragma(t, db, "user_version"); got != fmt.Sprint(len(mustMigrations(t))) {
			t.Fatalf("user_version = %s", got)
		}
	}
}
