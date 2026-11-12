package cli

import (
	"strings"
	"testing"
	"time"
)

func TestE2EAddAndShow(t *testing.T) {
	e := newEnv(t)
	out := e.ok("add", "story", "Token login", "--points", "5", "--accept", "expired → 401")
	if !strings.Contains(out, "KB-1") {
		t.Fatalf("add output = %q", out)
	}
	item := e.jsonOf("show", "KB-1")
	if item["type"] != "story" || item["title"] != "Token login" || item["points"] != 5.0 || item["status"] != "backlog" {
		t.Fatalf("item = %v", item)
	}
	byRef := e.ok("add", "task", "t", "--ref", "auth-t", "--parent", "KB-1")
	if !strings.Contains(byRef, "KB-2") {
		t.Fatalf("add output = %q", byRef)
	}
	if got := e.jsonOf("show", "auth-t")["id"]; got != "KB-2" {
		t.Fatalf("show by ref = %v, want KB-2", got)
	}
}

func TestE2EAddValidation(t *testing.T) {
	e := newEnv(t)
	e.fail(1, "add", "spike", "x")
	e.fail(1, "add", "task", "x", "--prio", "urgent")
	e.fail(3, "add", "task", "x", "--points", "4")
	e.fail(3, "add", "subtask", "orphan")
	e.fail(2, "add", "task", "x", "--parent", "KB-9")
}

func TestE2EMoveRejectedExit3(t *testing.T) {
	e := newEnv(t)
	e.ok("add", "task", "t", "--test", "a", "--test", "b")
	_, errOut := e.fail(3, "move", "KB-1", "review")
	for _, want := range []string{"cannot move KB-1 to review", `test 1 "a" is pending`, `test 2 "b" is pending`} {
		if !strings.Contains(errOut, want) {
			t.Errorf("stderr lacks %q:\n%s", want, errOut)
		}
	}
	e.fail(1, "move", "KB-1", "in-progress")
}

func TestE2EVersionConflictExit4(t *testing.T) {
	e := newEnv(t)
	e.ok("add", "task", "t")
	e.fail(4, "move", "KB-1", "todo", "--if-version", "3")
	e.ok("move", "KB-1", "todo", "--if-version", "1")
	e.fail(4, "edit", "KB-1", "--title", "x", "--if-version", "1")
}

func TestE2EClaimedExit5(t *testing.T) {
	e := newEnv(t)
	e.ok("add", "task", "t", "--status", "todo")
	e.as("agent-1", "claim", "KB-1")
	e.fail(5, "--as", "agent-2", "claim", "KB-1")
	e.now = e.now.Add(16 * time.Minute)
	e.as("agent-2", "claim", "KB-1")
}

func TestE2ENotFoundExit2(t *testing.T) {
	e := newEnv(t)
	for _, args := range [][]string{
		{"show", "KB-1"}, {"move", "KB-1", "todo"}, {"claim", "KB-1"}, {"note", "KB-1", "x"}, {"rm", "KB-1"},
	} {
		e.fail(2, args...)
	}
	e.fail(2, "next")
}

func TestE2EDescFromStdin(t *testing.T) {
	e := newEnv(t)
	desc := "Line one.\n\n- bullet\n"
	if _, errOut, code := e.run(desc, "add", "task", "t", "--desc", "-"); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	if got := e.jsonOf("show", "KB-1")["description"]; got != desc {
		t.Fatalf("description = %q, want %q", got, desc)
	}
}

