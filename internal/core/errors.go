package core

import (
	"errors"
	"fmt"
	"strings"
)

// Sentinel errors. The CLI maps each of them to an exit code (SPEC.md §6.1).
var (
	// ErrNotFound means an item, relation or test does not exist (exit 2).
	ErrNotFound = errors.New("not found")
	// ErrRule means a rule of the board was violated (exit 3).
	ErrRule = errors.New("rule violated")
	// ErrConflict means the item changed since the caller read it (exit 4).
	ErrConflict = errors.New("version conflict")
	// ErrClaimed means another actor holds a valid claim on the item (exit 5).
	ErrClaimed = errors.New("claimed by another actor")
)

// RuleError is a rule violation together with the list of unmet
// requirements, so interfaces can show every missing condition at once.
type RuleError struct {
	Msg   string
	Unmet []string
}

// NewRuleError builds a RuleError with an optional list of unmet requirements.
func NewRuleError(msg string, unmet ...string) *RuleError {
	return &RuleError{Msg: msg, Unmet: unmet}
}

func (e *RuleError) Error() string {
	if len(e.Unmet) == 0 {
		return e.Msg
	}
	return e.Msg + ": " + strings.Join(e.Unmet, "; ")
}

// Is makes every RuleError match ErrRule.
func (e *RuleError) Is(target error) bool { return target == ErrRule }

// wrapf annotates a sentinel error with a formatted message, keeping it
// matchable with errors.Is.
func wrapf(sentinel error, format string, args ...any) error {
	return fmt.Errorf("%s: %w", fmt.Sprintf(format, args...), sentinel)
}
