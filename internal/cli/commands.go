package cli

import (
	"errors"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/CptPhenomeno/kb/internal/buildinfo"
	"github.com/CptPhenomeno/kb/internal/core"
	"github.com/CptPhenomeno/kb/internal/store"
)

// ---- helpers ----

// resolveID accepts an item ID or a ref.
func resolveID(s *Session, c *cli, idOrRef string) (string, error) {
	if _, err := s.Service.Get(c.ctx, idOrRef); err == nil {
		return idOrRef, nil
	} else if !errors.Is(err, core.ErrNotFound) {
		return "", err
	}
	it, err := s.Service.GetByRef(c.ctx, idOrRef)
	if err != nil {
		return "", fmt.Errorf("item %q: %w", idOrRef, core.ErrNotFound)
	}
	return it.ID, nil
}

// readText returns v, or stdin when v is "-".
func (c *cli) readText(v string) (string, error) {
	if v != "-" {
		return v, nil
	}
	b, err := io.ReadAll(c.app.Stdin)
	return string(b), err
}

// printItem prints an item: JSON, its ID with --quiet, or a confirmation.
func (c *cli) printItem(s *Session, it *core.Item, msg string) error {
	switch {
	case c.g.json:
		snap, err := s.Service.Board(c.ctx)
		if err != nil {
			return err
		}
		return printJSON(c.app.Stdout, newItemView(snap, it, c.app.Now()))
	case c.g.quiet:
		_, err := fmt.Fprintln(c.app.Stdout, it.ID)
		return err
	default:
		_, err := fmt.Fprintf(c.app.Stdout, "✓ %s\n", msg)
		return err
	}
}

// ---- commands ----

func (c *cli) versionCmd() *cobra.Command {
	return &cobra.Command{
		Use: "version", Short: "Print the kb version", Args: args(0, 0),
		RunE: func(*cobra.Command, []string) error {
			_, err := fmt.Fprintln(c.app.Stdout, buildinfo.String())
			return err
		},
	}
}