// TestE2EAgentSession replays the sub agent session of SPEC.md §6.3.
func TestE2EAgentSession(t *testing.T) {
	e := newEnv(t)
	e.ok("add", "story", "Token login")
	e.ok("add", "task", "Token login task", "--parent", "KB-1", "--status", "todo", "--test", "rejects expired token")
	e.vars["KB_ACTOR"] = "agent-auth"
	next := e.jsonOf("next", "--claim")
	if next["id"] != "KB-2" || next["status"] != "doing" || next["assignee"] != "agent-auth" {
		t.Fatalf("next --claim = %v", next)
	}
	e.ok("phase", "KB-2", "red")
	e.ok("test", "KB-2", "fail", "rejects expired token")
	e.fail(3, "move", "KB-2", "review")
	e.ok("phase", "KB-2", "green")
	e.ok("test", "KB-2", "pass", "1")
	e.ok("note", "KB-2", "validation moved into the middleware")
	moved := e.jsonOf("move", "KB-2", "review")
	if moved["from"] != "doing" || moved["to"] != "review" {
		t.Fatalf("move JSON = %v", moved)
	}
	if got := e.jsonOf("show", "KB-1")["status"]; got != "review" {
		t.Fatalf("story status = %v, want review (derived)", got)
	}
}

func TestE2EEditLinkAcceptRm(t *testing.T) {
	e := newEnv(t)
	e.ok("add", "story", "s")
	e.ok("add", "task", "a", "--parent", "KB-1")
	e.ok("add", "task", "b")
	e.ok("edit", "KB-3", "--title", "renamed", "--prio", "low", "--label", "x", "--label", "y", "--parent", "KB-1")
	item := e.jsonOf("show", "KB-3")
	if item["title"] != "renamed" || item["priority"] != "low" || item["parent"] != "KB-1" {
		t.Fatalf("edited = %v", item)
	}
	e.ok("link", "KB-2", "relates", "KB-3")
	e.fail(3, "link", "KB-2", "relates", "KB-2")
	e.ok("unlink", "KB-2", "relates", "KB-3")
	e.fail(2, "unlink", "KB-2", "relates", "KB-3")
	e.ok("accept", "KB-1", "add", "works")
	e.ok("accept", "KB-1", "check", "1")
	if got := e.jsonOf("show", "KB-1")["acceptance"].([]any)[0].(map[string]any)["done"]; got != true {
		t.Fatalf("criterion not checked: %v", got)
	}
	e.ok("accept", "KB-1", "uncheck", "1")
	e.ok("test", "KB-2", "add", "works")
	e.fail(3, "rm", "KB-1")
	if out := e.ok("rm", "KB-1", "--recursive"); !strings.Contains(out, "KB-3") {
		t.Fatalf("rm output = %q", out)
	}
	e.ok("touch", "--help")
}

func TestE2EClaimReleaseTouch(t *testing.T) {
	e := newEnv(t)
	e.ok("add", "task", "t", "--status", "todo")
	e.as("agent-1", "claim", "KB-1")
	e.now = e.now.Add(10 * time.Minute)
	e.as("agent-1", "touch", "KB-1")
	if got := e.jsonOf("show", "KB-1")["claim"].(map[string]any)["expires_at"]; got != "2026-11-09T14:25:00Z" {
		t.Fatalf("expires_at = %v", got)
	}
	e.as("agent-1", "release", "KB-1")
	if got := e.jsonOf("show", "KB-1"); got["status"] != "todo" || got["claim"] != nil {
		t.Fatalf("released = %v", got)
	}
}

func TestE2EMovePosition(t *testing.T) {
	e := newEnv(t)
	for _, title := range []string{"a", "b", "c"} {
		e.ok("add", "task", title)
	}
	e.ok("move", "KB-3", "backlog", "--position", "0")
	if got := e.ok("--quiet", "list", "--status", "backlog", "--sort", "position"); got != "KB-3\nKB-1\nKB-2\n" {
		t.Fatalf("order = %q", got)
	}
}

