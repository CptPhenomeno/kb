package tui

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/CptPhenomeno/kb/internal/core"
)

func exists(t *testing.T, svc *core.Service, id string) bool {
	t.Helper()
	_, err := svc.Get(context.Background(), id)
	if err != nil && !errors.Is(err, core.ErrNotFound) {
		t.Fatal(err)
	}
	return err == nil
}

// reload makes the model read the board again, as the live refresh does.
func reload(t *testing.T, m *Model) { t.Helper(); send(t, m, tickMsg{}) }

func TestDeleteRequiresConfirm(t *testing.T) {
	m, svc := newLiveModel(t)
	selectID(t, m, "KB-2")
	press(t, m, "d")
	if !strings.Contains(screen(m), "Delete KB-2 Beta?") {
		t.Fatalf("d did not ask for confirmation:\n%s", screen(m))
	}
	press(t, m, "esc")
	if !exists(t, svc, "KB-2") {
		t.Fatal("esc deleted the item")
	}
	press(t, m, "d", "enter")
	if exists(t, svc, "KB-2") {
		t.Fatal("enter did not delete the item")
	}
}

func TestDeleteWithChildrenAsksRecursive(t *testing.T) {
	m, svc := newStoryModel(t)
	selectID(t, m, "KB-1")
	press(t, m, "d")
	if s := screen(m); !strings.Contains(s, "1 descendant") || !strings.Contains(s, "KB-2") {
		t.Fatalf("confirmation does not warn about the children:\n%s", s)
	}
	press(t, m, "enter")
	if exists(t, svc, "KB-1") || exists(t, svc, "KB-2") {
		t.Fatal("the story and its child must both be deleted")
	}
}

func TestAssignTogglesClaim(t *testing.T) {
	m, svc := newLiveModel(t)
	press(t, m, "a") // KB-1
	it, _ := svc.Get(context.Background(), "KB-1")
	if it.Claim == nil || it.Claim.Actor != "@dev" || it.Status != core.StatusDoing {
		t.Fatalf("a did not claim KB-1 for @dev: %+v", it)
	}
	press(t, m, "a")
	it, _ = svc.Get(context.Background(), "KB-1")
	if it.Claim != nil || it.Status != core.StatusTodo {
		t.Fatalf("a again did not release KB-1: claim %+v, status %s", it.Claim, it.Status)
	}
}

func TestClaimRefusedShowsAlert(t *testing.T) {
	m, svc := newLiveModel(t)
	if _, err := svc.Claim(context.Background(), "agent-x", "KB-2", 0); err != nil {
		t.Fatal(err)
	}
	reload(t, m)
	selectID(t, m, "KB-2")
	press(t, m, "a")
	s := screen(m)
	if !strings.Contains(s, "CLAIM REFUSED") || !strings.Contains(s, "agent-x") {
		t.Fatalf("claim of an item held by another agent:\n%s", s)
	}
}

