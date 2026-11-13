package core

import (
	"fmt"
	"math/rand"
	"testing"
	"time"
)

// perfBoard generates a realistic board of about n items (epics > stories >
// tasks, some subtasks) with about n blocks relations: chains inside stories,
// forks, blocks between stories and between tasks of different stories. The
// result is deterministic and acyclic: every relation goes forward in
// creation order.
func perfBoard(n int) *Snapshot {
	rng := rand.New(rand.NewSource(1))
	statuses := []Status{StatusBacklog, StatusTodo, StatusTodo, StatusTodo, StatusDoing, StatusReview, StatusDone}
	points := []int{0, 1, 2, 3, 5, 8, 13}
	priorities := []Priority{PriorityLow, PriorityMedium, PriorityMedium, PriorityHigh}

	var all []*Item
	var tasks, stories []*Item
	var rels []Relation
	add := func(typ Type, parent string) *Item {
		x := it(fmt.Sprintf("KB-%d", len(all)+1), typ, statuses[rng.Intn(len(statuses))], parent)
		x.Points = points[rng.Intn(len(points))]
		x.Priority = priorities[rng.Intn(len(priorities))]
		x.Position = len(all)
		all = append(all, x)
		return x
	}

	var epic *Item
	for len(all) < n {
		if epic == nil || len(stories)%5 == 0 {
			epic = add(TypeEpic, "")
		}
		story := add(TypeStory, epic.ID)
		stories = append(stories, story)
		var own []*Item
		for i := 0; i < 5; i++ {
			t := add(TypeTask, story.ID)
			own = append(own, t)
			tasks = append(tasks, t)
			if i%2 == 0 {
				add(TypeSubtask, t.ID)
			}
		}
		for i := 0; i+1 < len(own); i++ { // chain inside the story
			rels = append(rels, blocks(own[i].ID, own[i+1].ID))
		}
		if len(stories) > 1 { // story blocks the next one
			rels = append(rels, blocks(stories[len(stories)-2].ID, story.ID))
		}
		rels = append(rels, blocks(own[0].ID, own[2].ID), blocks(own[0].ID, own[3].ID)) // fork
	}
	for len(rels) < n { // blocks between tasks of different stories
		i := rng.Intn(len(tasks) - 1)
		j := i + 1 + rng.Intn(min(len(tasks)-1-i, 60))
		if tasks[i].Parent != tasks[j].Parent {
			rels = append(rels, blocks(tasks[i].ID, tasks[j].ID))
		}
	}
	return NewSnapshot(all, rels)
}

func TestPerfBoardHasAboutAsManyBlocksAsItems(t *testing.T) {
	for _, n := range []int{500, 4000} {
		s := perfBoard(n)
		got, rels := len(s.Items()), len(s.Relations)
		if got < n || got > n+20 {
			t.Errorf("n=%d: board has %d items", n, got)
		}
		if rels < got*8/10 || rels > got*12/10 {
			t.Errorf("n=%d: %d blocks relations for %d items, want about one per item", n, rels, got)
		}
		types := map[Type]bool{}
		for _, x := range s.Items() {
			types[x.Type] = true
		}
		for _, typ := range []Type{TypeEpic, TypeStory, TypeTask, TypeSubtask} {
			if !types[typ] {
				t.Errorf("n=%d: no %v items", n, typ)
			}
		}
	}
}

// minNextTime is the best of runs timings of the core next on s.
func minNextTime(s *Snapshot, runs int) time.Duration {
	best := time.Duration(1<<63 - 1)
	for i := 0; i < runs; i++ {
		start := time.Now()
		Available(s, NextFilter{}, now)
		if d := time.Since(start); d < best {
			best = d
		}
	}
	return best
}

func TestNextTimeGrowsLinearlyWithBoard(t *testing.T) {
	if testing.Short() {
		t.Skip("timing test skipped in short mode")
	}
	const n, runs, maxRatio = 2000, 7, 8.0
	small, big := perfBoard(n), perfBoard(4*n)
	minNextTime(small, 2) // warm up
	minNextTime(big, 2)
	ts, tb := minNextTime(small, runs), minNextTime(big, runs)
	ratio := float64(tb) / float64(ts)
	t.Logf("N=%d: %v, 4N: %v, ratio %.2f (linear ~4, quadratic ~16)", n, ts, tb, ratio)
	if ratio > maxRatio {
		t.Fatalf("time ratio %.2f exceeds %.0f: next is not linear in the board size", ratio, maxRatio)
	}
}

func BenchmarkNext(b *testing.B) {
	for _, n := range []int{2000, 8000} {
		s := perfBoard(n)
		b.Run(fmt.Sprintf("N=%d", n), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				Available(s, NextFilter{}, now)
			}
		})
	}
}
