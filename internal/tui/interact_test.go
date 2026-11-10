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

// newLiveModel runs the TUI on a real service and database with three
// tasks in To do; KB-1 has a pending test.
func newLiveModel(t *testing.T) (*Model, *core.Service) {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "kb.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	svc := &core.Service{Store: db, Now: func() time.Time { return testNow }, Prefix: "KB", ClaimTTL: 15 * time.Minute}
	ctx := context.Background()
	for _, it := range []core.Item{
		{Type: core.TypeTask, Title: "Alpha", Status: core.StatusTodo, Tests: []core.Test{{Name: "a"}}},
		{Type: core.TypeTask, Title: "Beta", Status: core.StatusTodo},
		{Type: core.TypeTask, Title: "Gamma", Status: core.StatusTodo},
	} {
		if _, err := svc.Create(ctx, "@dev", it); err != nil {
			t.Fatal(err)
		}
	}
	m := New(ctx, NewBackend(svc, db), Options{Project: "demo", Actor: "@dev", Now: func() time.Time { return testNow }, Loc: time.UTC})
	m.tickEvery = 0
	send(t, m, windowSize(160, 48))
	runCmd(t, m, m.Init(), 0)
	return m, svc
}

func status(t *testing.T, svc *core.Service, id string) core.Status {
	t.Helper()
	it, err := svc.Get(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	return it.Status
}

func columnOrder(t *testing.T, svc *core.Service, st core.Status) string {
	t.Helper()
	snap, err := svc.Board(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, it := range core.Column(snap, st) {
		ids = append(ids, it.ID)
	}
	return strings.Join(ids, " ")
}

func TestShiftRightMoves(t *testing.T) {
	m, svc := newLiveModel(t)
	if m.selectedID() != "KB-1" {
		t.Fatalf("selected %s, want KB-1", m.selectedID())
	}
	press(t, m, "shift+right")
	if got := status(t, svc, "KB-1"); got != core.StatusDoing {
		t.Fatalf("KB-1 is %s, want doing", got)
	}
	if m.selectedID() != "KB-1" || m.board.col != int(core.StatusDoing) {
		t.Fatalf("selection did not follow the card: %s in column %d", m.selectedID(), m.board.col)
	}
	press(t, m, "shift+left", "shift+left")
	if got := status(t, svc, "KB-1"); got != core.StatusBacklog {
		t.Fatalf("KB-1 is %s, want backlog after two moves left", got)
	}
}

func TestMoveRefusedShowsAlert(t *testing.T) {
	m, svc := newLiveModel(t)
	press(t, m, "shift+right", "shift+right") // doing → review needs the test to pass
	if got := status(t, svc, "KB-1"); got != core.StatusDoing {
		t.Fatalf("KB-1 is %s, want it to stay in doing", got)
	}
	s := screen(m)
	for _, want := range []string{"MOVE REFUSED", "KB-1 Alpha can't move to Review", `test 1 "a" is pending`, "stays in In progress"} {
		if !strings.Contains(s, want) {
			t.Errorf("alert lacks %q", want)
		}
	}
	if !strings.Contains(s, "Gamma") {
		t.Error("the board must stay visible around the alert")
	}
	golden(t, "alert_160x48", s)
}

func TestAlertDismissWithEnter(t *testing.T) {
	m, _ := newLiveModel(t)
	press(t, m, "shift+right", "shift+right")
	press(t, m, "down") // ignored while the alert is open
	if m.alert == nil || m.selectedID() != "KB-1" {
		t.Fatal("keys other than enter/esc must not reach the board while the alert is open")
	}
	press(t, m, "enter")
	if m.alert != nil || strings.Contains(screen(m), "MOVE REFUSED") {
		t.Fatal("enter did not close the alert")
	}
}

func TestReorder(t *testing.T) {
	m, svc := newLiveModel(t)
	press(t, m, "down", "down") // KB-3
	press(t, m, "shift+up")
	if got := columnOrder(t, svc, core.StatusTodo); got != "KB-1 KB-3 KB-2" {
		t.Fatalf("order after shift+up = %s", got)
	}
	if m.selectedID() != "KB-3" {
		t.Fatalf("selection = %s, want it to follow KB-3", m.selectedID())
	}
	press(t, m, "shift+up", "shift+up") // already on top: no-op
	if got := columnOrder(t, svc, core.StatusTodo); got != "KB-3 KB-1 KB-2" {
		t.Fatalf("order = %s", got)
	}
	press(t, m, "shift+down")
	if got := columnOrder(t, svc, core.StatusTodo); got != "KB-1 KB-3 KB-2" {
		t.Fatalf("order after shift+down = %s", got)
	}
}

// A manual reorder while a sort is active starts from the order shown:
// the column is fixed as seen, then the card moves, and the board shows
// the manual order from then on.
func TestReorderAfterSortKeepsShownOrder(t *testing.T) {
	m, svc := newLiveModel(t)
	svc.Now = func() time.Time { return testNow.Add(time.Minute) }
	title := "Beta, renamed"
	if _, err := svc.Edit(context.Background(), "agent-x", "KB-2", core.Patch{Title: &title}, 0); err != nil {
		t.Fatal(err)
	}
	reload(t, m)
	if got := boardColumn(m, core.StatusTodo); got != "KB-2 KB-1 KB-3" {
		t.Fatalf("newest first = %s", got)
	}
	press(t, m, "down", "down", "shift+up") // KB-3 above KB-1, as shown
	if got := columnOrder(t, svc, core.StatusTodo); got != "KB-2 KB-3 KB-1" {
		t.Fatalf("stored order = %s, want the shown order with KB-3 moved up", got)
	}
	if got := boardColumn(m, core.StatusTodo); got != "KB-2 KB-3 KB-1" || !strings.Contains(m.header(), "manual order") {
		t.Fatalf("board = %s, header %s: want the manual order", got, m.header())
	}
	if m.selectedID() != "KB-3" {
		t.Fatalf("selection = %s, want it to follow KB-3", m.selectedID())
	}
}

// The next sort chosen from the popup overrides the manual order.
func TestSortOverridesManualOrder(t *testing.T) {
	m, svc := newLiveModel(t)
	press(t, m, "down", "down", "shift+up", "shift+up") // KB-3 KB-1 KB-2
	if got := boardColumn(m, core.StatusTodo); got != "KB-3 KB-1 KB-2" {
		t.Fatalf("manual = %s", got)
	}
	press(t, m, "s", "down", "space", "enter") // id ascending
	if got := boardColumn(m, core.StatusTodo); got != "KB-1 KB-2 KB-3" {
		t.Fatalf("after sorting by id = %s", got)
	}
	press(t, m, "up", "shift+down") // the selection stays on KB-3: select KB-2 and move it below KB-3
	if got := columnOrder(t, svc, core.StatusTodo); got != "KB-1 KB-3 KB-2" {
		t.Fatalf("stored order = %s: the old manual order must be overwritten", got)
	}
}

// A card that changes jumps to the top in newest-first order.
func TestChangedCardJumpsToTop(t *testing.T) {
	m, svc := newLiveModel(t)
	svc.Now = func() time.Time { return testNow.Add(time.Minute) }
	title := "Gamma, renamed"
	if _, err := svc.Edit(context.Background(), "agent-x", "KB-3", core.Patch{Title: &title}, 0); err != nil {
		t.Fatal(err)
	}
	reload(t, m)
	if got := boardColumn(m, core.StatusTodo); got != "KB-3 KB-1 KB-2" {
		t.Fatalf("after editing KB-3 the column is %s, want KB-3 on top", got)
	}
}
