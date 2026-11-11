package tui

import (
	"context"
	"strings"
	"testing"

	"github.com/CptPhenomeno/kb/internal/core"
)

func TestLiveRefreshUpdatesTheOpenDetail(t *testing.T) {
	m, svc := newLiveModel(t)
	openDetail(t, m, "KB-2")
	title, ctx := "Beta renamed by an agent", context.Background()
	if _, err := svc.Edit(ctx, "agent-x", "KB-2", core.Patch{Title: &title}, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Move(ctx, "agent-x", "KB-2", core.StatusDoing, 0); err != nil {
		t.Fatal(err)
	}
	reload(t, m)
	if !m.detailOpen || !strings.Contains(detailText(m), title) || !strings.Contains(detailText(m), "KB-2") {
		t.Fatalf("open %v, the detail does not show the write:\n%s", m.detailOpen, detailText(m))
	}
}

func TestLiveRefreshReloadsTheHistoryOfTheItemShown(t *testing.T) {
	m, svc := newLiveModel(t)
	openDetail(t, m, "KB-2")
	if _, err := svc.Claim(context.Background(), "agent-x", "KB-2", 0); err != nil {
		t.Fatal(err)
	}
	reload(t, m)
	if !strings.Contains(detailText(m), "agent-x") {
		t.Fatalf("the history does not show the new event:\n%s", detailText(m))
	}
}

func TestDeletingTheItemShownClosesTheDetailWithANotice(t *testing.T) {
	m, svc := newLiveModel(t)
	openDetail(t, m, "KB-2")
	if _, err := svc.Remove(context.Background(), "agent-x", "KB-2", false); err != nil {
		t.Fatal(err)
	}
	reload(t, m)
	if m.detailOpen || !strings.Contains(m.notice, "KB-2") || !strings.Contains(m.notice, "deleted") {
		t.Fatalf("open %v, notice %q: want the detail closed and 'KB-2 was deleted'", m.detailOpen, m.notice)
	}
	if !strings.Contains(screen(m), "KB-2 was deleted") {
		t.Fatalf("the status line does not show the notice:\n%s", screen(m))
	}
}

func TestOpeningAnotherItemStartsTheDetailFromTheTop(t *testing.T) {
	m := longDetail(t)
	press(t, m, "pgdown", "esc")
	if m.detailScroll == 0 {
		t.Fatal("fixture: the detail did not scroll")
	}
	press(t, m, "up", "enter")
	if m.selectedID() == "KB-4" || !m.detailOpen || m.detailScroll != 0 {
		t.Fatalf("selected %s, open %v, scroll %d: want another item from the top", m.selectedID(), m.detailOpen, m.detailScroll)
	}
}

func TestReopeningTheSameItemKeepsTheDetailScroll(t *testing.T) {
	m := longDetail(t)
	press(t, m, "pgdown", "esc")
	want := m.detailScroll
	if want == 0 {
		t.Fatal("fixture: the detail did not scroll")
	}
	press(t, m, "up", "down")
	if m.selectedID() != "KB-4" {
		t.Fatalf("selected %s, want KB-4", m.selectedID())
	}
	press(t, m, "enter")
	if m.detailScroll != want {
		t.Fatalf("scroll %d after reopening, want %d", m.detailScroll, want)
	}
}

func TestActivityOOpensTheBoardWithTheDetailPopup(t *testing.T) {
	m := activityModel(t)
	press(t, m, "down", "o")
	if m.view != viewBoard || !m.detailOpen || !strings.Contains(detailText(m), "Status code counters") {
		t.Fatalf("view %v, open %v:\n%s", m.view, m.detailOpen, detailText(m))
	}
}
