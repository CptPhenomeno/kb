package core

import (
	"errors"
	"strings"
	"testing"
)

// assertRuleError fails unless err is a rule violation mentioning field.
func assertRuleError(t *testing.T, err error, field string) {
	t.Helper()
	if !errors.Is(err, ErrRule) {
		t.Fatalf("error = %v; want ErrRule", err)
	}
	if !strings.Contains(err.Error(), field) {
		t.Fatalf("error %q does not mention field %q", err, field)
	}
}

func TestTitleRequired(t *testing.T) {
	for _, title := range []string{"", "   ", "\t\n"} {
		err := ValidateItem(&Item{Type: TypeTask, Title: title})
		assertRuleError(t, err, "title")
	}
	if err := ValidateItem(&Item{Type: TypeTask, Title: "Token refresh"}); err != nil {
		t.Fatalf("valid item rejected: %v", err)
	}
}

func TestPointsFibonacciOnly(t *testing.T) {
	for _, p := range []int{0, 1, 2, 3, 5, 8, 13} {
		if err := ValidateItem(&Item{Type: TypeTask, Title: "x", Points: p}); err != nil {
			t.Errorf("points %d rejected: %v", p, err)
		}
	}
	for _, p := range []int{-1, 4, 6, 21, 100} {
		assertRuleError(t, ValidateItem(&Item{Type: TypeTask, Title: "x", Points: p}), "points")
	}
}

func TestPointsNotAllowedOnEpicAndSubtask(t *testing.T) {
	for _, typ := range []Type{TypeEpic, TypeSubtask} {
		assertRuleError(t, ValidateItem(&Item{Type: typ, Title: "x", Points: 3}), "points")
	}
	for _, typ := range []Type{TypeStory, TypeTask, TypeBug} {
		if err := ValidateItem(&Item{Type: typ, Title: "x", Points: 3}); err != nil {
			t.Errorf("points on %v rejected: %v", typ, err)
		}
	}
}

func TestTestsOnlyOnLeafTypes(t *testing.T) {
	tests := []Test{{Name: "rejects expired token"}}
	for _, typ := range []Type{TypeEpic, TypeStory} {
		assertRuleError(t, ValidateItem(&Item{Type: typ, Title: "x", Tests: tests}), "tests")
	}
	for _, typ := range []Type{TypeTask, TypeSubtask, TypeBug} {
		if err := ValidateItem(&Item{Type: typ, Title: "x", Tests: tests}); err != nil {
			t.Errorf("tests on %v rejected: %v", typ, err)
		}
	}
}

func TestTestNamesRequiredAndUnique(t *testing.T) {
	assertRuleError(t, ValidateItem(&Item{Type: TypeTask, Title: "x", Tests: []Test{{Name: " "}}}), "test")
	dup := []Test{{Name: "a"}, {Name: "a"}}
	assertRuleError(t, ValidateItem(&Item{Type: TypeTask, Title: "x", Tests: dup}), "test")
}

func TestCriteriaTextRequiredAndUnique(t *testing.T) {
	assertRuleError(t, ValidateItem(&Item{Type: TypeStory, Title: "x", Acceptance: []Criterion{{Text: ""}}}), "acceptance")
	dup := []Criterion{{Text: "a"}, {Text: "a"}}
	assertRuleError(t, ValidateItem(&Item{Type: TypeStory, Title: "x", Acceptance: dup}), "acceptance")
}

func TestPhaseOnlyOnLeafTypes(t *testing.T) {
	for _, typ := range []Type{TypeEpic, TypeStory} {
		assertRuleError(t, ValidateItem(&Item{Type: typ, Title: "x", Phase: PhaseRed}), "tdd phase")
	}
	for _, typ := range []Type{TypeTask, TypeSubtask, TypeBug} {
		if err := ValidateItem(&Item{Type: typ, Title: "x", Phase: PhaseGreen}); err != nil {
			t.Errorf("phase on %v rejected: %v", typ, err)
		}
	}
}

func TestErrorKinds(t *testing.T) {
	rule := NewRuleError("cannot move KB-2 to done", "child KB-4 is doing", "acceptance 0/2 checked")
	if !errors.Is(rule, ErrRule) {
		t.Error("RuleError must match ErrRule")
	}
	var re *RuleError
	if !errors.As(rule, &re) || len(re.Unmet) != 2 {
		t.Fatalf("errors.As(RuleError) = %v, unmet = %v", re, re)
	}
	want := "cannot move KB-2 to done: child KB-4 is doing; acceptance 0/2 checked"
	if rule.Error() != want {
		t.Errorf("Error() = %q, want %q", rule.Error(), want)
	}
	for _, sentinel := range []error{ErrNotFound, ErrConflict, ErrClaimed} {
		wrapped := wrapf(sentinel, "item %s", "KB-9")
		if !errors.Is(wrapped, sentinel) {
			t.Errorf("wrapped %v does not match its sentinel", sentinel)
		}
		if errors.Is(wrapped, ErrRule) {
			t.Errorf("%v must not match ErrRule", sentinel)
		}
	}
}
