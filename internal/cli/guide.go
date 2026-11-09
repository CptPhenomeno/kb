package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// agentGuide is printed by `kb guide`, to be pasted into CLAUDE.md or
// AGENTS.md so that coding agents use the board correctly.
const agentGuide = `# Working with the kb board

This project tracks its work on a kb kanban board. Use the kb CLI for every
change to the board; never edit .kanban/ files by hand.

## Identity

Every sub agent sets its own name before running kb:

    export KB_ACTOR=agent-<topic>      # e.g. agent-auth

## Planning (after SPEC.md is written)

Write the plan as YAML and import it in one shot, all or nothing:

    kb import plan.yaml --dry-run      # validate and preview
    kb import plan.yaml                # create epics, stories, tasks, tests
    kb import plan.yaml --update       # re-import after the spec changes

## Working loop (one task at a time)

    kb next --claim --json             # take the best available task or bug
    kb phase <id> red                  # write the failing tests first
    kb test <id> fail <n|name>         # record test outcomes
    kb phase <id> green                # make them pass
    kb test <id> pass <n|name>...
    kb phase <id> refactor
    kb accept <id> check <n>           # check acceptance criteria
    kb note <id> "what changed and why"
    kb move <id> review                # only allowed when every test passes

Other commands: kb show <id>, kb list --json, kb tree, kb touch <id> (renew
your claim during long work), kb release <id> (give a task back),
kb add subtask "<title>" --parent <id>.

## Rules

- A move is refused unless the target column's requirements are met; the
  error lists every unmet requirement. Fix them, do not work around them.
- Claims expire after a period without writes: write often or kb touch.
- Use --json for anything you parse: the format is stable.

## Exit codes

Every command sets an exit code you can branch on:

  0 ok
  1 error or wrong usage
  2 not found (also: kb next found nothing available)
  3 requirements or rules not met
  4 version conflict (re-read the item, then retry)
  5 claimed by another actor
`

func (c *cli) guideCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "guide",
		Short: "Print the usage guide for coding agents (paste it into CLAUDE.md or AGENTS.md)",
		Args:  args(0, 0),
		RunE: func(*cobra.Command, []string) error {
			_, err := fmt.Fprint(c.app.Stdout, agentGuide)
			return err
		},
	}
}
