package store

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strings"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// ErrSchemaTooNew means the database was written by a newer kb.
var ErrSchemaTooNew = errors.New("database schema is newer than this kb binary")

// migrationNames returns the migration files of fsys in application order.
// Files are named NNNN_description.sql and numbered from 0001 without gaps.
func migrationNames(fsys fs.FS) ([]string, error) {
	entries, err := fs.ReadDir(fsys, "migrations")
	if err != nil {
		return nil, fmt.Errorf("read migrations: %w", err)
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	slices.Sort(names)
	for i, n := range names {
		if want := fmt.Sprintf("%04d_", i+1); !strings.HasPrefix(n, want) {
			return nil, fmt.Errorf("migration %s: expected prefix %s", n, want)
		}
	}
	return names, nil
}

// migrate applies, in one transaction each, the migrations of fsys that are
// newer than the database's PRAGMA user_version.
func migrate(ctx context.Context, db *sql.DB, fsys fs.FS) error {
	names, err := migrationNames(fsys)
	if err != nil {
		return err
	}
	var current int
	if err := db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&current); err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}
	if current > len(names) {
		return fmt.Errorf("%w (database v%d, binary v%d)", ErrSchemaTooNew, current, len(names))
	}
	for i := current; i < len(names); i++ {
		body, err := fs.ReadFile(fsys, path.Join("migrations", names[i]))
		if err != nil {
			return fmt.Errorf("read migration %s: %w", names[i], err)
		}
		if err := applyMigration(ctx, db, string(body), i+1); err != nil {
			return fmt.Errorf("apply migration %s: %w", names[i], err)
		}
	}
	return nil
}

func applyMigration(ctx context.Context, db *sql.DB, body string, version int) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, body); err != nil {
		return err
	}
	// PRAGMA does not accept bound parameters; version is an integer.
	if _, err := tx.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version = %d", version)); err != nil {
		return err
	}
	return tx.Commit()
}
