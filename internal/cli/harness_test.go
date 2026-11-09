package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var update = flag.Bool("update", false, "rewrite golden files")

// env runs the CLI in-process against a board in a temporary directory,
// with a fixed clock and controlled environment variables.
type env struct {
	t    *testing.T
	dir  string
	now  time.Time
	vars map[string]string
	tui  func(context.Context, *Session) error
}

func newEnv(t *testing.T) *env {
	t.Helper()
	e := &env{t: t, dir: t.TempDir(), now: time.Date(2026, 11, 9, 14, 0, 0, 0, time.UTC),
		vars: map[string]string{}}
	e.vars["KB_DIR"] = e.dir
	e.ok("init")
	return e
}

func (e *env) app(stdin string) (*App, *bytes.Buffer, *bytes.Buffer) {
	var out, errOut bytes.Buffer
	return &App{
		Stdin:     strings.NewReader(stdin),
		Stdout:    &out,
		Stderr:    &errOut,
		Getenv:    func(k string) string { return e.vars[k] },
		Cwd:       e.dir,
		Now:       func() time.Time { return e.now },
		GitUser:   func() string { return "dev" },
		LaunchTUI: e.tui,
	}, &out, &errOut
}

// run executes kb with args and returns stdout, stderr and the exit code.
func (e *env) run(stdin string, args ...string) (string, string, int) {
	e.t.Helper()
	app, out, errOut := e.app(stdin)
	code := Run(context.Background(), args, app)
	return out.String(), errOut.String(), code
}

// ok runs kb and fails the test unless it exits with 0.
func (e *env) ok(args ...string) string {
	e.t.Helper()
	out, errOut, code := e.run("", args...)
	if code != 0 {
		e.t.Fatalf("kb %s: exit %d\nstdout: %s\nstderr: %s", strings.Join(args, " "), code, out, errOut)
	}
	return out
}

// fail runs kb and checks the exit code.
func (e *env) fail(code int, args ...string) (string, string) {
	e.t.Helper()
	out, errOut, got := e.run("", args...)
	if got != code {
		e.t.Fatalf("kb %s: exit %d, want %d\nstdout: %s\nstderr: %s", strings.Join(args, " "), got, code, out, errOut)
	}
	return out, errOut
}

// as runs kb as the given actor.
func (e *env) as(actor string, args ...string) string {
	e.t.Helper()
	return e.ok(append([]string{"--as", actor}, args...)...)
}

// jsonOf runs kb with --json and decodes the output.
func (e *env) jsonOf(args ...string) map[string]any {
	e.t.Helper()
	out := e.ok(append([]string{"--json"}, args...)...)
	var v map[string]any
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		e.t.Fatalf("invalid JSON from kb %v: %v\n%s", args, err, out)
	}
	return v
}

// golden compares got with testdata/<name>.golden; -update rewrites it.
func golden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name+".golden")
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("missing golden file %s (run go test -update): %v", path, err)
	}
	if got != string(want) {
		t.Fatalf("output differs from %s:\n--- got\n%s\n--- want\n%s", path, got, want)
	}
}
