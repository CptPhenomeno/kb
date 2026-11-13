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

## Plan format

Children nest under their parent: epic > story > task or bug > subtask.
Refs are stable keys: links may point to refs defined later in the file,
and --update matches items by ref.

    spec: SPEC.md                      # default spec of every item
    status: todo                       # default initial status
    items:
      - ref: auth
        type: epic
        title: Authentication
        children:
          - ref: auth-login
            type: story
            title: Token login
            description: |
              Users sign in with a token instead of a password.
              Done when a valid token opens a session.
            acceptance:
              - expired token -> 401
            children:
              - ref: auth-validate
                type: task
                title: Validate token in middleware
                priority: high
                points: 3                # 1, 2, 3, 5, 8 or 13
                tests:
                  - rejects expired token
                blocks: [auth-refresh]
              - ref: auth-refresh
                type: task
                title: Token refresh
                points: 2

## Writing items

Every item needs a description that a teammate can act on without reading
the spec. Write it in three parts:

    <why it exists: the context or the problem>
    <what changes: the scope, in concrete terms>
    Done when <how anyone can verify it is finished>.

Titles name the outcome ("Token refresh"), not the activity ("Work on
tokens"). Test names describe observable behavior ("rejects expired token").

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

kb next shows the best available item without claiming it. Available means
in To do, Doing or Review, not blocked, and (for tasks and bugs) without a
valid claim. The order is: review first, then doing, then to do; then
priority; then the item that unblocks the heaviest chain of story points;
then its own points, its position and its id. --type takes epic, story,
task or bug (default task and bug); subtask exits 3. --claim only claims
tasks and bugs: stories and epics are ignored, exit 3 if --type leaves
neither, and a task in review stays in review.

Other commands:

    kb show <id>                       # the item with its history
    kb list --json                     # filters: --status --type --label --blocked --stale
    kb list --stale                    # items in progress whose claim expired
    kb tree                            # the hierarchy with progress and points
    kb test <id> edit <n> "<name>"     # rename a test; it is pending again
    kb test <id> rm <n>                # remove a test; later ones shift up
    kb accept <id> edit <n> "<text>"   # replace a criterion's text; it is unchecked again
    kb accept <id> rm <n>              # remove a criterion; later ones shift up
    kb touch <id>                      # renew your claim during long work
    kb release <id>                    # give a task back
    kb add subtask "<title>" --parent <id>
    kb log --follow --json             # stream the events as JSON Lines

## Rules

- A move is refused unless the target column's requirements are met; the
  error lists every unmet requirement. Fix them, do not work around them.
- Claims expire after a period without writes: write often or kb touch.
- Use --json for anything you parse: the format is stable.

## Exit codes

Every command sets an exit code you can branch on:

- 0 ok
- 1 error or wrong usage
- 2 not found (also: kb next found nothing available)
- 3 requirements or rules not met
- 4 version conflict (re-read the item, then retry)
- 5 claimed by another actor
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
