package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLogJSONLines(t *testing.T) {
	e := sampleBoard(t)
	out := e.ok("--json", "log", "KB-3")
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 7 {
		t.Fatalf("got %d lines, want 7 events of KB-3:\n%s", len(lines), out)
	}
	for _, l := range lines {
		var ev map[string]any
		if err := json.Unmarshal([]byte(l), &ev); err != nil {
			t.Fatalf("line is not a JSON object: %q", l)
		}
		if ev["item"] != "KB-3" {
			t.Fatalf("event of another item: %v", ev)
		}
	}
}

func TestLogText(t *testing.T) {
	e := sampleBoard(t)
	out := e.ok("log", "--limit", "2")
	want := "2026-11-09 14:02  agent-auth  KB-3  test \"rejects expired token\" → failing\n" +
		"2026-11-09 14:02  agent-auth  KB-3  note: validation moved into the middleware\n"
	if out != want {
		t.Fatalf("log =\n%s\nwant\n%s", out, want)
	}
	if got := e.ok("--quiet", "log", "--since", "1m"); strings.Count(got, "\n") != 3 {
		t.Fatalf("log --since 1m:\n%s", got)
	}
	if got := e.ok("--quiet", "log", "--since", "2026-11-09T14:01:00Z"); strings.Count(got, "\n") != 3 {
		t.Fatalf("log --since <time>:\n%s", got)
	}
	e.fail(1, "log", "--since", "yesterday")
}

func TestExportGolden(t *testing.T) {
	e := sampleBoard(t)
	e.ok("add", "bug", "CORS headers missing on 404", "--status", "todo")
	out := e.ok("export")
	if !strings.Contains(out, "board.md") {
		t.Fatalf("export output = %q", out)
	}
	body, err := os.ReadFile(filepath.Join(e.dir, ".kanban", "board.md"))
	if err != nil {
		t.Fatal(err)
	}
	golden(t, "export_md", string(body))
	if stdout := e.ok("export", "--out", "-"); stdout != string(body) {
		t.Fatal("export --out - differs from the file")
	}
}

func TestExportDeterministic(t *testing.T) {
	e := sampleBoard(t)
	first := e.ok("export", "--out", "-")
	e.now = e.now.Add(48 * time.Hour)
	if second := e.ok("export", "--out", "-"); second != first {
		t.Fatal("export changed without board changes")
	}
}

func TestGuideMentionsAllAgentCommands(t *testing.T) {
	e := newEnv(t)
	out := e.ok("guide")
	for _, want := range []string{"KB_ACTOR", "kb next --claim", "kb phase", "kb test", "kb accept",
		"kb note", "kb move", "kb release", "kb touch", "kb import", "--json", "exit code",
		"description", "Done when"} {
		if !strings.Contains(out, want) {
			t.Errorf("guide does not mention %q", want)
		}
	}
	for code := 0; code <= 5; code++ {
		if !strings.Contains(out, "- "+string(rune('0'+code))+" ") {
			t.Errorf("guide does not explain exit code %d", code)
		}
	}
}
