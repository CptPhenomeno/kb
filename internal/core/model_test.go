package core

import (
	"slices"
	"testing"
)

func TestPassingTests(t *testing.T) {
	it := Item{Tests: []Test{
		{Name: "a", State: TestPassing},
		{Name: "b", State: TestFailing},
		{Name: "c", State: TestPassing},
		{Name: "d", State: TestPending},
	}}
	if got := it.PassingTests(); got != 2 {
		t.Errorf("PassingTests() = %d, want 2", got)
	}
	if it.AllTestsPassing() {
		t.Error("AllTestsPassing() = true with failing and pending tests")
	}
	if !(&Item{}).AllTestsPassing() {
		t.Error("AllTestsPassing() must be true for an item without tests")
	}
}

func TestCheckedCriteria(t *testing.T) {
	it := Item{Acceptance: []Criterion{{Text: "x", Done: true}, {Text: "y"}}}
	if got := it.CheckedCriteria(); got != 1 {
		t.Errorf("CheckedCriteria() = %d, want 1", got)
	}
	if it.AllCriteriaChecked() {
		t.Error("AllCriteriaChecked() = true with an unchecked criterion")
	}
	if !(&Item{}).AllCriteriaChecked() {
		t.Error("AllCriteriaChecked() must be true for an item without criteria")
	}
}

func TestEventKindsAreTheSpecOnes(t *testing.T) {
	want := []string{"created", "updated", "moved", "claimed", "released", "claim_expired",
		"linked", "unlinked", "phase", "test", "acceptance", "note", "deleted"}
	var got []string
	for _, k := range EventKinds() {
		got = append(got, string(k))
	}
	if !slices.Equal(got, want) {
		t.Errorf("EventKinds() = %v, want %v", got, want)
	}
}

func TestIsHumanActor(t *testing.T) {
	cases := map[string]bool{"@dev": true, "@gabriele": true, "agent-auth": false, "kb": false, "": false}
	for actor, want := range cases {
		if got := IsHumanActor(actor); got != want {
			t.Errorf("IsHumanActor(%q) = %v, want %v", actor, got, want)
		}
	}
}
