package tui

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/CptPhenomeno/kb/internal/core"
)

// counterBoard has points in every type: the In progress column holds a 3pt task,
// a 5pt bug, a 2pt subtask, a story and an epic with derived points, and a
// task without points.
func counterBoard() *fakeBackend {
	items := []*core.Item{
		item("KB-1", core.TypeEpic, core.StatusDoing, "", "Epic"),
		item("KB-2", core.TypeStory, core.StatusDoing, "KB-1", "Story"),
		item("KB-3", core.TypeTask, core.StatusDoing, "KB-2", "Three", points(3)),
		item("KB-4", core.TypeBug, core.StatusDoing, "", "Five", points(5)),
		item("KB-5", core.TypeTask, core.StatusDoing, "", "None"),
		item("KB-6", core.TypeSubtask, core.StatusDoing, "KB-3", "Sub", points(2)),
		item("KB-7", core.TypeTask, core.StatusTodo, "", "Eight", points(8)),
	}
	return &fakeBackend{snap: core.NewSnapshot(items, nil), version: 1}
}

func counterModel(t *testing.T, opts Options) *Model {
	t.Helper()
	opts.Project, opts.Actor, opts.Now, opts.Loc = "api-gateway", "@dev", func() time.Time { return testNow }, time.UTC
	m := New(context.Background(), counterBoard(), opts)
	m.tickEvery = 0
	send(t, m, windowSize(160, 48))
	runCmd(t, m, m.Init(), 0)
	return m
}

// laneHeader returns the title row of the column labelled label.
func laneHeader(t *testing.T, m *Model, label string) string {
	t.Helper()
	for _, l := range strings.Split(screen(m), "\n") {
		if strings.Contains(l, label) {
			return l
		}
	}
	t.Fatalf("no %s header:\n%s", label, screen(m))
	return ""
}

func TestHashKeyTogglesTheLaneCountersToPoints(t *testing.T) {
	var saved []string
	m := counterModel(t, Options{SaveBoardCounter: func(s string) error { saved = append(saved, s); return nil }})
	if h := laneHeader(t, m, "In progress"); strings.Contains(h, "pt") {
		t.Fatalf("cards mode shows points: %q", h)
	}
	press(t, m, "#")
	if h := laneHeader(t, m, "In progress"); !strings.Contains(h, "8pt") {
		t.Fatalf("points mode header: %q", h)
	}
	press(t, m, "#")
	if h := laneHeader(t, m, "In progress"); strings.Contains(h, "pt") {
		t.Fatalf("back in cards mode: %q", h)
	}
	if strings.Join(saved, ",") != "points,cards" {
		t.Fatalf("saved = %v", saved)
	}
}

func TestLanePointsSumOnlyVisibleTasksAndBugs(t *testing.T) {
	m := counterModel(t, Options{BoardCounter: "points"})
	// 3 (task) + 5 (bug); the subtask's 2 is hidden by default
	if h := laneHeader(t, m, "In progress"); !strings.Contains(h, "8pt") {
		t.Fatalf("In progress header: %q", h)
	}
	if h := laneHeader(t, m, "To do"); !strings.Contains(h, "8pt") {
		t.Fatalf("To do header: %q", h)
	}
}

func TestLanePointsIgnoreStoryAndEpicDerivedPoints(t *testing.T) {
	m := counterModel(t, Options{BoardCounter: "points"})
	cols := m.board.columns(m)
	doing := cols[core.StatusDoing]
	if got := visiblePoints(doing); got != 8 {
		t.Fatalf("visiblePoints = %d, want 8 (epic and story derived points left out)", got)
	}
	if got := visiblePoints([]*core.Item{m.snap.Get("KB-1"), m.snap.Get("KB-2")}); got != 0 {
		t.Fatalf("parents count %d, want 0", got)
	}
}

func TestLanePointsFollowTheActiveFilters(t *testing.T) {
	m := counterModel(t, Options{BoardCounter: "points"})
	m.board.types[core.TypeBug] = false
	if h := laneHeader(t, m, "In progress"); !strings.Contains(h, "3pt") {
		t.Fatalf("bugs hidden: %q", h)
	}
	m.board.types[core.TypeBug] = true
	m.board.query = "five"
	if h := laneHeader(t, m, "In progress"); !strings.Contains(h, "5pt") {
		t.Fatalf("search five: %q", h)
	}
	m.board.query = ""
	m.board.types[core.TypeSubtask] = true
	if h := laneHeader(t, m, "In progress"); !strings.Contains(h, "8pt") {
		t.Fatalf("subtasks shown must not count: %q", h)
	}
}
