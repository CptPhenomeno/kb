package store

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Names of the board files, relative to the project root.
const (
	boardDir   = ".kanban"
	dbFile     = "kb.db"
	configFile = "config.toml"
)

// ErrNoBoard means no project root could be found for the current directory.
var ErrNoBoard = errors.New("no kb board found (run kb init)")

// ResolveRoot finds the project root that holds .kanban/ (SPEC.md §5):
//  1. the KB_DIR environment variable, if set;
//  2. inside a git repository, the main worktree, i.e. the directory that
//     contains `git rev-parse --git-common-dir`, so that every linked
//     worktree shares one board;
//  3. otherwise the nearest directory, from cwd upwards, holding .kanban/.
//
// In cases 1 and 2 the board may not be initialized yet.
func ResolveRoot(cwd string, getenv func(string) string) (string, error) {
	if dir := getenv("KB_DIR"); dir != "" {
		return filepath.Abs(dir)
	}
	if root, ok := gitMainWorktree(cwd); ok {
		return root, nil
	}
	for dir := cwd; ; dir = filepath.Dir(dir) {
		if info, err := os.Stat(filepath.Join(dir, boardDir)); err == nil && info.IsDir() {
			return dir, nil
		}
		if filepath.Dir(dir) == dir {
			return "", ErrNoBoard
		}
	}
}

// gitMainWorktree returns the root of the main worktree of the repository
// containing dir, if dir is inside a non-bare git repository.
func gitMainWorktree(dir string) (string, bool) {
	cmd := exec.Command("git", "-C", dir, "rev-parse", "--path-format=absolute", "--git-common-dir")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return "", false
	}
	common := strings.TrimSpace(out.String())
	if filepath.Base(common) != ".git" {
		return "", false // bare repository or unusual layout: fall back
	}
	return filepath.Dir(common), true
}
