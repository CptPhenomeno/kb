package core

import (
	"cmp"
	"slices"
	"strconv"
	"strings"
)

// Snapshot is an in-memory view of the part of the board a rule needs:
// items (with their parent links) and relations. Rules are pure functions
// over a Snapshot, so they can be tested without a database.
type Snapshot struct {
	items     map[string]*Item
	Relations []Relation
}

// NewSnapshot builds a snapshot from items and relations. Items are shared,
// not copied.
func NewSnapshot(items []*Item, rels []Relation) *Snapshot {
	s := &Snapshot{items: make(map[string]*Item, len(items)), Relations: rels}
	for _, it := range items {
		s.items[it.ID] = it
	}
	return s
}

// Get returns the item with the given ID, or nil.
func (s *Snapshot) Get(id string) *Item { return s.items[id] }

// Items returns every item, ordered by ID.
func (s *Snapshot) Items() []*Item {
	out := make([]*Item, 0, len(s.items))
	for _, it := range s.items {
		out = append(out, it)
	}
	slices.SortFunc(out, func(a, b *Item) int { return CompareIDs(a.ID, b.ID) })
	return out
}

// Children returns the direct children of an item, ordered by position and ID.
func (s *Snapshot) Children(id string) []*Item {
	var out []*Item
	for _, it := range s.items {
		if it.Parent == id && id != "" {
			out = append(out, it)
		}
	}
	slices.SortFunc(out, byPosition)
	return out
}

// Ancestors returns the parent chain of an item, nearest first.
func (s *Snapshot) Ancestors(id string) []*Item {
	var out []*Item
	seen := map[string]bool{id: true}
	cur := s.items[id]
	for cur != nil && cur.Parent != "" && !seen[cur.Parent] {
		seen[cur.Parent] = true
		cur = s.items[cur.Parent]
		if cur != nil {
			out = append(out, cur)
		}
	}
	return out
}

// Descendants returns every item below id, depth first, children ordered
// by position.
func (s *Snapshot) Descendants(id string) []*Item {
	var out []*Item
	var walk func(string)
	walk = func(parent string) {
		for _, c := range s.Children(parent) {
			out = append(out, c)
			walk(c.ID)
		}
	}
	walk(id)
	return out
}

// isDescendant reports whether candidate lies below id.
func (s *Snapshot) isDescendant(id, candidate string) bool {
	for _, a := range s.Ancestors(candidate) {
		if a.ID == id {
			return true
		}
	}
	return false
}

func byPosition(a, b *Item) int {
	if c := cmp.Compare(a.Position, b.Position); c != 0 {
		return c
	}
	return CompareIDs(a.ID, b.ID)
}

// CompareIDs orders item IDs naturally, so that KB-9 comes before KB-10.
func CompareIDs(a, b string) int {
	ap, an := splitID(a)
	bp, bn := splitID(b)
	if c := cmp.Compare(ap, bp); c != 0 {
		return c
	}
	if an >= 0 && bn >= 0 {
		if c := cmp.Compare(an, bn); c != 0 {
			return c
		}
	}
	return cmp.Compare(a, b)
}

// splitID splits "KB-17" into ("KB", 17). IDs without a numeric suffix
// return -1 as number.
func splitID(id string) (string, int) {
	i := strings.LastIndexByte(id, '-')
	if i < 0 {
		return id, -1
	}
	n, err := strconv.Atoi(id[i+1:])
	if err != nil {
		return id, -1
	}
	return id[:i], n
}
