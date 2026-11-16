package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

func tagFilterBoard(t *testing.T) *env {
	e := newEnv(t)
	e.ok("add", "task", "one", "--status", "todo", "--tag", "api")
	e.ok("add", "task", "two", "--status", "todo", "--tag", "apiary")
	e.ok("add", "task", "three", "--status", "todo")
	return e
}

func TestCLIListTagFilterIgnoresCase(t *testing.T) {
	e := tagFilterBoard(t)
	if got := e.ok("--quiet", "list", "--tag", "API"); got != "KB-1\n" {
		t.Fatalf("list --tag API = %q", got)
	}
}

func TestCLIListTagFilterDoesNotMatchSubstring(t *testing.T) {
	e := tagFilterBoard(t)
	if got := e.ok("--quiet", "list", "--tag", "ap"); got != "" {
		t.Fatalf("list --tag ap = %q", got)
	}
}

func TestCLINextTagFilterIgnoresCase(t *testing.T) {
	e := tagFilterBoard(t)
	if got := e.ok("--quiet", "next", "--tag", "APIARY"); got != "KB-2\n" {
		t.Fatalf("next --tag APIARY = %q", got)
	}
}

func TestCLILabelFlagOnEditSetsTagsAndWarns(t *testing.T) {
	e := tagFilterBoard(t)
	out, errOut, code := e.run("", "edit", "KB-3", "--label", "x", "--label", "y")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	_ = out
	if strings.Count(errOut, "warning: --label is deprecated, use --tag\n") != 1 || errOut != "warning: --label is deprecated, use --tag\n" {
		t.Fatalf("stderr = %q", errOut)
	}
	tags := e.jsonOf("show", "KB-3")["tags"].([]any)
	if len(tags) != 2 || tags[0] != "x" || tags[1] != "y" {
		t.Fatalf("tags = %v", tags)
	}
	for _, args := range [][]string{{"add", "task", "n", "--label", "z"}, {"list", "--label", "x"}, {"next", "--label", "x"}} {
		if _, errOut, code := e.run("", args...); code != 0 || errOut != "warning: --label is deprecated, use --tag\n" {
			t.Fatalf("%v: exit %d stderr %q", args, code, errOut)
		}
	}
}

func TestCLILabelFlagWithJSONKeepsStdoutValidJSON(t *testing.T) {
	e := tagFilterBoard(t)
	out, errOut, code := e.run("", "--json", "list", "--label", "api")
	if code != 0 || errOut != "warning: --label is deprecated, use --tag\n" {
		t.Fatalf("exit %d stderr %q", code, errOut)
	}
	var v []map[string]any
	if err := json.Unmarshal([]byte(out), &v); err != nil || len(v) != 1 {
		t.Fatalf("stdout %q: %v", out, err)
	}
}

func TestCLILabelFlagIsHiddenFromTheHelp(t *testing.T) {
	e := newEnv(t)
	for _, cmd := range []string{"add", "edit", "list", "next"} {
		out, errOut, _ := e.run("", cmd, "--help")
		if strings.Contains(out+errOut, "--label") || !strings.Contains(out, "--tag") {
			t.Errorf("%s --help: label shown or tag missing\n%s", cmd, out)
		}
	}
}
