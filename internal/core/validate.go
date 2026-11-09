package core

import (
	"fmt"
	"slices"
	"strings"
)

// fibonacciPoints is the fixed story-point scale (SPEC.md §3.1).
var fibonacciPoints = []int{1, 2, 3, 5, 8, 13}

// ValidateItem checks the fields of an item that do not depend on the rest
// of the board. Hierarchy and relations are validated separately.
func ValidateItem(it *Item) error {
	if strings.TrimSpace(it.Title) == "" {
		return NewRuleError("title is required")
	}
	if it.Points != 0 {
		if !slices.Contains(fibonacciPoints, it.Points) {
			return NewRuleError(fmt.Sprintf("points must be one of %v, got %d", fibonacciPoints, it.Points))
		}
		if it.Type == TypeEpic || it.Type == TypeSubtask {
			return NewRuleError(fmt.Sprintf("points are not allowed on a %s", it.Type))
		}
	}
	if !it.Type.IsLeafType() {
		if len(it.Tests) > 0 {
			return NewRuleError(fmt.Sprintf("tests are not allowed on a %s", it.Type))
		}
		if it.Phase != PhaseNone {
			return NewRuleError(fmt.Sprintf("tdd phase is not allowed on a %s", it.Type))
		}
	}
	seen := map[string]bool{}
	for _, t := range it.Tests {
		name := strings.TrimSpace(t.Name)
		if name == "" {
			return NewRuleError("test name is required")
		}
		if seen[name] {
			return NewRuleError(fmt.Sprintf("test %q is listed twice", name))
		}
		seen[name] = true
	}
	seen = map[string]bool{}
	for _, c := range it.Acceptance {
		text := strings.TrimSpace(c.Text)
		if text == "" {
			return NewRuleError("acceptance criterion text is required")
		}
		if seen[text] {
			return NewRuleError(fmt.Sprintf("acceptance criterion %q is listed twice", text))
		}
		seen[text] = true
	}
	return nil
}
