package cli

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"
)

// sampleBoard builds the board used by the golden tests.
func sampleBoard(t *testing.T) *env {
	e := newEnv(t)
	e.ok("add", "epic", "Authentication", "--ref", "auth", "--spec", "SPEC.md#authentication")
	e.ok("add", "story", "Token login", "--parent", "KB-1",
		"--accept", "expired token → 401", "--accept", "valid token → 200")
	e.ok("add", "task", "Validate token in middleware", "--parent", "KB-2", "--status", "todo",
		"--prio", "high", "--points", "3", "--test", "rejects expired token", "--test", "accepts valid token",
		"--tag", "api")
	e.ok("add", "task", "Token refresh", "--parent", "KB-2", "--status", "todo", "--points", "2",
		"--test", "refresh rotates the token", "--desc", "Exchange a refresh token for a new pair.")
	e.ok("link", "KB-3", "blocks", "KB-4")
	e.as("agent-auth", "claim", "KB-3")
	e.now = e.now.Add(2 * time.Minute)
	e.as("agent-auth", "phase", "KB-3", "red")
	e.as("agent-auth", "test", "KB-3", "fail", "1")
	e.as("agent-auth", "note", "KB-3", "validation moved into the middleware")
	return e
}

func TestItemJSONGolden(t *testing.T) {
	e := sampleBoard(t)
	golden(t, "show_json", e.ok("--json", "show", "KB-3"))
}

func TestItemTextGolden(t *testing.T) {
	e := sampleBoard(t)
	golden(t, "show_text", e.ok("show", "KB-3"))
	golden(t, "list_text", e.ok("list"))
}

func TestQuietOutput(t *testing.T) {
	e := sampleBoard(t)
	if got := e.ok("--quiet", "list", "--type", "task"); got != "KB-3\nKB-4\n" {
		t.Fatalf("--quiet list = %q", got)
	}
}

// TestJSONFieldsStable pins the keys of the item JSON (SPEC.md §6.4):
// agents depend on them, so adding or renaming one is a deliberate change.
func TestJSONFieldsStable(t *testing.T) {
	e := sampleBoard(t)
	out := e.ok("--json", "show", "KB-3")
	var v map[string]any
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		t.Fatal(err)
	}
	var keys []string
	for k := range v {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	want := []string{"acceptance", "assignee", "blocked_by", "blocks", "claim", "created_at", "created_by",
		"description", "duplicates", "history", "id", "links", "parent", "points", "position",
		"priority", "ref", "relates", "spec", "status", "tags", "tdd_phase", "tests", "title", "type",
		"updated_at", "updated_by", "version"}
	if !slices.Equal(keys, want) {
		t.Fatalf("item JSON keys:\n got %s\nwant %s", strings.Join(keys, " "), strings.Join(want, " "))
	}
}

// derivedBoard has a story with tasks of 3 and 5 points under an epic.
func derivedBoard(t *testing.T) *env {
	t.Helper()
	e := newEnv(t)
	e.ok("add", "epic", "Epic")
	e.ok("add", "story", "Story", "--parent", "KB-1")
	e.ok("add", "task", "Three", "--parent", "KB-2", "--points", "3")
	e.ok("add", "task", "Five", "--parent", "KB-2", "--points", "5")
	e.ok("add", "story", "Empty story", "--parent", "KB-1")
	return e
}

func TestShowAndTreePrintDerivedPointsForStories(t *testing.T) {
	e := derivedBoard(t)
	for id, want := range map[string]string{"KB-2": "points      8pt (from children)", "KB-1": "points      8pt (from children)"} {
		if out := e.ok("show", id); !strings.Contains(out, want) {
			t.Errorf("show %s lacks %q:\n%s", id, want, out)
		}
	}
	if out := e.ok("show", "KB-5"); !strings.Contains(out, "points      -\n") {
		t.Errorf("show of an empty story:\n%s", out)
	}
	if out := e.ok("show", "KB-3"); !strings.Contains(out, "points      3pt\n") {
		t.Errorf("show of a task:\n%s", out)
	}
	tree := e.ok("tree")
	for _, line := range strings.Split(tree, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "KB-2 ") && !strings.Contains(line, "8pt") {
			t.Errorf("tree line of the story has no 8pt: %q", line)
		}
	}
	if j := e.jsonOf("--json", "show", "KB-2"); j["points"] != nil {
		t.Errorf("story JSON points = %v, want null", j["points"])
	}
}
