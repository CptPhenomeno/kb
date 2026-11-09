package store

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

var (
	// ErrAlreadyInitialized means the board database already exists.
	ErrAlreadyInitialized = errors.New("kb board already initialized")
	// ErrNotInitialized means the project root has no board database.
	ErrNotInitialized = errors.New("kb board not initialized (run kb init)")
)

// gitignore keeps the local database out of git; config.toml stays versioned.
const gitignore = "# The board database is local; config.toml is versioned.\nkb.db\nkb.db-wal\nkb.db-shm\n"

// Init creates the board under root/.kanban: the database, config.toml and
// .gitignore. An existing config.toml (a fresh clone of a project that
// already uses kb) is kept; prefix, when given, must then match it. An
// empty prefix means the default one.
func Init(root, prefix string) error {
	dir := filepath.Join(root, boardDir)
	dbPath := filepath.Join(dir, dbFile)
	if _, err := os.Stat(dbPath); err == nil {
		return fmt.Errorf("%s: %w", dbPath, ErrAlreadyInitialized)
	}
	if prefix != "" {
		if err := ValidatePrefix(prefix); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	cfgPath := filepath.Join(dir, configFile)
	if _, err := os.Stat(cfgPath); err == nil {
		cfg, err := LoadConfig(cfgPath)
		if err != nil {
			return err
		}
		if prefix != "" && prefix != cfg.Prefix {
			return fmt.Errorf("%s already sets prefix %q, not %q", cfgPath, cfg.Prefix, prefix)
		}
	} else {
		cfg := DefaultConfig()
		if prefix != "" {
			cfg.Prefix = prefix
		}
		if err := os.WriteFile(cfgPath, []byte(cfg.encode()), 0o644); err != nil {
			return err
		}
	}
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(gitignore), 0o644); err != nil {
		return err
	}
	db, err := Open(dbPath)
	if err != nil {
		return err
	}
	return db.Close()
}

// OpenBoard opens the initialized board under root with its configuration.
func OpenBoard(root string) (*DB, Config, error) {
	dir := filepath.Join(root, boardDir)
	dbPath := filepath.Join(dir, dbFile)
	if _, err := os.Stat(dbPath); err != nil {
		return nil, Config{}, fmt.Errorf("%s: %w", root, ErrNotInitialized)
	}
	cfg, err := LoadConfig(filepath.Join(dir, configFile))
	if err != nil {
		return nil, Config{}, err
	}
	db, err := Open(dbPath)
	if err != nil {
		return nil, Config{}, err
	}
	return db, cfg, nil
}
