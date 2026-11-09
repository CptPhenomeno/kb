package plan

import (
	"errors"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/CptPhenomeno/kb/internal/core"
)

func parseFile(t *testing.T, path string) *core.ImportPlan {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	p, err := Parse(f, path)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return p
}

func TestParseSpecExample(t *testing.T) {
	p := parseFile(t, "testdata/spec_example.yaml")
	var refs []string
	for _, it := range p.Items {
		refs = append(refs, it.Ref)
	}
	if want := []string{"auth", "auth-login", "auth-login-validate", "auth-login-refresh"}; !slices.Equal(refs, want) {
		t.Fatalf("refs = %v, want %v (parents before children)", refs, want)
	}
	story, task := p.Items[1], p.Items[2]
	if story.ParentRef != "auth" || task.ParentRef != "auth-login" || p.Items[0].ParentRef != "" {
		t.Fatalf("parents = %q %q %q", p.Items[0].ParentRef, story.ParentRef, task.ParentRef)
	}
	if *story.Type != core.TypeStory || *story.Points != 5 || len(*story.Acceptance) != 2 {
		t.Fatalf("story = %+v", story)
	}
	if *task.Priority != core.PriorityHigh || !slices.Equal(*task.Tests, []string{"rejects expired token", "accepts valid token"}) {
		t.Fatalf("task = %+v", task)
	}
	if !slices.Equal(task.Blocks, []string{"auth-login-refresh"}) {
		t.Fatalf("blocks = %v", task.Blocks)
	}
	if task.Pos != "testdata/spec_example.yaml:17" {
		t.Fatalf("pos = %q, want the line of the item", task.Pos)
	}
}

func TestDefaultsApplied(t *testing.T) {
	p := parseFile(t, "testdata/spec_example.yaml")
	if *p.Items[0].Spec != "SPEC.md#authentication" {
		t.Errorf("explicit spec overridden: %q", *p.Items[0].Spec)
	}
	if *p.Items[1].Spec != "SPEC.md" {
		t.Errorf("default spec not applied: %v", p.Items[1].Spec)
	}
	for _, it := range p.Items {
		if it.Status == nil || *it.Status != core.StatusTodo {
			t.Errorf("%s: default status not applied", it.Ref)
		}
	}
	p, err := Parse(strings.NewReader("items:\n  - ref: a\n    type: task\n    title: A\n"), "p.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if p.Items[0].Spec != nil || p.Items[0].Status != nil {
		t.Errorf("no defaults expected without plan-level keys: %+v", p.Items[0])
	}
}

func TestParseUnknownFieldRejected(t *testing.T) {
	_, err := Parse(strings.NewReader("items:\n  - ref: a\n    type: task\n    title: A\n    owner: me\n"), "p.yaml")
	if err == nil || !strings.Contains(err.Error(), "owner") {
		t.Fatalf("err = %v, want an error naming the unknown field", err)
	}
}

func TestParseErrorHasPosition(t *testing.T) {
	_, err := Parse(strings.NewReader("items:\n  - ref: a\n    type: [task\n"), "p.yaml")
	if err == nil || !strings.Contains(err.Error(), "line") {
		t.Fatalf("syntax error = %v, want a line number", err)
	}
	_, err = Parse(strings.NewReader("items:\n  - ref: a\n    type: task\n    title: A\n  - ref: b\n    type: spike\n    title: B\n"), "p.yaml")
	if !errors.Is(err, core.ErrRule) || !strings.Contains(err.Error(), "p.yaml:5") || !strings.Contains(err.Error(), "spike") {
		t.Fatalf("invalid type error = %v, want a rule error at p.yaml:5", err)
	}
}

func TestParseCollectsAllFieldErrors(t *testing.T) {
	src := "items:\n  - ref: a\n    type: spike\n    title: A\n  - type: task\n    title: B\n    priority: urgent\n"
	_, err := Parse(strings.NewReader(src), "p.yaml")
	var re *core.RuleError
	if !errors.As(err, &re) || len(re.Unmet) != 3 {
		t.Fatalf("err = %v, want 3 problems (type, missing ref, priority)", err)
	}
}

func TestParseTopLevelParent(t *testing.T) {
	p, err := Parse(strings.NewReader("items:\n  - ref: a\n    type: task\n    title: A\n    parent: KB-7\n"), "p.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if p.Items[0].ParentRef != "KB-7" {
		t.Fatalf("parent = %q", p.Items[0].ParentRef)
	}
	_, err = Parse(strings.NewReader("items:\n  - ref: a\n    type: story\n    title: A\n    children:\n      - ref: b\n        type: task\n        title: B\n        parent: KB-7\n"), "p.yaml")
	if !errors.Is(err, core.ErrRule) {
		t.Fatalf("parent on a nested item: err = %v, want ErrRule", err)
	}
}
