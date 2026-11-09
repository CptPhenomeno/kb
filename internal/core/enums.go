package core

import (
	"errors"
	"fmt"
)

// ErrInvalidValue is returned when a string does not name a known enum value.
var ErrInvalidValue = errors.New("invalid value")

// enumNames maps every value of an enum to its canonical lowercase name.
type enumNames[T comparable] []struct {
	value T
	name  string
}

func (n enumNames[T]) name(v T) string {
	for _, e := range n {
		if e.value == v {
			return e.name
		}
	}
	return fmt.Sprintf("unknown(%v)", any(v))
}

func (n enumNames[T]) parse(kind, s string) (T, error) {
	for _, e := range n {
		if e.name == s {
			return e.value, nil
		}
	}
	var zero T
	return zero, fmt.Errorf("%w: unknown %s %q", ErrInvalidValue, kind, s)
}

// Type is the kind of a board item.
type Type int

// Item types. The hierarchy between them is defined in hierarchy.go.
const (
	TypeEpic Type = iota
	TypeStory
	TypeTask
	TypeSubtask
	TypeBug
)

var typeNames = enumNames[Type]{
	{TypeEpic, "epic"},
	{TypeStory, "story"},
	{TypeTask, "task"},
	{TypeSubtask, "subtask"},
	{TypeBug, "bug"},
}

// Types returns every item type in declaration order.
func Types() []Type { return []Type{TypeEpic, TypeStory, TypeTask, TypeSubtask, TypeBug} }

// ParseType parses the canonical name of a Type.
func ParseType(s string) (Type, error) { return typeNames.parse("type", s) }

func (t Type) String() string                { return typeNames.name(t) }
func (t Type) MarshalText() ([]byte, error)  { return []byte(t.String()), nil }
func (t *Type) UnmarshalText(b []byte) error { v, err := ParseType(string(b)); *t = v; return err }

// IsParentType reports whether items of this type are containers whose
// status follows their children (epics and stories).
func (t Type) IsParentType() bool { return t == TypeEpic || t == TypeStory }

// IsLeafType reports whether items of this type carry tests and a TDD phase.
func (t Type) IsLeafType() bool { return t == TypeTask || t == TypeSubtask || t == TypeBug }

// IsClaimable reports whether items of this type can be claimed by an actor.
func (t Type) IsClaimable() bool { return t == TypeTask || t == TypeBug }

// Status is the board column an item is in. Statuses are ordered from
// backlog to done; the order drives requirements and propagation.
type Status int

// Board columns, in board order.
const (
	StatusBacklog Status = iota
	StatusTodo
	StatusDoing
	StatusReview
	StatusDone
)

var statusNames = enumNames[Status]{
	{StatusBacklog, "backlog"},
	{StatusTodo, "todo"},
	{StatusDoing, "doing"},
	{StatusReview, "review"},
	{StatusDone, "done"},
}

var statusLabels = map[Status]string{
	StatusBacklog: "Backlog",
	StatusTodo:    "To do",
	StatusDoing:   "In progress",
	StatusReview:  "Review",
	StatusDone:    "Done",
}

// Statuses returns every status in board order.
func Statuses() []Status {
	return []Status{StatusBacklog, StatusTodo, StatusDoing, StatusReview, StatusDone}
}

// ParseStatus parses the canonical name of a Status.
func ParseStatus(s string) (Status, error) { return statusNames.parse("status", s) }

func (s Status) String() string                { return statusNames.name(s) }
func (s Status) MarshalText() ([]byte, error)  { return []byte(s.String()), nil }
func (s *Status) UnmarshalText(b []byte) error { v, err := ParseStatus(string(b)); *s = v; return err }

// Label is the column title shown in the TUI.
func (s Status) Label() string { return statusLabels[s] }

// Before reports whether s comes earlier than other on the board.
func (s Status) Before(other Status) bool { return s < other }

// Priority orders work items. The zero value is PriorityMedium, the default.
type Priority int

// Priorities, from lowest to highest.
const (
	PriorityLow Priority = iota - 1
	PriorityMedium
	PriorityHigh
)

var priorityNames = enumNames[Priority]{
	{PriorityLow, "low"},
	{PriorityMedium, "medium"},
	{PriorityHigh, "high"},
}

// ParsePriority parses the canonical name of a Priority.
func ParsePriority(s string) (Priority, error) { return priorityNames.parse("priority", s) }

func (p Priority) String() string               { return priorityNames.name(p) }
func (p Priority) MarshalText() ([]byte, error) { return []byte(p.String()), nil }
func (p *Priority) UnmarshalText(b []byte) error {
	v, err := ParsePriority(string(b))
	*p = v
	return err
}

// TDDPhase is the step of the red-green-refactor cycle a leaf item is in.
// The zero value, PhaseNone, means no phase has been set.
type TDDPhase int

// TDD phases.
const (
	PhaseNone TDDPhase = iota
	PhaseRed
	PhaseGreen
	PhaseRefactor
)

var phaseNames = enumNames[TDDPhase]{
	{PhaseNone, ""},
	{PhaseRed, "red"},
	{PhaseGreen, "green"},
	{PhaseRefactor, "refactor"},
}

// ParsePhase parses the canonical name of a TDDPhase; "" is PhaseNone.
func ParsePhase(s string) (TDDPhase, error) { return phaseNames.parse("tdd phase", s) }

func (p TDDPhase) String() string                { return phaseNames.name(p) }
func (p TDDPhase) MarshalText() ([]byte, error)  { return []byte(p.String()), nil }
func (p *TDDPhase) UnmarshalText(b []byte) error { v, err := ParsePhase(string(b)); *p = v; return err }

// TestState is the outcome of a test attached to an item.
type TestState int

// Test states. New tests start as pending.
const (
	TestPending TestState = iota
	TestFailing
	TestPassing
)

var testStateNames = enumNames[TestState]{
	{TestPending, "pending"},
	{TestFailing, "failing"},
	{TestPassing, "passing"},
}

// ParseTestState parses the canonical name of a TestState.
func ParseTestState(s string) (TestState, error) { return testStateNames.parse("test state", s) }

func (s TestState) String() string               { return testStateNames.name(s) }
func (s TestState) MarshalText() ([]byte, error) { return []byte(s.String()), nil }
func (s *TestState) UnmarshalText(b []byte) error {
	v, err := ParseTestState(string(b))
	*s = v
	return err
}

// RelationKind is the kind of a link between two items, besides parent/child.
type RelationKind int

// Relation kinds.
const (
	RelBlocks RelationKind = iota
	RelRelates
	RelDuplicates
)

var relationNames = enumNames[RelationKind]{
	{RelBlocks, "blocks"},
	{RelRelates, "relates"},
	{RelDuplicates, "duplicates"},
}

// ParseRelationKind parses the canonical name of a RelationKind.
func ParseRelationKind(s string) (RelationKind, error) { return relationNames.parse("relation", s) }

func (k RelationKind) String() string               { return relationNames.name(k) }
func (k RelationKind) MarshalText() ([]byte, error) { return []byte(k.String()), nil }
func (k *RelationKind) UnmarshalText(b []byte) error {
	v, err := ParseRelationKind(string(b))
	*k = v
	return err
}