func (c *cli) initCmd() *cobra.Command {
	var prefix string
	cmd := &cobra.Command{
		Use: "init", Short: "Create the board in the project root", Args: args(0, 0),
		RunE: func(*cobra.Command, []string) error {
			root, err := store.ResolveRoot(c.app.Cwd, c.app.Getenv)
			if errors.Is(err, store.ErrNoBoard) {
				root, err = c.app.Cwd, nil
			}
			if err != nil {
				return err
			}
			if err := store.Init(root, prefix); err != nil {
				return err
			}
			if !c.g.quiet && !c.g.json {
				fmt.Fprintf(c.app.Stdout, "✓ board created in %s/.kanban\n", root)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&prefix, "prefix", "", "item ID prefix (default KB)")
	return cmd
}

// itemFlags are the fields shared by add and edit.
type itemFlags struct {
	desc, prio, spec, parent, ref, status string
	points                                int
	labels, links, accept, tests          []string
}

func (f *itemFlags) register(cmd *cobra.Command, withCreateOnly bool) {
	fl := cmd.Flags()
	fl.StringVar(&f.desc, "desc", "", "description in markdown (- reads stdin)")
	fl.StringVar(&f.prio, "prio", "", "priority: high, medium or low")
	fl.IntVar(&f.points, "points", 0, "story points: 1, 2, 3, 5, 8 or 13")
	fl.StringVar(&f.spec, "spec", "", "reference to the specification, e.g. SPEC.md#auth")
	fl.StringVar(&f.parent, "parent", "", "parent item ID")
	fl.StringVar(&f.ref, "ref", "", "stable symbolic key, unique in the board")
	fl.StringArrayVar(&f.labels, "label", nil, "label (repeatable)")
	fl.StringArrayVar(&f.links, "link", nil, "link to a PR, branch or issue (repeatable)")
	if withCreateOnly {
		fl.StringVar(&f.status, "status", "", "initial status (default backlog)")
		fl.StringArrayVar(&f.accept, "accept", nil, "acceptance criterion (repeatable)")
		fl.StringArrayVar(&f.tests, "test", nil, "test name (repeatable)")
	}
}

func (c *cli) addCmd() *cobra.Command {
	var f itemFlags
	cmd := &cobra.Command{
		Use: "add <type> <title>", Short: "Create an item (epic, story, task, subtask, bug)", Args: args(2, 2),
		RunE: func(_ *cobra.Command, a []string) error {
			typ, err := core.ParseType(a[0])
			if err != nil {
				return err
			}
			it := core.Item{Type: typ, Title: a[1], Spec: f.spec, Ref: f.ref, Points: f.points,
				Labels: f.labels, Links: f.links}
			if it.Description, err = c.readText(f.desc); err != nil {
				return err
			}
			if f.prio != "" {
				if it.Priority, err = core.ParsePriority(f.prio); err != nil {
					return err
				}
			}
			if f.status != "" {
				if it.Status, err = core.ParseStatus(f.status); err != nil {
					return err
				}
			}
			for _, t := range f.tests {
				it.Tests = append(it.Tests, core.Test{Name: t})
			}
			for _, x := range f.accept {
				it.Acceptance = append(it.Acceptance, core.Criterion{Text: x})
			}
			return c.with(func(s *Session) error {
				if f.parent != "" {
					if it.Parent, err = resolveID(s, c, f.parent); err != nil {
						return err
					}
				}
				created, err := s.Service.Create(c.ctx, s.Actor, it)
				if err != nil {
					return err
				}
				return c.printItem(s, created, fmt.Sprintf("created %s (%s) in %s", created.ID, created.Type, created.Status))
			})
		},
	}
	f.register(cmd, true)
	return cmd
}

func (c *cli) showCmd() *cobra.Command {
	return &cobra.Command{
		Use: "show <id|ref>", Short: "Show an item with its history", Args: args(1, 1),
		RunE: func(_ *cobra.Command, a []string) error {
			return c.with(func(s *Session) error {
				id, err := resolveID(s, c, a[0])
				if err != nil {
					return err
				}
				snap, err := s.Service.Board(c.ctx)
				if err != nil {
					return err
				}
				history, err := s.Service.History(c.ctx, id)
				if err != nil {
					return err
				}
				it, now := snap.Get(id), c.app.Now()
				switch {
				case c.g.json:
					v := newItemView(snap, it, now)
					for _, e := range history {
						v.History = append(v.History, newEventView(e))
					}
					return printJSON(c.app.Stdout, v)
				case c.g.quiet:
					_, err := fmt.Fprintln(c.app.Stdout, it.ID)
					return err
				}
				writeItem(c.app.Stdout, snap, it, history, now)
				return nil
			})
		},
	}
}

// listFlags select items in list.
type listFlags struct {
	status, typ, assignee, label, parent, sort string
	blocked, stale                             bool
}

func (c *cli) listCmd() *cobra.Command {
	var f listFlags
	cmd := &cobra.Command{
		Use: "list", Short: "List items", Args: args(0, 0),
		RunE: func(*cobra.Command, []string) error {
			return c.with(func(s *Session) error {
				snap, err := s.Service.Board(c.ctx)
				if err != nil {
					return err
				}
				items, err := filterItems(s, c, snap, f)
				if err != nil {
					return err
				}
				return c.printItems(snap, items)
			})
		},
	}
	fl := cmd.Flags()
	fl.StringVar(&f.status, "status", "", "only items in this status")
	fl.StringVar(&f.typ, "type", "", "only items of this type")
	fl.StringVar(&f.assignee, "assignee", "", "only items assigned to this actor")
	fl.StringVar(&f.label, "label", "", "only items with this label")
	fl.StringVar(&f.parent, "parent", "", "only descendants of this item")
	fl.BoolVar(&f.blocked, "blocked", false, "only blocked items")
	fl.BoolVar(&f.stale, "stale", false, "only items whose claim expired")
	fl.StringVar(&f.sort, "sort", "id", "order: id or position (board order)")
	return cmd
}

func filterItems(s *Session, c *cli, snap *core.Snapshot, f listFlags) ([]*core.Item, error) {
	var status *core.Status
	if f.status != "" {
		st, err := core.ParseStatus(f.status)
		if err != nil {
			return nil, err
		}
		status = &st
	}
	var typ *core.Type
	if f.typ != "" {
		t, err := core.ParseType(f.typ)
		if err != nil {
			return nil, err
		}
		typ = &t
	}
	var scope map[string]bool
	if f.parent != "" {
		id, err := resolveID(s, c, f.parent)
		if err != nil {
			return nil, err
		}
		scope = map[string]bool{}
		for _, d := range snap.Descendants(id) {
			scope[d.ID] = true
		}
	}
	now := c.app.Now()
	var out []*core.Item
	for _, it := range snap.Items() {
		blocked, _ := core.Blockers(snap, it.ID)
		switch {
		case status != nil && it.Status != *status,
			typ != nil && it.Type != *typ,
			f.assignee != "" && it.Assignee != f.assignee,
			f.label != "" && !slices.Contains(it.Labels, f.label),
			scope != nil && !scope[it.ID],
			f.blocked && !blocked,
			f.stale && !core.IsStale(it, now):
			continue
		}
		out = append(out, it)
	}
	switch f.sort {
	case "id":
	case "position":
		slices.SortStableFunc(out, func(a, b *core.Item) int {
			if a.Status != b.Status {
				return int(a.Status) - int(b.Status)
			}
			return a.Position - b.Position
		})
	default:
		return nil, &usageError{msg: fmt.Sprintf("unknown sort %q: use id or position", f.sort)}
	}
	return out, nil
}

func (c *cli) printItems(snap *core.Snapshot, items []*core.Item) error {
	now := c.app.Now()
	switch {
	case c.g.json:
		views := make([]itemView, 0, len(items))
		for _, it := range items {
			views = append(views, newItemView(snap, it, now))
		}
		return printJSON(c.app.Stdout, views)
	case c.g.quiet:
		for _, it := range items {
			fmt.Fprintln(c.app.Stdout, it.ID)
		}
		return nil
	}
	writeList(c.app.Stdout, snap, items, now)
	return nil
}

func (c *cli) treeCmd() *cobra.Command {
	return &cobra.Command{
		Use: "tree [id]", Short: "Show the hierarchy with progress and points", Args: args(0, 1),
		RunE: func(_ *cobra.Command, a []string) error {
			return c.with(func(s *Session) error {
				snap, err := s.Service.Board(c.ctx)
				if err != nil {
					return err
				}
				var roots []*core.Item
				if len(a) == 1 {
					id, err := resolveID(s, c, a[0])
					if err != nil {
						return err
					}
					roots = []*core.Item{snap.Get(id)}
				} else {
					for _, it := range snap.Items() {
						if it.Parent == "" {
							roots = append(roots, it)
						}
					}
				}
				if c.g.json || c.g.quiet {
					var items []*core.Item
					for _, r := range roots {
						items = append(append(items, r), snap.Descendants(r.ID)...)
					}
					return c.printItems(snap, items)
				}
				writeTree(c.app.Stdout, snap, roots, c.app.Now())
				return nil
			})
		},
	}
}

func (c *cli) editCmd() *cobra.Command {
	var f itemFlags
	var title string
	var ifVersion int
	cmd := &cobra.Command{
		Use: "edit <id> [flags]", Short: "Change the fields of an item (no flags: open $EDITOR)", Args: args(1, 1),
		RunE: func(cmd *cobra.Command, a []string) error {
			return c.with(func(s *Session) error {
				id, err := resolveID(s, c, a[0])
				if err != nil {
					return err
				}
				var p core.Patch
				changed := cmd.Flags().Changed
				if !changed("title") && !changed("desc") && !changed("prio") && !changed("points") &&
					!changed("spec") && !changed("parent") && !changed("ref") && !changed("label") && !changed("link") {
					return c.editInEditor(s, id, ifVersion)
				}
				if changed("title") {
					p.Title = &title
				}
				if changed("desc") {
					d, err := c.readText(f.desc)
					if err != nil {
						return err
					}
					p.Description = &d
				}
				if changed("prio") {
					pr, err := core.ParsePriority(f.prio)
					if err != nil {
						return err
					}
					p.Priority = &pr
				}
				if changed("points") {
					p.Points = &f.points
				}
				if changed("spec") {
					p.Spec = &f.spec
				}
				if changed("ref") {
					p.Ref = &f.ref
				}
				if changed("parent") {
					parent := f.parent
					if parent != "" {
						if parent, err = resolveID(s, c, parent); err != nil {
							return err
						}
					}
					p.Parent = &parent
				}
				if changed("label") {
					p.Labels = &f.labels
				}
				if changed("link") {
					p.Links = &f.links
				}
				it, err := s.Service.Edit(c.ctx, s.Actor, id, p, ifVersion)
				if err != nil {
					return err
				}
				return c.printItem(s, it, fmt.Sprintf("%s updated (version %d)", it.ID, it.Version))
			})
		},
	}
	f.register(cmd, false)
	cmd.Flags().StringVar(&title, "title", "", "new title")
	cmd.Flags().IntVar(&ifVersion, "if-version", 0, "fail with exit 4 unless the item is at this version")
	return cmd
}

func (c *cli) moveCmd() *cobra.Command {
	var ifVersion, position int
	cmd := &cobra.Command{
		Use: "move <id> <status>", Short: "Move an item to a column, if its requirements are met", Args: args(2, 2),
		RunE: func(cmd *cobra.Command, a []string) error {
			to, err := core.ParseStatus(a[1])
			if err != nil {
				return err
			}
			return c.with(func(s *Session) error {
				id, err := resolveID(s, c, a[0])
				if err != nil {
					return err
				}
				before, err := s.Service.Get(c.ctx, id)
				if err != nil {
					return err
				}
				res, err := s.Service.Move(c.ctx, s.Actor, id, to, ifVersion)
				if err != nil {
					return err
				}
				it := res.Item
				if cmd.Flags().Changed("position") {
					if it, err = s.Service.Reorder(c.ctx, s.Actor, id, position, 0); err != nil {
						return err
					}
				}
				switch {
				case c.g.json:
					derived := []map[string]string{}
					for _, d := range res.Derived {
						derived = append(derived, map[string]string{"id": d.ID, "from": d.From.String(), "to": d.To.String()})
					}
					return printJSON(c.app.Stdout, map[string]any{"id": it.ID, "from": before.Status.String(),
						"to": it.Status.String(), "position": it.Position, "version": it.Version, "derived": derived})
				case c.g.quiet:
					_, err := fmt.Fprintln(c.app.Stdout, it.ID)
					return err
				}
				fmt.Fprintf(c.app.Stdout, "✓ %s: %s → %s (version %d)\n", it.ID, before.Status, it.Status, it.Version)
				for _, d := range res.Derived {
					fmt.Fprintf(c.app.Stdout, "  ↳ %s: %s → %s (derived)\n", d.ID, d.From, d.To)
				}
				return nil
			})
		},
	}
	cmd.Flags().IntVar(&ifVersion, "if-version", 0, "fail with exit 4 unless the item is at this version")
	cmd.Flags().IntVar(&position, "position", 0, "place the item at this index of the column (0 is the top)")
	return cmd
}

func (c *cli) linkCmd(add bool) *cobra.Command {
	use, short := "link <id> blocks|relates|duplicates <id>", "Link two items"
	if !add {
		use, short = "unlink <id> blocks|relates|duplicates <id>", "Remove a link between two items"
	}
	return &cobra.Command{
		Use: use, Short: short, Args: args(3, 3),
		RunE: func(_ *cobra.Command, a []string) error {
			kind, err := core.ParseRelationKind(a[1])
			if err != nil {
				return err
			}
			return c.with(func(s *Session) error {
				from, err := resolveID(s, c, a[0])
				if err != nil {
					return err
				}
				to, err := resolveID(s, c, a[2])
				if err != nil {
					return err
				}
				r := core.Relation{Kind: kind, From: from, To: to}
				verb := "linked"
				if add {
					err = s.Service.Link(c.ctx, s.Actor, r)
				} else {
					verb = "unlinked"
					err = s.Service.Unlink(c.ctx, s.Actor, r)
				}
				if err != nil {
					return err
				}
				if c.g.json {
					return printJSON(c.app.Stdout, map[string]any{"ok": true, "from": from, "kind": kind.String(), "to": to})
				}
				if !c.g.quiet {
					fmt.Fprintf(c.app.Stdout, "✓ %s: %s %s %s\n", verb, from, kind, to)
				}
				return nil
			})
		},
	}
}

func (c *cli) nextCmd() *cobra.Command {
	var types []string
	var label, parent string
	var claim bool
	cmd := &cobra.Command{
		Use: "next", Short: "Show (and optionally claim) the next available item (tasks and bugs by default)", Args: args(0, 0),
		RunE: func(*cobra.Command, []string) error {
			var f core.NextFilter
			for _, t := range types {
				typ, err := core.ParseType(t)
				if err != nil {
					return err
				}
				f.Types = append(f.Types, typ)
			}
			f.Label = label
			return c.with(func(s *Session) error {
				if parent != "" {
					id, err := resolveID(s, c, parent)
					if err != nil {
						return err
					}
					f.Parent = id
				}
				it, err := s.Service.Next(c.ctx, s.Actor, f, claim)
				if err != nil {
					return err
				}
				msg := fmt.Sprintf("next: %s %s %q", it.ID, it.Type, it.Title)
				if claim {
					msg = fmt.Sprintf("claimed %s %s %q until %s", it.ID, it.Type, it.Title, it.Claim.ExpiresAt.UTC().Format("15:04 UTC"))
				}
				return c.printItem(s, it, msg)
			})
		},
	}
	cmd.Flags().StringSliceVar(&types, "type", nil, "only these types: epic, story, task, bug (comma separated or repeated; default task and bug)")
	cmd.Flags().StringVar(&label, "label", "", "only items with this label")
	cmd.Flags().StringVar(&parent, "parent", "", "only descendants of this item")
	cmd.Flags().BoolVar(&claim, "claim", false, "claim the item atomically")
	return cmd
}

// simpleItemCmd builds a command taking one item ID and calling op.
func (c *cli) simpleItemCmd(use, short string, withVersion bool,
	op func(s *Session, id string, ifVersion int) (*core.Item, string, error)) *cobra.Command {
	var ifVersion int
	cmd := &cobra.Command{
		Use: use, Short: short, Args: args(1, 1),
		RunE: func(_ *cobra.Command, a []string) error {
			return c.with(func(s *Session) error {
				id, err := resolveID(s, c, a[0])
				if err != nil {
					return err
				}
				it, msg, err := op(s, id, ifVersion)
				if err != nil {
					return err
				}
				return c.printItem(s, it, msg)
			})
		},
	}
	if withVersion {
		cmd.Flags().IntVar(&ifVersion, "if-version", 0, "fail with exit 4 unless the item is at this version")
	}
	return cmd
}

func (c *cli) claimCmd() *cobra.Command {
	return c.simpleItemCmd("claim <id>", "Take an item for yourself (moves it to doing)", true,
		func(s *Session, id string, v int) (*core.Item, string, error) {
			it, err := s.Service.Claim(c.ctx, s.Actor, id, v)
			if err != nil {
				return nil, "", err
			}
			return it, fmt.Sprintf("%s claimed by %s until %s", it.ID, s.Actor, it.Claim.ExpiresAt.UTC().Format("15:04 UTC")), nil
		})
}

func (c *cli) releaseCmd() *cobra.Command {
	return c.simpleItemCmd("release <id>", "Drop the claim on an item", true,
		func(s *Session, id string, v int) (*core.Item, string, error) {
			it, err := s.Service.Release(c.ctx, s.Actor, id, v)
			if err != nil {
				return nil, "", err
			}
			return it, fmt.Sprintf("%s released (%s)", it.ID, it.Status), nil
		})
}

func (c *cli) touchCmd() *cobra.Command {
	return c.simpleItemCmd("touch <id>", "Renew your claim on an item", false,
		func(s *Session, id string, _ int) (*core.Item, string, error) {
			it, err := s.Service.Touch(c.ctx, s.Actor, id)
			if err != nil {
				return nil, "", err
			}
			return it, fmt.Sprintf("%s claim renewed until %s", it.ID, it.Claim.ExpiresAt.UTC().Format("15:04 UTC")), nil
		})
}

func (c *cli) phaseCmd() *cobra.Command {
	var ifVersion int
	cmd := &cobra.Command{
		Use: "phase <id> red|green|refactor", Short: "Set the TDD phase of a task, subtask or bug", Args: args(2, 2),
		RunE: func(_ *cobra.Command, a []string) error {
			phase, err := core.ParsePhase(a[1])
			if err != nil || phase == core.PhaseNone {
				return fmt.Errorf("%w: unknown tdd phase %q", core.ErrInvalidValue, a[1])
			}
			return c.with(func(s *Session) error {
				id, err := resolveID(s, c, a[0])
				if err != nil {
					return err
				}
				it, err := s.Service.SetPhase(c.ctx, s.Actor, id, phase, ifVersion)
				if err != nil {
					return err
				}
				return c.printItem(s, it, fmt.Sprintf("%s phase %s", it.ID, phase))
			})
		},
	}
	cmd.Flags().IntVar(&ifVersion, "if-version", 0, "fail with exit 4 unless the item is at this version")
	return cmd
}

func (c *cli) testCmd() *cobra.Command {
	var ifVersion int
	cmd := &cobra.Command{
		Use:   "test <id> add <name> | test <id> pass|fail|pending <n|name>... | test <id> edit <n|name> <name> | test <id> rm <n|name>",
		Short: "Add, edit or remove tests, or record their outcome",
		Args:  args(3, -1),
		RunE: func(cmd *cobra.Command, a []string) error {
			return c.with(func(s *Session) error {
				id, err := resolveID(s, c, a[0])
				if err != nil {
					return err
				}
				var it *core.Item
				switch a[1] {
				case "add":
					for _, name := range a[2:] {
						if it, err = s.Service.AddTest(c.ctx, s.Actor, id, name, ifVersion); err != nil {
							return err
						}
						ifVersion = 0
					}
				case "pass", "fail", "pending":
					state := map[string]core.TestState{"pass": core.TestPassing, "fail": core.TestFailing, "pending": core.TestPending}[a[1]]
					for _, sel := range a[2:] {
						if it, err = s.Service.SetTestState(c.ctx, s.Actor, id, sel, state, ifVersion); err != nil {
							return err
						}
						ifVersion = 0
					}
				case "edit":
					if len(a) != 4 {
						return &usageError{msg: "edit takes one test (number or name) and one new name", usage: cmd.UseLine()}
					}
					if it, err = s.Service.EditTest(c.ctx, s.Actor, id, a[2], a[3], ifVersion); err != nil {
						return err
					}
				case "rm":
					if len(a) != 3 {
						return &usageError{msg: "rm takes one test (number or name)", usage: cmd.UseLine()}
					}
					if it, err = s.Service.RemoveTest(c.ctx, s.Actor, id, a[2], ifVersion); err != nil {
						return err
					}
				default:
					return &usageError{msg: fmt.Sprintf("unknown test action %q", a[1]), usage: cmd.UseLine()}
				}
				return c.printItem(s, it, fmt.Sprintf("%s tests %d/%d passing", it.ID, it.PassingTests(), len(it.Tests)))
			})
		},
	}
	cmd.Flags().IntVar(&ifVersion, "if-version", 0, "fail with exit 4 unless the item is at this version")
	return cmd
}

func (c *cli) acceptCmd() *cobra.Command {
	var ifVersion int
	cmd := &cobra.Command{
		Use:   "accept <id> add <text>... | accept <id> check|uncheck <n>... | accept <id> edit <n> <text> | accept <id> rm <n>",
		Short: "Add, check, edit or remove acceptance criteria",
		Args:  args(3, -1),
		RunE: func(cmd *cobra.Command, a []string) error {
			return c.with(func(s *Session) error {
				id, err := resolveID(s, c, a[0])
				if err != nil {
					return err
				}
				var it *core.Item
				switch a[1] {
				case "add":
					for _, text := range a[2:] {
						if it, err = s.Service.AddCriterion(c.ctx, s.Actor, id, text, ifVersion); err != nil {
							return err
						}
						ifVersion = 0
					}
				case "check", "uncheck":
					for _, sel := range a[2:] {
						n, err := strconv.Atoi(sel)
						if err != nil {
							return &usageError{msg: fmt.Sprintf("criterion number expected, got %q", sel), usage: cmd.UseLine()}
						}
						if it, err = s.Service.SetCriterion(c.ctx, s.Actor, id, n, a[1] == "check", ifVersion); err != nil {
							return err
						}
						ifVersion = 0
					}
				case "edit":
					if len(a) != 4 {
						return &usageError{msg: "edit takes one criterion number and one text", usage: cmd.UseLine()}
					}
					n, err := strconv.Atoi(a[2])
					if err != nil {
						return &usageError{msg: fmt.Sprintf("criterion number expected, got %q", a[2]), usage: cmd.UseLine()}
					}
					if it, err = s.Service.EditCriterion(c.ctx, s.Actor, id, n, a[3], ifVersion); err != nil {
						return err
					}
				case "rm":
					if len(a) != 3 {
						return &usageError{msg: "rm takes one criterion number", usage: cmd.UseLine()}
					}
					n, err := strconv.Atoi(a[2])
					if err != nil {
						return &usageError{msg: fmt.Sprintf("criterion number expected, got %q", a[2]), usage: cmd.UseLine()}
					}
					if it, err = s.Service.RemoveCriterion(c.ctx, s.Actor, id, n, ifVersion); err != nil {
						return err
					}
				default:
					return &usageError{msg: fmt.Sprintf("unknown accept action %q", a[1]), usage: cmd.UseLine()}
				}
				return c.printItem(s, it, fmt.Sprintf("%s acceptance %d/%d checked (%s)", it.ID, it.CheckedCriteria(), len(it.Acceptance), it.Status))
			})
		},
	}
	cmd.Flags().IntVar(&ifVersion, "if-version", 0, "fail with exit 4 unless the item is at this version")
	return cmd
}

func (c *cli) noteCmd() *cobra.Command {
	return &cobra.Command{
		Use: "note <id> <text>", Short: "Add a note to the item's history", Args: args(2, 2),
		RunE: func(_ *cobra.Command, a []string) error {
			text, err := c.readText(a[1])
			if err != nil {
				return err
			}
			return c.with(func(s *Session) error {
				id, err := resolveID(s, c, a[0])
				if err != nil {
					return err
				}
				if err := s.Service.Note(c.ctx, s.Actor, id, text); err != nil {
					return err
				}
				it, err := s.Service.Get(c.ctx, id)
				if err != nil {
					return err
				}
				return c.printItem(s, it, fmt.Sprintf("note added to %s", id))
			})
		},
	}
}

func (c *cli) rmCmd() *cobra.Command {
	var recursive bool
	cmd := &cobra.Command{
		Use: "rm <id>", Short: "Delete an item", Args: args(1, 1),
		RunE: func(_ *cobra.Command, a []string) error {
			return c.with(func(s *Session) error {
				id, err := resolveID(s, c, a[0])
				if err != nil {
					return err
				}
				removed, err := s.Service.Remove(c.ctx, s.Actor, id, recursive)
				if err != nil {
					return err
				}
				switch {
				case c.g.json:
					return printJSON(c.app.Stdout, map[string]any{"removed": removed})
				case c.g.quiet:
					fmt.Fprintln(c.app.Stdout, strings.Join(removed, "\n"))
				default:
					fmt.Fprintf(c.app.Stdout, "✓ removed %s\n", strings.Join(removed, ", "))
				}
				return nil
			})
		},
	}
	cmd.Flags().BoolVarP(&recursive, "recursive", "r", false, "also delete every descendant")
	return cmd
}
