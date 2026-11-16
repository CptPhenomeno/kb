package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCLITagAddJSONPrintsTheTag(t *testing.T) {
	e := newEnv(t)
	v := e.jsonOf("tag", "add", "api")
	if v["name"] != "api" || v["color"] != "red" || v["cards"] != float64(0) {
		t.Fatalf("tag = %v", v)
	}
	v = e.jsonOf("tag", "add", "ui", "--color", "blue")
	if v["color"] != "blue" {
		t.Fatalf("tag = %v", v)
	}
	_, errOut := e.fail(3, "tag", "add", "x", "--color", "nope")
	if !strings.Contains(errOut, "red") || !strings.Contains(errOut, "pink") {
		t.Fatalf("stderr does not list the colors: %s", errOut)
	}
	e.fail(3, "tag", "add", "a,b")
}

func TestCLITagAddOfExistingNameExitsWithConflict(t *testing.T) {
	e := newEnv(t)
	e.ok("tag", "add", "api")
	e.fail(4, "tag", "add", "API")
}

func TestCLITagListJSONListsNameColorAndCardsByName(t *testing.T) {
	e := newEnv(t)
	if out := e.ok("tag", "list"); out != "" {
		t.Fatalf("fresh board prints %q", out)
	}
	e.ok("tag", "add", "zeta")
	e.ok("tag", "add", "Alpha", "--color", "cyan")
	out := e.ok("--json", "tag", "list")
	var got []map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0]["name"] != "Alpha" || got[0]["color"] != "cyan" || got[0]["cards"] != float64(0) ||
		got[1]["name"] != "zeta" || got[1]["color"] != "red" {
		t.Fatalf("list = %v", got)
	}
	text := e.ok("tag", "list")
	if !strings.Contains(text, "Alpha") || strings.Index(text, "Alpha") > strings.Index(text, "zeta") {
		t.Fatalf("text list:\n%s", text)
	}
}
