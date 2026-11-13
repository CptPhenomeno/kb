package core

// workIndex holds the indexes `kb next` needs, built once per Available call
// in one pass over items and relations (SPEC.md NEXT-2, NEXT-4): children by
// parent, blocks by non-done source, and the set of blocked targets. SP and
// CBSP are memoized, so the whole board costs O(N + R).
type workIndex struct {
	s        *Snapshot
	children map[string][]*Item
	// blocksOut maps a non-done source to the targets of its blocks relations.
	blocksOut map[string][]string
	// blockedTargets is { Y : Z blocks Y, Z exists and is not done }.
	blockedTargets map[string]bool

	sp       map[string]int
	cbspMemo map[string]int
	visiting map[string]bool
}

func newWorkIndex(s *Snapshot) *workIndex {
	w := &workIndex{
		s:              s,
		children:       map[string][]*Item{},
		blocksOut:      map[string][]string{},
		blockedTargets: map[string]bool{},
		sp:             map[string]int{},
		cbspMemo:       map[string]int{},
		visiting:       map[string]bool{},
	}
	for _, it := range s.items {
		if it.Parent != "" {
			w.children[it.Parent] = append(w.children[it.Parent], it)
		}
	}
	for _, r := range s.Relations {
		if r.Kind != RelBlocks {
			continue
		}
		if src := s.Get(r.From); src == nil || src.Status == StatusDone {
			continue
		}
		w.blocksOut[r.From] = append(w.blocksOut[r.From], r.To)
		w.blockedTargets[r.To] = true
	}
	return w
}

// isBlocked reports whether the item or one of its ancestors is a blocked
// target (SPEC.md §3.4); it agrees with Blockers at a cost proportional to
// the depth of the hierarchy.
func (w *workIndex) isBlocked(id string) bool {
	seen := map[string]bool{}
	for cur := w.s.Get(id); cur != nil && !seen[cur.ID]; cur = w.s.Get(cur.Parent) {
		if w.blockedTargets[cur.ID] {
			return true
		}
		seen[cur.ID] = true
	}
	return false
}

// subtree returns the IDs of every item below id, at any depth, walking the
// children index.
func (w *workIndex) subtree(id string) []string {
	var out []string
	seen := map[string]bool{id: true}
	stack := []string{id}
	for len(stack) > 0 {
		cur := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		for _, c := range w.children[cur] {
			if seen[c.ID] {
				continue
			}
			seen[c.ID] = true
			out = append(out, c.ID)
			stack = append(stack, c.ID)
		}
	}
	return out
}

// storyPoints is SP(Y): own points for task and bug; for story and epic the
// points of every non-done task and bug below; 0 for subtasks and items
// without points.
func (w *workIndex) storyPoints(id string) int {
	if v, ok := w.sp[id]; ok {
		return v
	}
	it := w.s.Get(id)
	total := 0
	switch {
	case it == nil:
	case it.Type == TypeTask || it.Type == TypeBug:
		total = it.Points
	case it.Type.IsParentType():
		for _, d := range w.subtree(id) {
			x := w.s.Get(d)
			if (x.Type == TypeTask || x.Type == TypeBug) && x.Status != StatusDone {
				total += x.Points
			}
		}
	}
	w.sp[id] = total
	return total
}

// cbsp is CriticalBlockedStoryPoints (SPEC.md NEXT-4): the heaviest chain of
// work X blocks, X's own points excluded.
//
//	CBSP(X) = max({SP(Y) + CBSP(Y) : Y in B(X)} U {0})
func (w *workIndex) cbsp(id string) int {
	if v, ok := w.cbspMemo[id]; ok {
		return v
	}
	if w.visiting[id] {
		return 0 // a child blocking its own ancestor: do not loop
	}
	w.visiting[id] = true
	best := 0
	x := w.s.Get(id)
	if x != nil && x.Status != StatusDone {
		for _, n := range append([]string{id}, w.subtree(id)...) {
			src := w.s.Get(n)
			if src.Status == StatusDone {
				continue
			}
			for _, y := range w.blocksOut[n] {
				t := w.s.Get(y)
				if t == nil || t.Status == StatusDone || y == id {
					continue
				}
				if v := w.storyPoints(y) + w.cbsp(y); v > best {
					best = v
				}
			}
		}
	}
	w.visiting[id] = false
	w.cbspMemo[id] = best
	return best
}
