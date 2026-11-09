package store

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// noEnv is a getenv that sees no variables.
func noEnv(string) string { return "" }

// realPath resolves symlinks so paths compare equal on systems where the
// temporary directory is a symlink.
func realPath(t *testing.T, p string) string {
	t.Helper()
	r, err := filepath.EvalSymlinks(p)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	base := []string{"-C", dir, "-c", "user.name=kb-test", "-c", "user.email=kb@test",
		"-c", "commit.gpgsign=false", "-c", "init.defaultBranch=main"}
	cmd := exec.Command("git", append(base, args...)...)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// newRepo creates a git repository with one commit and returns its path.
func newRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	repo := realPath(t, t.TempDir())
	git(t, repo, "init", "-q")
	git(t, repo, "commit", "-q", "--allow-empty", "-m", "init")
	return repo
}

func TestRootFromKBDir(t *testing.T) {
	dir := t.TempDir()
	env := func(k string) string {
		if k == "KB_DIR" {
			return dir
		}
		return ""
	}
	got, err := ResolveRoot(newRepo(t), env)
	if err != nil || got != dir {
		t.Fatalf("ResolveRoot = %q, %v; want %q (KB_DIR wins over git)", got, err, dir)
	}
}

func TestRootFromMainRepo(t *testing.T) {
	repo := newRepo(t)
	sub := filepath.Join(repo, "internal", "pkg")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := ResolveRoot(sub, noEnv)
	if err != nil || got != repo {
		t.Fatalf("ResolveRoot(subdir) = %q, %v; want %q", got, err, repo)
	}
}

func TestRootFromLinkedWorktree(t *testing.T) {
	repo := newRepo(t)
	wt := filepath.Join(realPath(t, t.TempDir()), "wt-auth")
	git(t, repo, "worktree", "add", "-q", "-b", "feature/auth", wt)
	got, err := ResolveRoot(wt, noEnv)
	if err != nil || got != repo {
		t.Fatalf("ResolveRoot(worktree) = %q, %v; want main worktree %q", got, err, repo)
	}
}

func TestRootWalkUpWithoutGit(t *testing.T) {
	root := realPath(t, t.TempDir())
	if err := os.MkdirAll(filepath.Join(root, ".kanban"), 0o755); err != nil {
		t.Fatal(err)
	}
	deep := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := ResolveRoot(deep, noEnv)
	if err != nil || got != root {
		t.Fatalf("ResolveRoot = %q, %v; want %q", got, err, root)
	}
}

func TestRootNotFound(t *testing.T) {
	if _, err := ResolveRoot(t.TempDir(), noEnv); !errors.Is(err, ErrNoBoard) {
		t.Fatalf("err = %v, want ErrNoBoard", err)
	}
}

func TestConfigDefaults(t *testing.T) {
	cfg, err := LoadConfig(filepath.Join(t.TempDir(), "missing.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Prefix != "KB" || cfg.ClaimTTL != 15*time.Minute {
		t.Fatalf("defaults = %+v, want prefix KB and ttl 15m", cfg)
	}
	path := filepath.Join(t.TempDir(), "config.toml")
	writeFile(t, path, "prefix = \"API\"\nclaim_ttl = \"30m\"\n")
	cfg, err = LoadConfig(path)
	if err != nil || cfg.Prefix != "API" || cfg.ClaimTTL != 30*time.Minute {
		t.Fatalf("LoadConfig = %+v, %v", cfg, err)
	}
	partial := filepath.Join(t.TempDir(), "config.toml")
	writeFile(t, partial, "prefix = \"API\"\n")
	if cfg, err = LoadConfig(partial); err != nil || cfg.ClaimTTL != 15*time.Minute {
		t.Fatalf("missing ttl must default: %+v, %v", cfg, err)
	}
}

func TestConfigTheme(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	writeFile(t, path, "theme = \"gruvbox-dark\"\n")
	cfg, err := LoadConfig(path)
	if err != nil || cfg.Theme != "gruvbox-dark" {
		t.Fatalf("LoadConfig = %+v, %v", cfg, err)
	}
	if cfg, _ := LoadConfig(filepath.Join(t.TempDir(), "none.toml")); cfg.Theme != "" {
		t.Fatalf("default theme = %q, want empty (the TUI default)", cfg.Theme)
	}
}

func TestConfigInvalidTTL(t *testing.T) {
	for _, body := range []string{`claim_ttl = "soon"`, `claim_ttl = "-5m"`, `claim_ttl = "0s"`} {
		path := filepath.Join(t.TempDir(), "config.toml")
		writeFile(t, path, body)
		if _, err := LoadConfig(path); err == nil || !strings.Contains(err.Error(), "claim_ttl") {
			t.Errorf("%s: err = %v, want an error about claim_ttl", body, err)
		}
	}
}

func TestConfigInvalidPrefix(t *testing.T) {
	for _, body := range []string{`prefix = "kb"`, `prefix = "K-B"`, `prefix = ""`, `prefix = "1KB"`} {
		path := filepath.Join(t.TempDir(), "config.toml")
		writeFile(t, path, body)
		if _, err := LoadConfig(path); err == nil || !strings.Contains(err.Error(), "prefix") {
			t.Errorf("%s: err = %v, want an error about prefix", body, err)
		}
	}
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