func TestE2EListFilters(t *testing.T) {
	e := sampleBoard(t)
	cases := []struct {
		args []string
		want string
	}{
		{nil, "KB-1\nKB-2\nKB-3\nKB-4\n"},
		{[]string{"--status", "doing"}, "KB-1\nKB-2\nKB-3\n"},
		{[]string{"--type", "task"}, "KB-3\nKB-4\n"},
		{[]string{"--assignee", "agent-auth"}, "KB-3\n"},
		{[]string{"--label", "api"}, "KB-3\n"},
		{[]string{"--blocked"}, "KB-4\n"},
		{[]string{"--parent", "KB-1"}, "KB-2\nKB-3\nKB-4\n"},
	}
	for _, c := range cases {
		if got := e.ok(append([]string{"--quiet", "list"}, c.args...)...); got != c.want {
			t.Errorf("list %v = %q, want %q", c.args, got, c.want)
		}
	}
	if got := e.ok("--quiet", "list", "--stale"); got != "" {
		t.Errorf("list --stale before expiry = %q, want nothing", got)
	}
	e.now = e.now.Add(time.Hour)
	if got := e.ok("--quiet", "list", "--stale"); got != "KB-3\n" {
		t.Errorf("list --stale = %q", got)
	}
}

func TestE2ETreeGolden(t *testing.T) {
	e := sampleBoard(t)
	e.ok("add", "task", "Health check", "--status", "done")
	golden(t, "tree_text", e.ok("tree"))
	golden(t, "tree_sub_text", e.ok("tree", "KB-2"))
}

func TestE2ENextClaim(t *testing.T) {
	e := sampleBoard(t)
	// KB-3 is claimed and KB-4 is blocked by it: nothing is available.
	e.fail(2, "--as", "agent-2", "next")
	e.now = e.now.Add(time.Hour)
	got := e.jsonOf("--as", "agent-2", "next", "--claim")
	if got["id"] != "KB-3" || got["assignee"] != "agent-2" {
		t.Fatalf("next --claim after expiry = %v", got)
	}
	e.fail(2, "next", "--type", "bug")
}

func TestE2EShowIncludesHistory(t *testing.T) {
	e := sampleBoard(t)
	out := e.ok("show", "KB-3")
	for _, want := range []string{"claimed", "phase → red", "validation moved into the middleware", "agent-auth"} {
		if !strings.Contains(out, want) {
			t.Errorf("show lacks %q:\n%s", want, out)
		}
	}
}

func TestShowListsLinksToFinishedItems(t *testing.T) {
	e := newEnv(t)
	e.ok("add", "task", "blocker")
	e.ok("add", "task", "blocked")
	e.ok("link", "KB-1", "blocks", "KB-2")
	e.ok("move", "KB-1", "done")
	out := e.ok("show", "KB-2")
	if !strings.Contains(out, "blocked by") || !strings.Contains(out, "KB-1 blocker (done)") {
		t.Fatalf("show hides the link to a finished blocker:\n%s", out)
	}
}

func TestAcceptEditPrintsUpdatedItem(t *testing.T) {
	e := newEnv(t)
	e.ok("add", "task", "t", "--accept", "old")
	e.ok("accept", "KB-1", "check", "1")
	out := e.ok("accept", "KB-1", "edit", "1", "new text")
	if !strings.Contains(out, "acceptance 0/1 checked") {
		t.Fatalf("output = %q", out)
	}
	if show := e.ok("show", "KB-1"); !strings.Contains(show, "1 [ ] new text") {
		t.Fatalf("show = %q", show)
	}
}

func TestAcceptEditJSON(t *testing.T) {
	e := newEnv(t)
	e.ok("add", "task", "t", "--accept", "old")
	e.ok("accept", "KB-1", "check", "1")
	item := e.jsonOf("accept", "KB-1", "edit", "1", "new text")
	crit := item["acceptance"].([]any)[0].(map[string]any)
	if item["id"] != "KB-1" || crit["text"] != "new text" || crit["done"] != false {
		t.Fatalf("item = %v", item)
	}
}

func TestAcceptEditExitCodes(t *testing.T) {
	e := newEnv(t)
	e.ok("add", "task", "t", "--accept", "a", "--accept", "b")
	e.fail(2, "accept", "KB-1", "edit", "3", "x")
	e.fail(3, "accept", "KB-1", "edit", "1", "   ")
	e.fail(3, "accept", "KB-1", "edit", "1", "b")
	e.fail(4, "accept", "KB-1", "edit", "1", "x", "--if-version", "99")
	e.ok("accept", "KB-1", "edit", "1", "x", "--if-version", "1")
}

