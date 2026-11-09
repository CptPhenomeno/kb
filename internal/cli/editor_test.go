package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeEditor writes a shell script that edits the file given as $1 and
// sets it as $EDITOR.
func fakeEditor(t *testing.T, e *env, script string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "editor.sh")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nset -e\n"+script+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	e.vars["EDITOR"] = path
}

func TestEditorRoundTrip(t *testing.T) {
	e := newEnv(t)
	e.ok("add", "story", "s")
	e.ok("add", "task", "old title", "--label", "api", "--desc", "first line")
	fakeEditor(t, e, `sed -i -e 's/^title: .*/title: new title/' -e 's/^priority: .*/priority: high/' "$1"`)
	out := e.ok("edit", "KB-2")
	if !strings.Contains(out, "KB-2 updated") {
		t.Fatalf("output = %q", out)
	}
	item := e.jsonOf("show", "KB-2")
	if item["title"] != "new title" || item["priority"] != "high" || item["description"] != "first line" {
		t.Fatalf("edited item = %v", item)
	}
	if labels := item["labels"].([]any); len(labels) != 1 || labels[0] != "api" {
		t.Fatalf("labels = %v", labels)
	}
	fakeEditor(t, e, `sed -i 's/^parent: .*/parent: KB-1/' "$1"`)
	e.ok("edit", "KB-2")
	if got := e.jsonOf("show", "KB-2")["parent"]; got != "KB-1" {
		t.Fatalf("parent = %v, want KB-1", got)
	}
}

func TestEditorNoChangesNoWrite(t *testing.T) {
	e := newEnv(t)
	e.ok("add", "task", "t")
	fakeEditor(t, e, `true`)
	out := e.ok("edit", "KB-1")
	if !strings.Contains(out, "no changes") {
		t.Fatalf("output = %q", out)
	}
	if v := e.jsonOf("show", "KB-1")["version"]; v != 1.0 {
		t.Fatalf("version = %v, want 1", v)
	}
}

func TestEditorInvalidYAML(t *testing.T) {
	e := newEnv(t)
	e.ok("add", "task", "t")
	fakeEditor(t, e, `printf 'title: [unclosed\n' > "$1"`)
	_, errOut := e.fail(1, "edit", "KB-1")
	if !strings.Contains(errOut, "no change applied") {
		t.Fatalf("stderr = %q", errOut)
	}
	if got := e.jsonOf("show", "KB-1")["title"]; got != "t" {
		t.Fatalf("title = %v, want unchanged", got)
	}
}

func TestEditorRuleViolationKeepsItem(t *testing.T) {
	e := newEnv(t)
	e.ok("add", "task", "t")
	fakeEditor(t, e, `sed -i 's/^points: .*/points: 4/' "$1"`)
	e.fail(3, "edit", "KB-1")
}

func TestEditorFailureAborts(t *testing.T) {
	e := newEnv(t)
	e.ok("add", "task", "t")
	fakeEditor(t, e, `exit 3`)
	_, errOut := e.fail(1, "edit", "KB-1")
	if !strings.Contains(errOut, "editor") {
		t.Fatalf("stderr = %q", errOut)
	}
}
