package tui

import (
	"context"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/CptPhenomeno/kb/internal/core"
)

// activityBoard is the sample board with writes by a person and by the
// stale agent, so every kind of actor shows.
func activityBoard() *fakeBackend {
	be := sampleBoard()
	at := func(m int) time.Time { return testNow.Add(time.Duration(-m) * time.Minute) }
	be.events = append([]core.Event{
		{ID: -3, At: at(40), Actor: "agent-metrics", Item: "KB-12", Kind: core.EventClaimed},
		{ID: -2, At: at(30), Actor: "agent-metrics", Item: "KB-12", Kind: core.EventPhase, Data: map[string]any{"from": "red", "to": "green"}},
		{ID: -1, At: at(24), Actor: "@dev", Item: "KB-17", Kind: core.EventMoved, Data: map[string]any{"from": "review", "to": "done"}},
	}, be.events...)
	return be
}

func activityModel(t *testing.T) *Model {
	t.Helper()
	m := newModel(t, activityBoard(), 160, 48)
	press(t, m, "shift+tab")
	if m.view != viewActivity {
		t.Fatalf("view = %v, want Activity", m.view)
	}
	return m
}

func actorNames(m *Model) []string {
	var out []string
	for _, a := range m.activity.actors(m) {
		out = append(out, a.name)
	}
	return out
}

func TestActorsDerivedFromEvents(t *testing.T) {
	m := activityModel(t)
	// Active claims first, then stale ones, then the rest by last write.
	if got := strings.Join(actorNames(m), " "); got != "agent-auth agent-metrics @dev" {
		t.Fatalf("actors = %s", got)
	}
	auth := m.activity.actors(m)[0]
	if auth.item == nil || auth.item.ID != "KB-4" {
		t.Fatalf("agent-auth works on %v, want KB-4 (the active claim, not the expired one on KB-3)", auth.item)
	}
	scr := screen(m)
	for _, want := range []*regexp.Regexp{
		regexp.MustCompile(`agent-auth .*active · claim 12m left`),
		regexp.MustCompile(`KB-4 Token refresh`),
		regexp.MustCompile(`● RED .*tests 0/2 .*last write just now`),
		regexp.MustCompile(`@dev .*human`),
		regexp.MustCompile(`no claimed item`),
	} {
		if !want.MatchString(scr) {
			t.Fatalf("activity misses %s:\n%s", want, scr)
		}
	}
}

func TestStaleActorHighlighted(t *testing.T) {
	m := activityModel(t)
	scr := screen(m)
	if !regexp.MustCompile(`agent-metrics .*stale · claim expired 22m ago`).MatchString(scr) {
		t.Fatalf("stale actor not flagged:\n%s", scr)
	}
	if !strings.Contains(scr, "silent for 30m") {
		t.Fatalf("stale actor does not say how long it has been silent:\n%s", scr)
	}
	if !strings.Contains(m.header(), "1 stale") {
		t.Fatal("header lacks the stale count")
	}
}

func TestFilterByActor(t *testing.T) {
	m := activityModel(t)
	events := func() string { return m.activity.renderEvents(m, 100, 40) }
	if !strings.Contains(events(), "@dev") || !strings.Contains(events(), "agent-auth") {
		t.Fatalf("all actors are not shown:\n%s", events())
	}
	press(t, m, "down", "enter")
	got := events()
	if !strings.Contains(got, "only agent-metrics") || strings.Contains(got, "agent-auth") || strings.Contains(got, "@dev") {
		t.Fatalf("enter did not keep only agent-metrics:\n%s", got)
	}
	if !strings.Contains(got, "phase red → green") && !strings.Contains(got, "green") {
		t.Fatalf("agent-metrics events missing:\n%s", got)
	}
	press(t, m, "esc")
	if got := events(); !strings.Contains(got, "all actors") || !strings.Contains(got, "agent-auth") {
		t.Fatalf("esc did not show every actor again:\n%s", got)
	}
}

func TestEventsNewestFirst(t *testing.T) {
	m := activityModel(t)
	got := m.activity.renderEvents(m, 100, 40)
	if strings.Index(got, "14:04") > strings.Index(got, "13:40") {
		t.Fatalf("events are not newest first:\n%s", got)
	}
}

func TestOpenItemFromActor(t *testing.T) {
	m := activityModel(t)
	press(t, m, "down", "o")
	if m.view != viewBoard || m.selectedID() != "KB-12" || !m.detailOpen {
		t.Fatalf("o: view=%v selected=%s detail=%v, want the Board on KB-12 with its detail", m.view, m.selectedID(), m.detailOpen)
	}
	m = activityModel(t)
	press(t, m, "down", "down", "o")
	if m.view != viewActivity || m.notice == "" {
		t.Fatalf("o on an actor without items should stay with a notice, got view=%v notice=%q", m.view, m.notice)
	}
}

func TestReleaseFromActivity(t *testing.T) {
	m, svc := newLiveModel(t)
	ctx := context.Background()
	if _, err := svc.Claim(ctx, "agent-x", "KB-2", 0); err != nil {
		t.Fatal(err)
	}
	reload(t, m)
	press(t, m, "shift+tab")
	if names := actorNames(m); len(names) == 0 || names[0] != "agent-x" {
		t.Fatalf("actors = %v, want agent-x first", names)
	}
	press(t, m, "a")
	it, err := svc.Get(ctx, "KB-2")
	if err != nil {
		t.Fatal(err)
	}
	if it.Claim != nil || it.Status != core.StatusTodo {
		t.Fatalf("after a: claim=%v status=%v, want released and back in To do", it.Claim, it.Status)
	}
	if !strings.Contains(m.notice, "released") {
		t.Fatalf("notice = %q", m.notice)
	}
}

func TestActivitySnapshot(t *testing.T) {
	m := activityModel(t)
	press(t, m, "down")
	golden(t, "activity", screen(m))
}

// A claim left on a finished item does not hide the item in progress.
func TestActorWorksOnItemInProgress(t *testing.T) {
	be := activityBoard()
	be.snap.Get("KB-17").Claim = &core.Claim{Actor: "agent-auth", ExpiresAt: testNow.Add(10 * time.Minute)}
	be.snap.Get("KB-3").Claim.ExpiresAt = testNow.Add(10 * time.Minute) // review, still claimed
	m := newModel(t, be, 160, 48)
	press(t, m, "shift+tab")
	if it := m.activity.actors(m)[0].item; it == nil || it.ID != "KB-4" {
		t.Fatalf("agent-auth works on %v, want KB-4 in progress", it)
	}
}
