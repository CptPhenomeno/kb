package cli

import (
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
