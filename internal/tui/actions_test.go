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