func TestAcceptEditWithoutTextIsUsageError(t *testing.T) {
	e := newEnv(t)
	e.ok("add", "task", "t", "--accept", "a")
	e.fail(1, "accept", "KB-1", "edit", "1")
	e.fail(1, "accept", "KB-1", "edit", "1", "x", "y")
	e.fail(1, "accept", "KB-1", "edit", "one", "x")
}

func TestAcceptRmJSON(t *testing.T) {
	e := newEnv(t)
	e.ok("add", "task", "t", "--accept", "a", "--accept", "b")
	item := e.jsonOf("accept", "KB-1", "rm", "1")
	acc := item["acceptance"].([]any)
	if item["id"] != "KB-1" || len(acc) != 1 || acc[0].(map[string]any)["text"] != "b" {
		t.Fatalf("item = %v", item)
	}
}

func TestAcceptRmExitCodes(t *testing.T) {
	e := newEnv(t)
	e.ok("add", "task", "t", "--accept", "a", "--accept", "b")
	e.fail(2, "accept", "KB-1", "rm", "3")
	e.fail(4, "accept", "KB-1", "rm", "1", "--if-version", "99")
	e.ok("accept", "KB-1", "rm", "1", "--if-version", "1")
}

func TestAcceptRmWrongArgumentsIsUsageError(t *testing.T) {
	e := newEnv(t)
	e.ok("add", "task", "t", "--accept", "a", "--accept", "b")
	e.fail(1, "accept", "KB-1", "rm", "1", "2")
	e.fail(1, "accept", "KB-1", "rm", "one")
}

func TestTestEditRenamesAndSetsPending(t *testing.T) {
	e := newEnv(t)
	e.ok("add", "task", "t", "--test", "a", "--test", "b")
	e.ok("test", "KB-1", "pass", "2")
	item := e.jsonOf("test", "KB-1", "edit", "b", "better b")
	tests := item["tests"].([]any)
	second := tests[1].(map[string]any)
	if item["id"] != "KB-1" || second["name"] != "better b" || second["state"] != "pending" {
		t.Fatalf("item = %v", item)
	}
}

func TestTestRmRemovesAndKeepsOrder(t *testing.T) {
	e := newEnv(t)
	e.ok("add", "task", "t", "--test", "a", "--test", "b", "--test", "c")
	item := e.jsonOf("test", "KB-1", "rm", "1")
	tests := item["tests"].([]any)
	if len(tests) != 2 || tests[0].(map[string]any)["name"] != "b" || tests[1].(map[string]any)["name"] != "c" {
		t.Fatalf("item = %v", item)
	}
	out := e.ok("test", "KB-1", "rm", "c")
	if !strings.Contains(out, "tests 0/1 passing") {
		t.Fatalf("output = %q", out)
	}
}

func TestTestEditAndRmExitCodes(t *testing.T) {
	e := newEnv(t)
	e.ok("add", "task", "t", "--test", "a", "--test", "b")
	e.fail(2, "test", "KB-1", "edit", "3", "x")
	e.fail(2, "test", "KB-1", "rm", "nope")
	e.fail(3, "test", "KB-1", "edit", "1", "   ")
	e.fail(3, "test", "KB-1", "edit", "1", "b")
	e.fail(4, "test", "KB-1", "edit", "1", "x", "--if-version", "99")
	e.fail(4, "test", "KB-1", "rm", "1", "--if-version", "99")
	e.ok("test", "KB-1", "edit", "1", "x", "--if-version", "1")
	e.ok("test", "KB-1", "rm", "1", "--if-version", "2")
}

func TestTestEditAndRmWrongArgumentsIsUsageError(t *testing.T) {
	e := newEnv(t)
	e.ok("add", "task", "t", "--test", "a", "--test", "b")
	e.fail(1, "test", "KB-1", "edit", "1")
	e.fail(1, "test", "KB-1", "edit", "1", "x", "y")
	e.fail(1, "test", "KB-1", "rm", "1", "2")
}
