package core

import (
	"strings"
	"time"
)

// Item is a unit of work on the board: an epic, story, task, subtask or bug.
type Item struct {
	ID          string
	Ref         string
	Type        Type
	Title       string
	Description string
	Parent      string // ID of the parent item, or "" for a root item.
	Status      Status
	Priority    Priority
	Points      int // Story points; 0 means unset.
	Spec        string
	Labels      []string
	Links       []string
	Acceptance  []Criterion
	Tests       []Test
	Phase       TDDPhase
	Assignee    string
	Claim       *Claim
	Position    int // Order inside the item's column, ascending.
	Version     int
	CreatedAt   time.Time
	CreatedBy   string
	UpdatedAt   time.Time
	UpdatedBy   string
}

// Criterion is an acceptance criterion of an item.
type Criterion struct {
	Text string
	Done bool
}

// Test is a test the item must pass, tracked for the TDD cycle.
type Test struct {
	Name  string
	State TestState
}

// Claim records that an actor has taken an item, until ExpiresAt.
type Claim struct {
	Actor     string
	ExpiresAt time.Time
}

// Relation is a directed link between two items. For RelRelates the
// direction carries no meaning.
type Relation struct {
	Kind RelationKind
	From string
	To   string
}

// PassingTests returns how many tests of the item are passing.
func (it *Item) PassingTests() int {
	n := 0
	for _, t := range it.Tests {
		if t.State == TestPassing {
			n++
		}
	}
	return n
}

// AllTestsPassing reports whether every test passes. It is true when the
// item has no tests.
func (it *Item) AllTestsPassing() bool { return it.PassingTests() == len(it.Tests) }

// CheckedCriteria returns how many acceptance criteria are checked.
func (it *Item) CheckedCriteria() int {
	n := 0
	for _, c := range it.Acceptance {
		if c.Done {
			n++
		}
	}
	return n
}

// AllCriteriaChecked reports whether every acceptance criterion is checked.
// It is true when the item has no criteria.
func (it *Item) AllCriteriaChecked() bool { return it.CheckedCriteria() == len(it.Acceptance) }

// EventKind is the kind of an entry in the event log.
type EventKind string

// Event kinds, as listed in the specification (§3.5).
const (
	EventCreated      EventKind = "created"
	EventUpdated      EventKind = "updated"
	EventMoved        EventKind = "moved"
	EventClaimed      EventKind = "claimed"
	EventReleased     EventKind = "released"
	EventClaimExpired EventKind = "claim_expired"
	EventLinked       EventKind = "linked"
	EventUnlinked     EventKind = "unlinked"
	EventPhase        EventKind = "phase"
	EventTest         EventKind = "test"
	EventAcceptance   EventKind = "acceptance"
	EventNote         EventKind = "note"
	EventDeleted      EventKind = "deleted"
)

// EventKinds returns every event kind in specification order.
func EventKinds() []EventKind {
	return []EventKind{EventCreated, EventUpdated, EventMoved, EventClaimed, EventReleased,
		EventClaimExpired, EventLinked, EventUnlinked, EventPhase, EventTest, EventAcceptance,
		EventNote, EventDeleted}
}

// Event is an append-only record of a change to the board.
type Event struct {
	ID    int64
	At    time.Time
	Actor string
	Item  string
	Kind  EventKind
	Data  map[string]any
}

// IsHumanActor reports whether an actor name denotes a person. By
// convention people are written as "@name"; every other name is an agent.
func IsHumanActor(actor string) bool { return strings.HasPrefix(actor, "@") }
