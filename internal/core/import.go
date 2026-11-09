package core

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
)

// ImportOptions control Import.
type ImportOptions struct {
	Update bool // Refs already on the board update their item instead of failing.
	DryRun bool // Validate and report, then roll everything back.
}

// ImportResult is the outcome for one item of the plan.
type ImportResult struct {
	Ref     string
	ID      string
	Outcome string   // "created", "updated" or "unchanged"
	Changes []string // For updates: what changed, e.g. "title", "tests +1".
}

// ImportReport is the outcome of an import, in plan order.
type ImportReport struct {
	Items                       []ImportResult
	Created, Updated, Unchanged int
	DryRun                      bool
}

var errDryRun = errors.New("dry run")

// Import creates, or with Update updates, every item of the plan in one
// transaction (SPEC.md §7): the whole plan is validated against the board
// first and every problem is reported at once; if there is any, nothing is
// written. Status changes propagate once, after all items are written.
func (s *Service) Import(ctx context.Context, actor string, p *ImportPlan, opts ImportOptions) (*ImportReport, error) {
	var rep *ImportReport
	err := s.write(ctx, actor, func(o *op) (err error) {
		if rep, err = o.importPlan(p, opts); err != nil {
			return err
		}
		if opts.DryRun {
			return errDryRun
		}
		return nil
	})
	if errors.Is(err, errDryRun) {
		err = nil
	}
	if err != nil {
		return nil, err
	}
	rep.DryRun = opts.DryRun
	return rep, nil
}

// planned is the working state of one plan item during an import.
type planned struct {
	in        *ImportItem
	item      *Item // Target state: a new item or a copy of the stored one.
	existing  *Item // Stored item, nil for new items.
	changes   []string
	oldParent string
}

