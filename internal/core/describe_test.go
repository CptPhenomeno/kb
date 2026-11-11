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
