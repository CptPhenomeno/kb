package core

import "fmt"

// allowedParents lists, for every type, the parent types it accepts
// (SPEC.md §3.2). rootAllowed reports whether the type may have no parent.
var (
	allowedParents = map[Type][]Type{
		TypeEpic:    nil,
		TypeStory:   {TypeEpic},
		TypeTask:    {TypeStory},
		TypeBug:     {TypeStory},
		TypeSubtask: {TypeTask, TypeBug},
	}
	rootAllowed = map[Type]bool{
		TypeEpic:  true,
		TypeStory: true,
		TypeTask:  true,
		TypeBug:   true,
	}
)

// ValidateParent checks that an item of type child may sit under a parent
// of type *parent. A nil parent means the item is a root item.
func ValidateParent(child Type, parent *Type) error {
	if parent == nil {
		if rootAllowed[child] {
			return nil
		}
		return NewRuleError(fmt.Sprintf("a %s must have a parent (one of %v)", child, allowedParents[child]))
	}
	for _, p := range allowedParents[child] {
		if p == *parent {
			return nil
		}
	}
	if len(allowedParents[child]) == 0 {
		return NewRuleError(fmt.Sprintf("a %s cannot have a parent", child))
	}
	return NewRuleError(fmt.Sprintf("a %s cannot be placed under a %s (allowed: %v)", child, *parent, allowedParents[child]))
}

// ValidateReparent checks that item id may move under newParent ("" for
// the root): the parent exists, is not the item or one of its descendants,
// and its type is allowed by the hierarchy.
func ValidateReparent(s *Snapshot, id, newParent string) error {
	item := s.Get(id)
	if item == nil {
		return wrapf(ErrNotFound, "item %s", id)
	}
	if newParent == "" {
		return ValidateParent(item.Type, nil)
	}
	parent := s.Get(newParent)
	if parent == nil {
		return wrapf(ErrNotFound, "parent %s", newParent)
	}
	if newParent == id {
		return NewRuleError(fmt.Sprintf("%s cannot be its own parent", id))
	}
	if s.isDescendant(id, newParent) {
		return NewRuleError(fmt.Sprintf("%s cannot move under %s, which is one of its descendants", id, newParent))
	}
	return ValidateParent(item.Type, &parent.Type)
}
