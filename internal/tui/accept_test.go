package tui

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/CptPhenomeno/kb/internal/core"
	"github.com/CptPhenomeno/kb/internal/store"
)

// newStoryModel runs the TUI on a real board holding a story with two
// unchecked criteria and a done task: the story waits in review.
func newStoryModel(t *testing.T) (*Model, *core.Service) {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "kb.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	svc := &core.Service{Store: db, Now: func() time.Time { return testNow }, Prefix: "KB", ClaimTTL: 15 * time.Minute}
	ctx := context.Background()
	story, err := svc.Create(ctx, "@dev", core.Item{Type: core.TypeStory, Title: "Login",
		Acceptance: []core.Criterion{{Text: "opens the board"}, {Text: "shows writes within a second"}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Create(ctx, "@dev", core.Item{Type: core.TypeTask, Title: "Done task", Parent: story.ID, Status: core.StatusDone}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Create(ctx, "@dev", core.Item{Type: core.TypeTask, Title: "Other", Status: core.StatusTodo}); err != nil {
		t.Fatal(err)
	}
	m := New(ctx, NewBackend(svc, db), Options{Project: "demo", Actor: "@dev", Now: func() time.Time { return testNow }, Loc: time.UTC})
	m.tickEvery = 0
	send(t, m, windowSize(160, 48))
	runCmd(t, m, m.Init(), 0)
	return m, svc
}

func selectID(t *testing.T, m *Model, id string) {
	t.Helper()
	cols := m.board.columns(m)
	for c := range cols {
		for r, it := range cols[c] {
			if it.ID == id {
				m.board.col, m.board.row = c, r
				return
			}
		}
	}
	t.Fatalf("%s not on the board", id)
}

func TestCOpensAcceptancePopup(t *testing.T) {
	m, _ := newStoryModel(t)
	selectID(t, m, "KB-1")
	press(t, m, "c")
	s := screen(m)
	for _, want := range []string{"Acceptance", "KB-1 Login", "[ ] opens the board", "[ ] shows writes within a second"} {
		if !strings.Contains(s, want) {
			t.Errorf("popup lacks %q", want)
		}
	}
}

func TestSpaceChecksACriterionAndSavesIt(t *testing.T) {
	m, svc := newStoryModel(t)
	selectID(t, m, "KB-1")
	press(t, m, "c", "space")
	it, _ := svc.Get(context.Background(), "KB-1")
	if !it.Acceptance[0].Done || it.Acceptance[1].Done {
		t.Fatalf("criteria = %+v, want only the first checked", it.Acceptance)
	}
	if !strings.Contains(screen(m), "[x] opens the board") {
		t.Fatal("the popup does not show the saved check")
	}
	press(t, m, "space")
	if it, _ := svc.Get(context.Background(), "KB-1"); it.Acceptance[0].Done {
		t.Fatal("space again must uncheck the criterion")
	}
}

func TestCheckingTheLastCriterionTakesTheStoryToDone(t *testing.T) {
	m, svc := newStoryModel(t)
	selectID(t, m, "KB-1")
	if got := status(t, svc, "KB-1"); got != core.StatusReview {
		t.Fatalf("fixture story is %s, want review", got)
	}
	press(t, m, "c", "space", "down", "space", "esc")
	if got := status(t, svc, "KB-1"); got != core.StatusDone {
		t.Fatalf("story is %s after checking every criterion, want done", got)
	}
	if m.selectedID() != "KB-1" {
		t.Fatalf("selection = %s, want it to follow the story to Done", m.selectedID())
	}
}

func TestItemWithoutCriteriaExplainsThereIsNone(t *testing.T) {
	m, _ := newStoryModel(t)
	selectID(t, m, "KB-3")
	press(t, m, "c")
	if s := screen(m); !strings.Contains(s, "no acceptance criteria") {
		t.Fatalf("popup for an item without criteria:\n%s", s)
	}
}
