package core

// ImportPlan is a plan to create or update many items at once with
// `kb import`. Items are listed parents before children.
type ImportPlan struct {
	Items []*ImportItem
}

// ImportItem is one item of a plan. Nil fields were absent from the plan:
// on --update they leave the stored value untouched.
type ImportItem struct {
	Ref       string // Required: the key that matches items across imports.
	Pos       string // Where the item is defined, e.g. "plan.yaml:12".
	ParentRef string // Ref of the enclosing plan item, or an existing ID or ref.

	Type        *Type
	Title       *string
	Description *string
	Spec        *string
	Priority    *Priority
	Points      *int
	Status      *Status // Initial status; ignored for existing items.
	Labels      *[]string
	Links       *[]string
	Acceptance  *[]string
	Tests       *[]string

	// Relations from this item, to refs of the plan or existing IDs/refs.
	Blocks     []string
	Relates    []string
	Duplicates []string
}