func relations(t *testing.T, svc *core.Service) []core.Relation {
	t.Helper()
	snap, err := svc.Board(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return snap.Relations
}

func TestLinkAddRemove(t *testing.T) {
	m, svc := newLiveModel(t)
	press(t, m, "l") // KB-1
	if !strings.Contains(screen(m), "Links") {
		t.Fatal("l did not open the links popup")
	}
	press(t, m, "a", "K", "B", "-", "2", "enter")
	rels := relations(t, svc)
	if len(rels) != 1 || rels[0] != (core.Relation{Kind: core.RelBlocks, From: "KB-1", To: "KB-2"}) {
		t.Fatalf("relations = %v, want KB-1 blocks KB-2", rels)
	}
	if !strings.Contains(screen(m), "blocks") || !strings.Contains(screen(m), "KB-2 Beta") {
		t.Fatalf("the popup does not list the new link:\n%s", screen(m))
	}
	press(t, m, "a", "right", "right", "K", "B", "-", "3", "enter") // relates
	if len(relations(t, svc)) != 2 {
		t.Fatalf("relations = %v, want two", relations(t, svc))
	}
	press(t, m, "x") // remove the selected (first) link
	if rels := relations(t, svc); len(rels) != 1 || rels[0].Kind != core.RelRelates {
		t.Fatalf("relations after x = %v, want only relates", rels)
	}
}

func TestLinkCycleShowsAlert(t *testing.T) {
	m, svc := newLiveModel(t)
	if err := svc.Link(context.Background(), "@dev", core.Relation{Kind: core.RelBlocks, From: "KB-1", To: "KB-2"}); err != nil {
		t.Fatal(err)
	}
	reload(t, m)
	selectID(t, m, "KB-2")
	press(t, m, "l", "a", "K", "B", "-", "1", "enter")
	if s := screen(m); !strings.Contains(s, "LINK REFUSED") || !strings.Contains(s, "cycle") {
		t.Fatalf("a cyclic link must be refused with an alert:\n%s", s)
	}
}

func TestLinksShowRelationTitleAndStatus(t *testing.T) {
	m, svc := newLiveModel(t)
	if err := svc.Link(context.Background(), "@dev", core.Relation{Kind: core.RelBlocks, From: "KB-3", To: "KB-1"}); err != nil {
		t.Fatal(err)
	}
	reload(t, m)
	selectID(t, m, "KB-1")
	press(t, m, "l")
	var row string
	for _, line := range strings.Split(screen(m), "\n") {
		if strings.Contains(line, "blocked by") {
			row = line
		}
	}
	for _, want := range []string{"KB-3", "Gamma", "To do"} {
		if !strings.Contains(row, want) {
			t.Errorf("the link row lacks %q: %q", want, row)
		}
	}
}

func TestEnterGoesToTheLinkedItem(t *testing.T) {
	m, svc := newLiveModel(t)
	if err := svc.Link(context.Background(), "@dev", core.Relation{Kind: core.RelRelates, From: "KB-1", To: "KB-3"}); err != nil {
		t.Fatal(err)
	}
	reload(t, m)
	selectID(t, m, "KB-1")
	press(t, m, "l", "enter")
	if m.links != nil || m.selectedID() != "KB-3" {
		t.Fatalf("enter: popup %v, selected %s, want KB-3", m.links, m.selectedID())
	}
}

func TestALinkedItemHiddenByFiltersIsShown(t *testing.T) {
	m, svc := newStoryModel(t) // KB-1 story, KB-2 done task under it, KB-3 task
	ctx := context.Background()
	sub, err := svc.Create(ctx, "@dev", core.Item{Type: core.TypeSubtask, Title: "Hidden subtask", Parent: "KB-3"})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Link(ctx, "@dev", core.Relation{Kind: core.RelRelates, From: "KB-3", To: sub.ID}); err != nil {
		t.Fatal(err)
	}
	reload(t, m)
	press(t, m, "/", "O", "t", "h", "enter") // search shows only KB-3 "Other"
	selectID(t, m, "KB-3")
	press(t, m, "l", "enter")
	if m.selectedID() != sub.ID {
		t.Fatalf("selected %s, want the subtask %s", m.selectedID(), sub.ID)
	}
	if m.board.query != "" || !m.board.types[core.TypeSubtask] {
		t.Fatalf("filters still hide the item: query %q, subtasks %v", m.board.query, m.board.types[core.TypeSubtask])
	}
}

func TestLinksPopupListsParentAndChildren(t *testing.T) {
	m, _ := newStoryModel(t) // KB-1 story with the child KB-2
	selectID(t, m, "KB-1")
	press(t, m, "l")
	var row string
	for _, line := range strings.Split(screen(m), "\n") {
		if strings.Contains(line, "child") && strings.Contains(line, "KB-2") {
			row = line
		}
	}
	if !strings.Contains(row, "Done task") || !strings.Contains(row, "Done") {
		t.Fatalf("children missing from the links popup: %q\n%s", row, screen(m))
	}
	press(t, m, "x") // the hierarchy is not a removable link
	if m.alert != nil {
		t.Fatal("x on a hierarchy row must not try to unlink")
	}
	press(t, m, "enter")
	if m.selectedID() != "KB-2" {
		t.Fatalf("enter on the child selected %s, want KB-2", m.selectedID())
	}
	selectID(t, m, "KB-2")
	press(t, m, "l", "enter")
	if m.selectedID() != "KB-1" {
		t.Fatalf("enter on the parent row selected %s, want KB-1", m.selectedID())
	}
}
