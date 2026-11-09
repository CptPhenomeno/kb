package core

import (
	"fmt"
	"slices"
)

// sameRelation reports whether a and b describe the same link. relates is
// symmetric; blocks and duplicates are directed.
func sameRelation(a, b Relation) bool {
	if a.Kind != b.Kind {
		return false
	}
	if a.From == b.From && a.To == b.To {
		return true
	}
	return a.Kind == RelRelates && a.From == b.To && a.To == b.From
}

// HasRelation reports whether rels already contains r.
func HasRelation(rels []Relation, r Relation) bool {
	for _, x := range rels {
		if sameRelation(x, r) {
			return true
		}
	}
	return false
}

// CreatesBlocksCycle reports whether adding r to rels would close a cycle
// of blocks relations.
func CreatesBlocksCycle(rels []Relation, r Relation) bool {
	if r.Kind != RelBlocks {
		return false
	}
	// A cycle exists if r.From is reachable from r.To through blocks edges.
	next := map[string][]string{}
	for _, x := range rels {
		if x.Kind == RelBlocks {
			next[x.From] = append(next[x.From], x.To)
		}
	}
	seen := map[string]bool{}
	stack := []string{r.To}
	for len(stack) > 0 {
		cur := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if cur == r.From {
			return true
		}
		if seen[cur] {
			continue
		}
		seen[cur] = true
		stack = append(stack, next[cur]...)
	}
	return false
}

// ValidateLink checks that r can be added to the board: both items exist,
// the link is not a self link and a blocks link does not close a cycle.
func ValidateLink(s *Snapshot, r Relation) error {
	for _, id := range []string{r.From, r.To} {
		if s.Get(id) == nil {
			return wrapf(ErrNotFound, "item %s", id)
		}
	}
	if r.From == r.To {
		return NewRuleError(fmt.Sprintf("%s cannot be linked to itself", r.From))
	}
	if CreatesBlocksCycle(s.Relations, r) {
		return NewRuleError(fmt.Sprintf("%s blocks %s would create a cycle of blocks", r.From, r.To))
	}
	return nil
}

// Blockers reports whether an item is blocked and by which items. An item
// is blocked when it or one of its ancestors is the target of a blocks
// relation whose source is not done (SPEC.md §3.4).
func Blockers(s *Snapshot, id string) (bool, []string) {
	targets := map[string]bool{id: true}
	for _, a := range s.Ancestors(id) {
		targets[a.ID] = true
	}
	var by []string
	for _, r := range s.Relations {
		if r.Kind != RelBlocks || !targets[r.To] {
			continue
		}
		src := s.Get(r.From)
		if src == nil || src.Status == StatusDone || slices.Contains(by, r.From) {
			continue
		}
		by = append(by, r.From)
	}
	slices.SortFunc(by, CompareIDs)
	return len(by) > 0, by
}
