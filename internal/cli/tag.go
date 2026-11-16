package cli

import (
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/CptPhenomeno/kb/internal/core"
)

func (c *cli) tagCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tag",
		Short: "Manage the tags of the board",
		Args:  unknownCommand,
		RunE:  func(cmd *cobra.Command, _ []string) error { return cmd.Help() },
	}
	cmd.AddCommand(c.tagAddCmd(), c.tagListCmd())
	return cmd
}

func (c *cli) tagAddCmd() *cobra.Command {
	var color string
	cmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Create a tag (the color follows the rotation unless --color is given)",
		Args:  args(1, 1),
		RunE: func(_ *cobra.Command, a []string) error {
			return c.with(func(s *Session) error {
				t, err := s.Service.AddTag(c.ctx, s.Actor, a[0], color)
				if err != nil {
					return err
				}
				switch {
				case c.g.json:
					return printJSON(c.app.Stdout, t)
				case c.g.quiet:
					fmt.Fprintln(c.app.Stdout, t.Name)
				default:
					fmt.Fprintf(c.app.Stdout, "✓ tag %s (%s)\n", t.Name, t.Color)
				}
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&color, "color", "", "tag color: red, orange, yellow, green, cyan, blue, purple or pink")
	return cmd
}

func (c *cli) tagListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List the tags with their color and card count, by name",
		Args:  args(0, 0),
		RunE: func(*cobra.Command, []string) error {
			return c.with(func(s *Session) error {
				tags, err := s.Service.Tags(c.ctx)
				if err != nil {
					return err
				}
				if c.g.json {
					if tags == nil {
						tags = []core.Tag{}
					}
					return printJSON(c.app.Stdout, tags)
				}
				w := tabwriter.NewWriter(c.app.Stdout, 0, 0, 2, ' ', 0)
				for _, t := range tags {
					if c.g.quiet {
						fmt.Fprintln(c.app.Stdout, t.Name)
						continue
					}
					fmt.Fprintf(w, "%s\t%s\t%d\n", t.Name, t.Color, t.Cards)
				}
				return w.Flush()
			})
		},
	}
}
