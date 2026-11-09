package cli

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

// kbBin is the kb binary built once for the end-to-end tests.
var kbBin string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "kb-e2e-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	kbBin = filepath.Join(dir, "kb")
	if out, err := exec.Command("go", "build", "-o", kbBin, "../../cmd/kb").CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "build kb: %v\n%s", err, out)
		os.Exit(1)
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// binBoard is a board driven through the real kb binary.
type binBoard struct {
	t   *testing.T
	dir string
}

func newBinBoard(t *testing.T) *binBoard {
	t.Helper()
	b := &binBoard{t: t, dir: t.TempDir()}
	b.mustRun("", "init")
	return b
}

func (b *binBoard) cmd(stdin string, args ...string) *exec.Cmd {
	c := exec.Command(kbBin, args...)
	c.Dir = b.dir
	c.Env = append(os.Environ(), "KB_DIR="+b.dir, "KB_ACTOR=")
	c.Stdin = strings.NewReader(stdin)
	return c
}

// run returns stdout and the exit code.
func (b *binBoard) run(stdin string, args ...string) (string, int) {
	b.t.Helper()
	var out bytes.Buffer
	c := b.cmd(stdin, args...)
	c.Stdout = &out
	err := c.Run()
	var exitErr *exec.ExitError
	switch {
	case err == nil:
		return out.String(), 0
	case errors.As(err, &exitErr):
		return out.String(), exitErr.ExitCode()
	default:
		b.t.Fatalf("kb %v: %v", args, err)
		return "", -1
	}
}

func (b *binBoard) mustRun(stdin string, args ...string) string {
	b.t.Helper()
	out, code := b.run(stdin, args...)
	if code != 0 {
		b.t.Fatalf("kb %v: exit %d\n%s", args, code, out)
	}
	return out
}

func TestE2EBinaryExitCodes(t *testing.T) {
	b := newBinBoard(t)
	b.mustRun("described from stdin\n", "add", "task", "t", "--test", "a", "--desc", "-")
	cases := []struct {
		args []string
		code int
	}{
		{[]string{"show", "KB-1"}, 0},
		{[]string{"nope"}, 1},
		{[]string{"show", "KB-9"}, 2},
		{[]string{"move", "KB-1", "done"}, 3},
		{[]string{"move", "KB-1", "todo", "--if-version", "9"}, 4},
	}
	for _, c := range cases {
		if _, got := b.run("", c.args...); got != c.code {
			t.Errorf("kb %v: exit %d, want %d", c.args, got, c.code)
		}
	}
	b.mustRun("", "--as", "agent-1", "claim", "KB-1")
	if _, got := b.run("", "--as", "agent-2", "claim", "KB-1"); got != 5 {
		t.Errorf("claim by another agent: exit %d, want 5", got)
	}
}

// TestE2EParallelNextClaim starts N processes claiming M items at once:
// every item is handed out exactly once and the others exit with 2.
func TestE2EParallelNextClaim(t *testing.T) {
	b := newBinBoard(t)
	const items, procs = 6, 10
	for i := range items {
		b.mustRun("", "add", "task", fmt.Sprintf("t%d", i), "--status", "todo")
	}
	type result struct {
		id   string
		code int
	}
	results := make([]result, procs)
	var wg sync.WaitGroup
	for p := range procs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			out, code := b.run("", "--as", fmt.Sprintf("agent-%d", p), "--quiet", "next", "--claim")
			results[p] = result{strings.TrimSpace(out), code}
		}()
	}
	wg.Wait()
	seen := map[string]bool{}
	misses := 0
	for _, r := range results {
		switch r.code {
		case 0:
			if seen[r.id] {
				t.Fatalf("%s handed out twice", r.id)
			}
			seen[r.id] = true
		case 2:
			misses++
		default:
			t.Fatalf("unexpected exit %d (%q)", r.code, r.id)
		}
	}
	if len(seen) != items || misses != procs-items {
		t.Fatalf("%d claims and %d misses, want %d and %d", len(seen), misses, items, procs-items)
	}
}

// TestLogFollowSeesOtherProcessWrite checks that `kb log --follow` prints an
// event written by another process within a second, and exits cleanly on
// SIGINT.
func TestLogFollowSeesOtherProcessWrite(t *testing.T) {
	b := newBinBoard(t)
	b.mustRun("", "add", "task", "t")
	follow := b.cmd("", "--json", "log", "--follow", "--since", "1s")
	stdout, err := follow.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := follow.Start(); err != nil {
		t.Fatal(err)
	}
	lines := make(chan string)
	go func() {
		sc := bufio.NewScanner(stdout)
		for sc.Scan() {
			lines <- sc.Text()
		}
		close(lines)
	}()
	time.Sleep(300 * time.Millisecond) // let the follower start polling
	start := time.Now()
	b.mustRun("", "--as", "agent-1", "note", "KB-1", "hello from another process")

	deadline := time.After(3 * time.Second)
	found := false
	for !found {
		select {
		case l, ok := <-lines:
			if !ok {
				t.Fatal("follower exited early")
			}
			var ev map[string]any
			if json.Unmarshal([]byte(l), &ev) == nil && ev["kind"] == "note" {
				found = true
			}
		case <-deadline:
			t.Fatal("note not printed within 3s")
		}
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("event printed after %v, want under 1s", elapsed)
	}
	if err := follow.Process.Signal(syscall.SIGINT); err != nil {
		t.Fatal(err)
	}
	for range lines {
	}
	if err := follow.Wait(); err != nil {
		t.Fatalf("follower exit: %v, want a clean exit on SIGINT", err)
	}
}
