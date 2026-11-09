package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/CptPhenomeno/kb/internal/core"
	"github.com/CptPhenomeno/kb/internal/plan"
)

func (c *cli) importCmd() *cobra.Command {
	var opts core.ImportOptions
	cmd := &cobra.Command{
		Use:   "import <plan.yaml|->",
		Short: "Create (or with --update, update) many items from a YAML plan, all or nothing",
		Args:  args(1, 1),
		RunE: func(_ *cobra.Command, a []string) error {
			var r io.Reader
			name := a[0]
			if name == "-" {
				r, name = c.app.Stdin, "stdin"
			} else {
				f, err := os.Open(name)
				if err != nil {
					return err
				}
				defer f.Close()
				r = f
			}
			p, err := plan.Parse(r, name)
			if err != nil {
				return err
			}
			return c.with(func(s *Session) error {
				rep, err := s.Service.Import(c.ctx, s.Actor, p, opts)
				if err != nil {
					return err
				}
				return c.printImport(rep)
			})
		},
	}
	cmd.Flags().BoolVar(&opts.Update, "update", false, "update items whose ref is already on the board")
	cmd.Flags().BoolVar(&opts.DryRun, "dry-run", false, "validate and report without writing")
	return cmd
}

func (c *cli) printImport(rep *core.ImportReport) error {
	switch {
	case c.g.json:
		items := make([]map[string]any, 0, len(rep.Items))
		for _, r := range rep.Items {
			items = append(items, map[string]any{"ref": r.Ref, "id": r.ID, "result": r.Outcome, "changes": nonNil(r.Changes)})
		}
		return printJSON(c.app.Stdout, map[string]any{"dry_run": rep.DryRun, "created": rep.Created,
			"updated": rep.Updated, "unchanged": rep.Unchanged, "items": items})
	case c.g.quiet:
		for _, r := range rep.Items {
			fmt.Fprintln(c.app.Stdout, r.ID)
		}
		return nil
	}
	t := newTable(c.app.Stdout)
	fmt.Fprintln(t, "ref\tid\tresult\tchanges")
	for _, r := range rep.Items {
		fmt.Fprintf(t, "%s\t%s\t%s\t%s\n", r.Ref, r.ID, r.Outcome, strings.Join(r.Changes, ", "))
	}
	t.Flush()
	summary := fmt.Sprintf("%d created · %d updated · %d unchanged", rep.Created, rep.Updated, rep.Unchanged)
	if rep.DryRun {
		summary = "dry run, nothing written: " + summary
	}
	_, err := fmt.Fprintln(c.app.Stdout, summary)
	return err
}
