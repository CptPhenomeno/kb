package core

import (
	"fmt"
	"strings"
)

// CheckRequirements returns every entry requirement of target that item
// does not meet (SPEC.md §4.2). An empty result means the move is allowed.
// The messages are part of the public contract (they reach agents through
// --json) and must stay stable.
func CheckRequirements(s *Snapshot, item *Item, target Status) []string {
	var unmet []string
	children := s.Children(item.ID)

	switch target {
	case StatusBacklog, StatusTodo:
		for _, c := range children {
			if target.Before(c.Status) {
				unmet = append(unmet, childMsg(c))
			}
		}
	case StatusDoing:
		if blocked, by := Blockers(s, item.ID); blocked {
			unmet = append(unmet, "blocked by "+strings.Join(by, ", "))
		}
	case StatusReview:
		for _, c := range children {
			if c.Status.Before(StatusReview) {
				unmet = append(unmet, childMsg(c))
			}
		}
		unmet = append(unmet, testMsgs(item)...)
	case StatusDone:
		for _, c := range children {
			if c.Status != StatusDone {
				unmet = append(unmet, childMsg(c))
			}
		}
		unmet = append(unmet, testMsgs(item)...)
		if !item.AllCriteriaChecked() {
			unmet = append(unmet, fmt.Sprintf("acceptance %d/%d checked", item.CheckedCriteria(), len(item.Acceptance)))
		}
	}
	return unmet
}

func childMsg(c *Item) string { return fmt.Sprintf("child %s is %s", c.ID, c.Status) }

func testMsgs(item *Item) []string {
	var out []string
	for i, t := range item.Tests {
		if t.State != TestPassing {
			out = append(out, fmt.Sprintf("test %d %q is %s", i+1, t.Name, t.State))
		}
	}
	return out
}
