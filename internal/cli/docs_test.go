package cli

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// codeIn returns the code of a markdown document: fenced blocks and
// inline code spans.
func codeIn(doc string) []string {
	var out []string
	for _, m := range regexp.MustCompile("(?s)```[a-z]*\n(.*?)```").FindAllStringSubmatch(doc, -1) {
		out = append(out, m[1])
	}
	prose := regexp.MustCompile("(?s)```.*?```").ReplaceAllString(doc, "")
	for _, m := range regexp.MustCompile("`([^`\n]+)`").FindAllStringSubmatch(prose, -1) {
		out = append(out, m[1])
	}
	return out
}

func TestReadmeCommandsExist(t *testing.T) {
	data, err := os.ReadFile("../../README.md")
	if err != nil {
		t.Fatal(err)
	}
	root := (&cli{app: DefaultApp()}).rootCmd()
	cmdRe := regexp.MustCompile(`(?m)(?:^|[\s$|(])kb ([a-z][a-z-]*)`)
	seen := map[string]bool{}
	for _, code := range codeIn(string(data)) {
		for _, m := range cmdRe.FindAllStringSubmatch(code, -1) {
			name := m[1]
			if seen[name] {
				continue
			}
			seen[name] = true
			if c, _, err := root.Find([]string{name}); err != nil || c == root {
				t.Errorf("README uses `kb %s`, which is not a command", name)
			}
		}
	}
	for _, must := range []string{"init", "guide", "import", "next", "move"} {
		if !seen[must] {
			t.Errorf("README does not show `kb %s`", must)
		}
	}
}

// docs/cli-json.md documents every field of the item JSON.
func TestCLIJSONDocumentsEveryItemField(t *testing.T) {
	data, err := os.ReadFile("../../docs/cli-json.md")
	if err != nil {
		t.Fatal(err)
	}
	e := newEnv(t)
	e.ok("add", "task", "Documented", "--test", "a", "--accept", "b")
	for key := range e.jsonOf("show", "KB-1") {
		if !strings.Contains(string(data), `"`+key+`"`) {
			t.Errorf("docs/cli-json.md does not document the item field %q", key)
		}
	}
}

// The guide is pasted into CLAUDE.md or AGENTS.md: lists must be markdown
// lists, not lines that a renderer joins into one paragraph.
func TestGuideIsPasteableMarkdown(t *testing.T) {
	e := newEnv(t)
	out := e.ok("guide")
	if !strings.HasPrefix(out, "# ") {
		t.Fatal("the guide does not start with a markdown heading")
	}
	for code := 0; code <= 5; code++ {
		if !regexp.MustCompile(`(?m)^- ` + string(rune('0'+code)) + ` `).MatchString(out) {
			t.Errorf("exit code %d is not a markdown list item", code)
		}
	}
	for _, want := range []string{"items:", "children:", "kb list --stale", "kb log --follow"} {
		if !strings.Contains(out, want) {
			t.Errorf("guide does not mention %q", want)
		}
	}
}

// The plan example of the guide imports as it is.
func TestGuidePlanExampleImports(t *testing.T) {
	e := newEnv(t)
	out := e.ok("guide")
	_, rest, ok := strings.Cut(out, "## Plan format")
	if !ok {
		t.Fatal("the guide has no plan format section")
	}
	var plan []string
	for _, l := range strings.Split(rest, "\n") {
		if strings.HasPrefix(l, "## ") {
			break
		}
		if strings.HasPrefix(l, "    ") {
			plan = append(plan, strings.TrimPrefix(l, "    "))
		}
	}
	path := filepath.Join(t.TempDir(), "plan.yaml")
	if err := os.WriteFile(path, []byte(strings.Join(plan, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	e.ok("import", path)
	if got := e.ok("list", "--quiet"); strings.Count(got, "KB-") != 4 {
		t.Fatalf("the plan example created %q, want 4 items", got)
	}
}