func (o *op) importPlan(p *ImportPlan, opts ImportOptions) (*ImportReport, error) {
	snap, err := o.tx.Snapshot()
	if err != nil {
		return nil, err
	}
	byRef := map[string]*Item{}
	for _, it := range snap.Items() {
		if it.Ref != "" {
			byRef[it.Ref] = it
		}
	}

	var problems []string
	problem := func(in *ImportItem, format string, args ...any) {
		problems = append(problems, fmt.Sprintf("%s (%s): %s", in.Pos, in.Ref, fmt.Sprintf(format, args...)))
	}

	// 1. Build the target state of every item.
	var items []*planned
	plan := map[string]*planned{}
	for _, in := range p.Items {
		if _, dup := plan[in.Ref]; dup {
			problem(in, "ref %q is defined twice in the plan", in.Ref)
			continue
		}
		pl := &planned{in: in}
		if ex := byRef[in.Ref]; ex != nil {
			if !opts.Update {
				problem(in, "ref %q is already used by %s (use --update to update it)", in.Ref, ex.ID)
				continue
			}
			pl.existing = ex
			cp := *ex
			cp.Labels, cp.Links = slices.Clone(ex.Labels), slices.Clone(ex.Links)
			cp.Tests, cp.Acceptance = slices.Clone(ex.Tests), slices.Clone(ex.Acceptance)
			pl.item = &cp
			pl.oldParent = ex.Parent
			if in.Type != nil && *in.Type != ex.Type {
				problem(in, "type cannot change (%s is a %s)", ex.ID, ex.Type)
			}
			pl.changes = applyPlanFields(pl.item, in)
		} else {
			if in.Type == nil {
				problem(in, "type is required for a new item")
				continue
			}
			pl.item = newPlannedItem(in)
			if pl.item.ID, err = o.tx.NextID(o.s.Prefix); err != nil {
				return nil, err
			}
		}
		plan[in.Ref] = pl
		items = append(items, pl)
	}

	// resolve maps a plan ref, an existing ID or an existing ref to an ID.
	resolve := func(key string) string {
		if pl := plan[key]; pl != nil {
			return pl.item.ID
		}
		if snap.Get(key) != nil {
			return key
		}
		if it := byRef[key]; it != nil {
			return it.ID
		}
		return ""
	}

	// 2. Parents.
	for _, pl := range items {
		if pl.in.ParentRef == "" {
			continue // new root item, or existing item keeping its parent
		}
		id := resolve(pl.in.ParentRef)
		if id == "" {
			problem(pl.in, "unknown parent %q", pl.in.ParentRef)
			continue
		}
		if id != pl.item.Parent {
			if pl.existing != nil {
				pl.changes = append(pl.changes, "parent")
			}
			pl.item.Parent = id
		}
	}

	// The board as it will be after the import.
	trialItems := []*Item{}
	for _, it := range snap.Items() {
		if pl := plan[it.Ref]; pl != nil && it.Ref != "" && pl.existing != nil {
			continue
		}
		trialItems = append(trialItems, it)
	}
	for _, pl := range items {
		trialItems = append(trialItems, pl.item)
	}
	rels := slices.Clone(snap.Relations)
	trial := NewSnapshot(trialItems, rels)

	// 3. Fields and hierarchy.
	for _, pl := range items {
		if err := ValidateItem(pl.item); err != nil {
			problem(pl.in, "%s", ruleMessage(err))
		}
		var parentType *Type
		if pl.item.Parent != "" {
			if parent := trial.Get(pl.item.Parent); parent != nil {
				parentType = &parent.Type
			}
		}
		if err := ValidateParent(pl.item.Type, parentType); err != nil {
			problem(pl.in, "%s", ruleMessage(err))
		}
		if hasParentCycle(trial, pl.item.ID) {
			problem(pl.in, "%s would become its own ancestor", pl.item.ID)
		}
	}

	// 4. Relations.
	var newRels []Relation
	relCount := map[string]int{}
	for _, pl := range items {
		for _, group := range []struct {
			kind    RelationKind
			targets []string
		}{{RelBlocks, pl.in.Blocks}, {RelRelates, pl.in.Relates}, {RelDuplicates, pl.in.Duplicates}} {
			for _, target := range group.targets {
				to := resolve(target)
				switch {
				case to == "":
					problem(pl.in, "unknown item %q in %s", target, group.kind)
					continue
				case to == pl.item.ID:
					problem(pl.in, "%s cannot be linked to itself", target)
					continue
				}
				r := Relation{Kind: group.kind, From: pl.item.ID, To: to}
				if HasRelation(trial.Relations, r) {
					continue
				}
				if CreatesBlocksCycle(trial.Relations, r) {
					problem(pl.in, "%s blocks %s would create a cycle of blocks", pl.in.Ref, target)
					continue
				}
				trial.Relations = append(trial.Relations, r)
				newRels = append(newRels, r)
				relCount[pl.in.Ref]++
			}
		}
	}

	// 5. Initial status of new items. Items with children start where the
	// plan says and are then settled by propagation.
	for _, pl := range items {
		st := pl.item.Status
		if pl.existing != nil || st == StatusBacklog || len(trial.Children(pl.item.ID)) > 0 {
			continue
		}
		if unmet := CheckRequirements(trial, pl.item, st); len(unmet) > 0 {
			problem(pl.in, "cannot start in %s: %s", st, strings.Join(unmet, "; "))
		}
	}

	if len(problems) > 0 {
		return nil, NewRuleError(fmt.Sprintf("the plan has %d problems; nothing was imported", len(problems)), problems...)
	}

	// 6. Write: new items first (plan order puts parents first), then
	// updates, then relations.
	rep := &ImportReport{}
	for _, pl := range items {
		if pl.existing != nil {
			continue
		}
		it := pl.item
		if it.Position, err = o.tx.NextPosition(it.Status); err != nil {
			return nil, err
		}
		it.CreatedAt, it.CreatedBy, it.UpdatedAt, it.UpdatedBy = o.now, o.actor, o.now, o.actor
		if err := o.tx.InsertItem(it); err != nil {
			return nil, err
		}
		if err := o.event(it.ID, EventCreated, map[string]any{"type": it.Type.String(), "title": it.Title,
			"status": it.Status.String(), "import": true}); err != nil {
			return nil, err
		}
	}
	for _, pl := range items {
		if pl.existing == nil || len(pl.changes) == 0 {
			continue
		}
		if err := o.save(pl.item); err != nil {
			return nil, err
		}
		if err := o.event(pl.item.ID, EventUpdated, map[string]any{"fields": pl.changes, "import": true}); err != nil {
			return nil, err
		}
	}
	for _, r := range newRels {
		if err := o.tx.AddRelation(r); err != nil {
			return nil, err
		}
		if err := o.relationEvents(EventLinked, r); err != nil {
			return nil, err
		}
	}

	// 7. Propagate once, deepest items first.
	starts := map[string]bool{}
	for _, pl := range items {
		created := pl.existing == nil
		if created || len(pl.changes) > 0 || relCount[pl.in.Ref] > 0 {
			starts[pl.item.ID] = true
			starts[pl.item.Parent] = true
			starts[pl.oldParent] = true
		}
	}
	delete(starts, "")
	final, err := o.tx.Snapshot()
	if err != nil {
		return nil, err
	}
	order := make([]string, 0, len(starts))
	for id := range starts {
		order = append(order, id)
	}
	slices.SortFunc(order, func(a, b string) int {
		if c := cmp.Compare(len(final.Ancestors(b)), len(final.Ancestors(a))); c != 0 {
			return c
		}
		return CompareIDs(a, b)
	})
	for _, id := range order {
		if _, err := o.propagate(id); err != nil {
			return nil, err
		}
	}

	// 8. Report.
	for _, pl := range items {
		res := ImportResult{Ref: pl.in.Ref, ID: pl.item.ID}
		changes := pl.changes
		if n := relCount[pl.in.Ref]; n > 0 && pl.existing != nil {
			changes = append(changes, fmt.Sprintf("relations +%d", n))
		}
		switch {
		case pl.existing == nil:
			res.Outcome = "created"
			rep.Created++
		case len(changes) > 0:
			res.Outcome, res.Changes = "updated", changes
			rep.Updated++
		default:
			res.Outcome = "unchanged"
			rep.Unchanged++
		}
		rep.Items = append(rep.Items, res)
	}
	return rep, nil
}

