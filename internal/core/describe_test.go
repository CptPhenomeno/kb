package core_test

import (
	"testing"

	"github.com/CptPhenomeno/kb/internal/core"
)

func TestDescribeCriterionEditedShowsOldAndNewText(t *testing.T) {
	ev := core.Event{Kind: core.EventAcceptance, Data: map[string]any{
		"text": "new text", "done": false, "edited": true, "old": "old text",
	}}
	want := `criterion edited: "old text" → "new text"`
	if got := core.DescribeEvent(ev); got != want {
		t.Fatalf("DescribeEvent = %q, want %q", got, want)
	}
}

func TestDescribeCriterionRemovedShowsItsText(t *testing.T) {
	ev := core.Event{Kind: core.EventAcceptance, Data: map[string]any{
		"text": "gone", "done": true, "removed": true,
	}}
	want := `criterion removed: "gone"`
	if got := core.DescribeEvent(ev); got != want {
		t.Fatalf("DescribeEvent = %q, want %q", got, want)
	}
}

func TestDescribeTestEditedIsARename(t *testing.T) {
	ev := core.Event{Kind: core.EventTest, Data: map[string]any{
		"name": "bee", "state": "pending", "edited": true, "old": "b",
	}}
	want := `test renamed: "b" → "bee"`
	if got := core.DescribeEvent(ev); got != want {
		t.Fatalf("DescribeEvent = %q, want %q", got, want)
	}
}

func TestDescribeTestRemovedIsARemoval(t *testing.T) {
	ev := core.Event{Kind: core.EventTest, Data: map[string]any{
		"name": "a", "state": "passing", "removed": true,
	}}
	want := `test removed: "a"`
	if got := core.DescribeEvent(ev); got != want {
		t.Fatalf("DescribeEvent = %q, want %q", got, want)
	}
}
