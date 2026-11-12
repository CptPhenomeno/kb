package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/CptPhenomeno/kb/internal/core"
	"github.com/CptPhenomeno/kb/internal/store"
)

func TestGlobalFlagsParsed(t *testing.T) {
	e := newEnv(t)
	e.ok("--as", "agent-1", "add", "task", "t")
	item := e.jsonOf("show", "KB-1")
	if item["created_by"] != "agent-1" {
		t.Fatalf("created_by = %v, want agent-1 (from --as)", item["created_by"])
	}
	e.vars["KB_ACTOR"] = "agent-env"
	e.ok("add", "task", "u")
	if got := e.jsonOf("show", "KB-2")["created_by"]; got != "agent-env" {
		t.Fatalf("created_by = %v, want agent-env (from KB_ACTOR)", got)
	}
	delete(e.vars, "KB_ACTOR")
	e.ok("add", "task", "v")
	if got := e.jsonOf("show", "KB-3")["created_by"]; got != "@dev" {
		t.Fatalf("created_by = %v, want @dev (from git user)", got)
	}
	if out := e.ok("--quiet", "add", "task", "w"); out != "KB-4\n" {
		t.Fatalf("--quiet add = %q, want the bare ID", out)
	}
}

func TestNoArgsLaunchesTUI(t *testing.T) {
	e := newEnv(t)
	var got *Session
	e.tui = func(_ context.Context, s *Session) error { got = s; return nil }
	e.ok()
	if got == nil || got.Service == nil || got.Actor != "@dev" {
		t.Fatalf("TUI launched with %+v", got)
	}
}

func TestNotInitializedError(t *testing.T) {
	e := newEnv(t)
	e.vars["KB_DIR"] = t.TempDir()
	_, errOut := e.fail(1, "list")
	if !strings.Contains(errOut, "kb init") {
		t.Fatalf("stderr = %q, want a hint to run kb init", errOut)
	}
}

func TestInitTwice(t *testing.T) {
	e := newEnv(t)
	_, errOut := e.fail(1, "init")
	if !strings.Contains(errOut, "already initialized") {
		t.Fatalf("stderr = %q", errOut)
	}
}

func TestExitCodeMapping(t *testing.T) {
	cases := []struct {
		err  error
		code int
	}{
		{nil, 0},
		{errors.New("boom"), 1},
		{fmt.Errorf("x: %w", core.ErrNotFound), 2},
		{core.NewRuleError("nope"), 3},
		{fmt.Errorf("x: %w", core.ErrConflict), 4},
		{fmt.Errorf("x: %w", core.ErrClaimed), 5},
		{fmt.Errorf("x: %w", store.ErrNotInitialized), 1},
	}
	for _, c := range cases {
		if got := ExitCode(c.err); got != c.code {
			t.Errorf("ExitCode(%v) = %d, want %d", c.err, got, c.code)
		}
	}
}

func TestJSONErrorRequirements(t *testing.T) {
	e := newEnv(t)
	e.ok("add", "task", "t", "--test", "a")
	out, _ := e.fail(3, "--json", "move", "KB-1", "review")
	var v struct {
		Error   string   `json:"error"`
		Message string   `json:"message"`
		Unmet   []string `json:"unmet"`
	}
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		t.Fatalf("stdout is not JSON: %q", out)
	}
	if v.Error != "requirements" || len(v.Unmet) != 1 || v.Unmet[0] != `test 1 "a" is pending` {
		t.Fatalf("error JSON = %+v", v)
	}
}

func TestTextErrorToStderr(t *testing.T) {
	e := newEnv(t)
	out, errOut := e.fail(2, "show", "KB-404")
	if out != "" || !strings.Contains(errOut, "KB-404") {
		t.Fatalf("stdout = %q, stderr = %q", out, errOut)
	}
	_, errOut = e.fail(1, "move", "KB-1")
	if !strings.Contains(errOut, "usage") && !strings.Contains(errOut, "Usage") {
		t.Fatalf("usage error without usage: %q", errOut)
	}
}

func TestUnknownCommandSuggestsClosestCommand(t *testing.T) {
	e := newEnv(t)
	_, errOut := e.fail(1, "lsit")
	if !strings.Contains(errOut, `unknown command "lsit" for "kb"`) || !strings.Contains(errOut, `did you mean "list"?`) {
		t.Fatalf("stderr = %q, want the unknown command with the list suggestion", errOut)
	}
	if strings.Contains(errOut, "wrong number of arguments") {
		t.Fatalf("stderr = %q, still an arity error", errOut)
	}
}

func TestUnknownCommandWithoutCloseMatchHasNoSuggestion(t *testing.T) {
	e := newEnv(t)
	_, errOut := e.fail(1, "xyzzy")
	if !strings.Contains(errOut, `unknown command "xyzzy" for "kb"`) || strings.Contains(errOut, "did you mean") {
		t.Fatalf("stderr = %q, want the unknown command without a suggestion", errOut)
	}
}

func TestUnknownCommandExitsWithUsageErrorCode(t *testing.T) {
	e := newEnv(t)
	_, errOut, code := e.run("", "lsit", "KB-1")
	if code != 1 {
		t.Fatalf("exit %d, want 1 (usage error)", code)
	}
	if !strings.Contains(errOut, `unknown command "lsit" for "kb"`) || !strings.Contains(errOut, "usage: ") {
		t.Fatalf("stderr = %q, want the unknown command with the usage line", errOut)
	}
}

func TestUnknownCommandJSONError(t *testing.T) {
	e := newEnv(t)
	out, errOut := e.fail(1, "--json", "lsit")
	if errOut != "" {
		t.Fatalf("stderr = %q, want nothing with --json", errOut)
	}
	var v map[string]any
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		t.Fatalf("stdout is not JSON: %q", out)
	}
	if v["error"] != "error" || v["message"] != `unknown command "lsit" for "kb"; did you mean "list"?` || len(v) != 2 {
		t.Fatalf("error JSON = %v", v)
	}
}

func TestNoArgsStillLaunchesTUIWithGlobalFlags(t *testing.T) {
	e := newEnv(t)
	launched := 0
	e.tui = func(context.Context, *Session) error { launched++; return nil }
	e.ok()
	e.ok("--as", "agent-1")
	if launched != 2 {
		t.Fatalf("TUI launched %d times, want 2", launched)
	}
}
