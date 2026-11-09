package core

import (
	"errors"
	"fmt"
	"testing"
)

func TestHierarchyMatrix(t *testing.T) {
	// allowed[child] lists the parent types accepted for child; "none"
	// means the child may be a root item.
	allowed := map[Type]map[string]bool{
		TypeEpic:    {"none": true},
		TypeStory:   {"none": true, "epic": true},
		TypeTask:    {"none": true, "story": true},
		TypeBug:     {"none": true, "story": true},
		TypeSubtask: {"task": true, "bug": true},
	}
	parents := append([]*Type{nil}, ptrs(Types())...)
	for _, child := range Types() {
		for _, parent := range parents {
			name := "none"
			if parent != nil {
				name = parent.String()
			}
			t.Run(fmt.Sprintf("%s under %s", child, name), func(t *testing.T) {
				err := ValidateParent(child, parent)
				if allowed[child][name] && err != nil {
					t.Fatalf("expected allowed, got %v", err)
				}
				if !allowed[child][name] && !errors.Is(err, ErrRule) {
					t.Fatalf("expected ErrRule, got %v", err)
				}
			})
		}
	}
}

func ptrs(ts []Type) []*Type {
	out := make([]*Type, len(ts))
	for i := range ts {
		out[i] = &ts[i]
	}
	return out
}

func TestSubtaskRequiresParent(t *testing.T) {
	err := ValidateParent(TypeSubtask, nil)
	if !errors.Is(err, ErrRule) {
		t.Fatalf("subtask without parent: err = %v, want ErrRule", err)
	}
}

func TestReparentRejectsDescendant(t *testing.T) {
	s := board(items(
		it("S1", TypeStory, StatusTodo, ""),
		it("T1", TypeTask, StatusTodo, "S1"),
		it("T2", TypeTask, StatusTodo, ""),
		it("ST", TypeSubtask, StatusTodo, "T1"),
	))
	// A task under its own subtask is both a cycle and a hierarchy error;
	// the cycle must be reported.
	err := ValidateReparent(s, "T1", "ST")
	if !errors.Is(err, ErrRule) {
		t.Fatalf("err = %v, want ErrRule", err)
	}
	// Valid move: the subtask goes under another task.
	if err := ValidateReparent(s, "ST", "T2"); err != nil {
		t.Fatalf("valid reparent rejected: %v", err)
	}
	// Moving a task to the root is allowed.
	if err := ValidateReparent(s, "T1", ""); err != nil {
		t.Fatalf("reparent to root rejected: %v", err)
	}
	// Unknown parent.
	if err := ValidateReparent(s, "T1", "NOPE"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown parent: err = %v, want ErrNotFound", err)
	}
}

func TestReparentDetectsDeepCycle(t *testing.T) {
	s := board(items(
		it("E", TypeEpic, StatusTodo, ""),
		it("S", TypeStory, StatusTodo, "E"),
	))
	if err := ValidateReparent(s, "E", "S"); !errors.Is(err, ErrRule) {
		t.Fatalf("epic under its own story: err = %v, want ErrRule", err)
	}
}

func TestReparentToSelf(t *testing.T) {
	s := board(items(it("T", TypeTask, StatusTodo, "")))
	if err := ValidateReparent(s, "T", "T"); !errors.Is(err, ErrRule) {
		t.Fatalf("err = %v, want ErrRule", err)
	}
}