// newPlannedItem builds a new item from a plan item.
func newPlannedItem(in *ImportItem) *Item {
	it := &Item{Ref: in.Ref, Type: *in.Type}
	applyPlanFields(it, in)
	if in.Status != nil {
		it.Status = *in.Status
	}
	return it
}

// applyPlanFields copies the fields present in the plan item onto it and
// returns the names of the fields that changed. Tests and criteria are
// merged by name: new ones are appended, existing ones keep their state.
func applyPlanFields(it *Item, in *ImportItem) []string {
	var changes []string
	set := func(name string, dst *string, v *string) {
		if v != nil && *v != *dst {
			*dst = *v
			changes = append(changes, name)
		}
	}
	set("title", &it.Title, in.Title)
	set("description", &it.Description, in.Description)
	set("spec", &it.Spec, in.Spec)
	if in.Priority != nil && *in.Priority != it.Priority {
		it.Priority = *in.Priority
		changes = append(changes, "priority")
	}
	if in.Points != nil && *in.Points != it.Points {
		it.Points = *in.Points
		changes = append(changes, "points")
	}
	if in.Labels != nil && !slices.Equal(*in.Labels, it.Labels) {
		it.Labels = slices.Clone(*in.Labels)
		changes = append(changes, "labels")
	}
	if in.Links != nil && !slices.Equal(*in.Links, it.Links) {
		it.Links = slices.Clone(*in.Links)
		changes = append(changes, "links")
	}
	if in.Tests != nil {
		added := 0
		for _, name := range *in.Tests {
			if !slices.ContainsFunc(it.Tests, func(t Test) bool { return t.Name == name }) {
				it.Tests = append(it.Tests, Test{Name: name})
				added++
			}
		}
		if added > 0 {
			changes = append(changes, fmt.Sprintf("tests +%d", added))
		}
	}
	if in.Acceptance != nil {
		added := 0
		for _, text := range *in.Acceptance {
			if !slices.ContainsFunc(it.Acceptance, func(c Criterion) bool { return c.Text == text }) {
				it.Acceptance = append(it.Acceptance, Criterion{Text: text})
				added++
			}
		}
		if added > 0 {
			changes = append(changes, fmt.Sprintf("acceptance +%d", added))
		}
	}
	return changes
}

// hasParentCycle reports whether following parents from id comes back to id.
func hasParentCycle(s *Snapshot, id string) bool {
	seen := map[string]bool{}
	for cur := s.Get(id); cur != nil && cur.Parent != ""; cur = s.Get(cur.Parent) {
		if cur.Parent == id || seen[cur.Parent] {
			return true
		}
		seen[cur.Parent] = true
	}
	return false
}

// ruleMessage returns the message of a rule error without its sentinel.
func ruleMessage(err error) string {
	var re *RuleError
	if errors.As(err, &re) {
		return re.Error()
	}
	return err.Error()
}
