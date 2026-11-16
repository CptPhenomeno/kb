package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const examplePlan = "../plan/testdata/spec_example.yaml"

func TestE2EImportGolden(t *testing.T) {
	e := newEnv(t)
	golden(t, "import_text", e.ok("import", examplePlan))
	golden(t, "import_tree", e.ok("tree"))
	// Re-importing the same plan with --update changes nothing.
	out := e.ok("import", examplePlan, "--update")
	if !strings.Contains(out, "0 created · 0 updated · 4 unchanged") {
		t.Fatalf("re-import = %q", out)
	}
}

func TestE2EImportFromStdin(t *testing.T) {
	e := newEnv(t)
	plan, err := os.ReadFile(examplePlan)
	if err != nil {
		t.Fatal(err)
	}
	if _, errOut, code := e.run(string(plan), "import", "-"); code != 0 {
		t.Fatalf("import -: exit %d: %s", code, errOut)
	}
	if got := e.ok("--quiet", "list"); got != "KB-1\nKB-2\nKB-3\nKB-4\n" {
		t.Fatalf("items = %q", got)
	}
}

func TestE2EImportInvalidExit3(t *testing.T) {
	e := newEnv(t)
	path := filepath.Join(t.TempDir(), "plan.yaml")
	body := "items:\n  - {ref: a, type: task, title: A, points: 4}\n  - {ref: b, type: task, title: B, blocks: [zzz]}\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	_, errOut := e.fail(3, "import", path)
	for _, want := range []string{"2 problems", "plan.yaml:2 (a): points must be one of", `plan.yaml:3 (b): unknown item "zzz"`} {
		if !strings.Contains(errOut, want) {
			t.Errorf("stderr lacks %q:\n%s", want, errOut)
		}
	}
	if got := e.ok("--quiet", "list"); got != "" {
		t.Fatalf("a failed import wrote %q", got)
	}
	e.fail(1, "import", filepath.Join(t.TempDir(), "missing.yaml"))
	bad := filepath.Join(t.TempDir(), "bad.yaml")
	if err := os.WriteFile(bad, []byte("items: [\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	e.fail(1, "import", bad)
}

func TestE2EImportDryRunJSON(t *testing.T) {
	e := newEnv(t)
	got := e.jsonOf("import", examplePlan, "--dry-run")
	items, _ := got["items"].([]any)
	if got["dry_run"] != true || got["created"] != 4.0 || len(items) != 4 {
		t.Fatalf("dry run JSON = %v", got)
	}
	first := items[0].(map[string]any)
	if first["ref"] != "auth" || first["id"] != "KB-1" || first["result"] != "created" {
		t.Fatalf("first item = %v", first)
	}
	if out := e.ok("--quiet", "list"); out != "" {
		t.Fatalf("dry run wrote %q", out)
	}
}

const tagPlan = "items:\n  - {ref: a, type: task, title: A, %s: [api, ui]}\n  - {ref: b, type: task, title: B, %s: [docs, API]}\n"

func writePlan(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "plan.yaml")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func tagColors(t *testing.T, e *env) map[string]string {
	t.Helper()
	var got []map[string]any
	if err := json.Unmarshal([]byte(e.ok("--json", "tag", "list")), &got); err != nil {
		t.Fatal(err)
	}
	m := map[string]string{}
	for _, g := range got {
		m[g["name"].(string)] = g["color"].(string)
	}
	return m
}

func TestE2EImportPlanTagsAssignedAndCreatedWithRotation(t *testing.T) {
	e := newEnv(t)
	path := writePlan(t, fmt.Sprintf(tagPlan, "tags", "tags"))
	_, errOut, code := e.run("", "import", path)
	if code != 0 || errOut != "" {
		t.Fatalf("exit %d stderr %q", code, errOut)
	}
	tags := e.jsonOf("show", "KB-2")["tags"].([]any)
	if len(tags) != 2 || tags[0] != "docs" || tags[1] != "api" {
		t.Fatalf("tags = %v", tags)
	}
	colors := tagColors(t, e)
	if len(colors) != 3 || colors["api"] != "red" || colors["ui"] != "orange" || colors["docs"] != "yellow" {
		t.Fatalf("colors = %v", colors)
	}
}

func TestE2EImportPlanLabelsWarnsOnStderrOnly(t *testing.T) {
	e := newEnv(t)
	path := writePlan(t, fmt.Sprintf(tagPlan, "labels", "labels"))
	out, errOut, code := e.run("", "--json", "import", path, "--dry-run")
	if code != 0 || !strings.HasPrefix(errOut, "warning: ") || !strings.Contains(errOut, "labels is deprecated, use tags") {
		t.Fatalf("dry run: exit %d stderr %q", code, errOut)
	}
	var v map[string]any
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		t.Fatalf("stdout is not clean JSON: %v\n%s", err, out)
	}
	_, errOut, code = e.run("", "import", path)
	if code != 0 || strings.Count(errOut, "warning:") != 1 {
		t.Fatalf("import: exit %d stderr %q", code, errOut)
	}
	if tags := e.jsonOf("show", "KB-1")["tags"].([]any); len(tags) != 2 || tags[0] != "api" {
		t.Fatalf("tags = %v", tags)
	}
}

func TestE2EImportPlanTagsAndLabelsExit3(t *testing.T) {
	e := newEnv(t)
	path := writePlan(t, "items:\n  - {ref: a, type: task, title: A, tags: [x], labels: [y]}\n")
	_, errOut := e.fail(3, "import", path)
	if !strings.Contains(errOut, "plan.yaml:2 (a)") || !strings.Contains(errOut, "tags and labels") {
		t.Fatalf("stderr = %q", errOut)
	}
}

func TestE2EImportDryRunCreatesNoTag(t *testing.T) {
	e := newEnv(t)
	e.ok("import", writePlan(t, fmt.Sprintf(tagPlan, "tags", "tags")), "--dry-run")
	if out := e.ok("tag", "list"); out != "" {
		t.Fatalf("dry run created tags: %q", out)
	}
}

func TestE2EImportUpdateReplacesTags(t *testing.T) {
	e := newEnv(t)
	e.ok("import", writePlan(t, "items:\n  - {ref: a, type: task, title: A, tags: [api, ui]}\n"))
	e.ok("import", writePlan(t, "items:\n  - {ref: a, type: task, title: A, tags: [docs]}\n"), "--update")
	tags := e.jsonOf("show", "KB-1")["tags"].([]any)
	if len(tags) != 1 || tags[0] != "docs" {
		t.Fatalf("tags = %v", tags)
	}
	if _, ok := tagColors(t, e)["docs"]; !ok {
		t.Fatal("docs tag not created by update")
	}
}

func TestE2EImportPointsOnStoryFailsValidation(t *testing.T) {
	e := newEnv(t)
	path := filepath.Join(t.TempDir(), "plan.yaml")
	body := "items:\n  - {ref: s, type: story, title: S, points: 5}\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	_, errOut := e.fail(3, "import", path)
	if !strings.Contains(errOut, "plan.yaml:2 (s): points are not allowed on a story") {
		t.Errorf("stderr: %s", errOut)
	}
}
