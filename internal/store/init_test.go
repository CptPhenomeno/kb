package store

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitCreatesFiles(t *testing.T) {
	root := t.TempDir()
	if err := Init(root, ""); err != nil {
		t.Fatalf("Init: %v", err)
	}
	for _, f := range []string{"kb.db", "config.toml", ".gitignore"} {
		if _, err := os.Stat(filepath.Join(root, ".kanban", f)); err != nil {
			t.Errorf(".kanban/%s not created: %v", f, err)
		}
	}
	db, cfg, err := OpenBoard(root)
	if err != nil {
		t.Fatalf("OpenBoard after Init: %v", err)
	}
	defer db.Close()
	if cfg.Prefix != "KB" {
		t.Errorf("prefix = %q, want KB", cfg.Prefix)
	}
}

func TestInitGitignoreContent(t *testing.T) {
	root := t.TempDir()
	if err := Init(root, ""); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(root, ".kanban", ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"kb.db", "kb.db-wal", "kb.db-shm"} {
		if !strings.Contains(string(body), want+"\n") {
			t.Errorf(".gitignore lacks %s:\n%s", want, body)
		}
	}
	for _, line := range strings.Split(string(body), "\n") {
		if strings.TrimSpace(line) == "config.toml" {
			t.Error("config.toml must stay versioned, but .gitignore ignores it")
		}
	}
}

func TestInitTwiceFails(t *testing.T) {
	root := t.TempDir()
	if err := Init(root, ""); err != nil {
		t.Fatal(err)
	}
	if err := Init(root, ""); !errors.Is(err, ErrAlreadyInitialized) {
		t.Fatalf("second Init: err = %v, want ErrAlreadyInitialized", err)
	}
}

func TestInitCustomPrefix(t *testing.T) {
	root := t.TempDir()
	if err := Init(root, "API"); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(filepath.Join(root, ".kanban", "config.toml"))
	if err != nil || cfg.Prefix != "API" {
		t.Fatalf("config = %+v, %v; want prefix API", cfg, err)
	}
	if err := Init(t.TempDir(), "bad prefix"); err == nil {
		t.Fatal("invalid prefix accepted")
	}
}

// TestInitKeepsCommittedConfig covers a fresh clone: config.toml is
// versioned, kb.db is not, so init must create the database and keep the
// existing configuration.
func TestInitKeepsCommittedConfig(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".kanban"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(root, ".kanban", "config.toml")
	writeFile(t, cfgPath, "prefix = \"GW\"\nclaim_ttl = \"20m\"\n")
	if err := Init(root, ""); err != nil {
		t.Fatalf("Init with committed config: %v", err)
	}
	body, _ := os.ReadFile(cfgPath)
	if !strings.Contains(string(body), `"GW"`) || !strings.Contains(string(body), `"20m"`) {
		t.Fatalf("committed config overwritten:\n%s", body)
	}
	other := t.TempDir()
	if err := os.MkdirAll(filepath.Join(other, ".kanban"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(other, ".kanban", "config.toml"), "prefix = \"GW\"\n")
	if err := Init(other, "API"); err == nil {
		t.Fatal("a prefix different from the committed one must be rejected")
	}
}

func TestOpenBoardNotInitialized(t *testing.T) {
	if _, _, err := OpenBoard(t.TempDir()); !errors.Is(err, ErrNotInitialized) {
		t.Fatalf("err = %v, want ErrNotInitialized", err)
	}
}
