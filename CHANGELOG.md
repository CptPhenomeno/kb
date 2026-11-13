# Changelog

All notable changes to kb are listed here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and versions follow
[Semantic Versioning](https://semver.org/).

## [1.2.1] - 2026-11-13

`kb next` picks work in a more useful order and accepts every item type.

### Fixed

- `kb next --type` now accepts `epic` and `story` as well as `task` and
  `bug`; `subtask` is rejected with exit code 3.

### Changed

- `kb next` orders candidates by status (review, then doing, then todo),
  then priority, then the item that unblocks the heaviest chain of story
  points, then points, then position, then id.
- Review tasks without a valid claim are now offered by `kb next`.
- `kb next --claim` ignores stories and epics, and fails when no claimable
  type is left.
- `kb next` cost now grows linearly with the size of the board.

## [1.2.0] - 2026-11-12

Tests become as manageable as criteria from the CLI and the TUI, the create
form is split into tabs, the Tree opens with done branches closed, the
command bar is compact by default, and the theme list grows to eighteen
with a new default.

### Added

- `kb test <id> edit` and `kb test <id> rm` change the text of a test or
  remove it; `kb log`, `kb show` and the Activity view describe edited and
  removed tests.
- The criteria popup (`c`) is now titled "Criteria and tests" and has a
  Criteria tab and a Tests tab. On the Tests tab `space` cycles the state
  of the selected test, and `n`, `e` and `d` add, rename and delete tests.
- The create form is split into Details, Criteria and Tests tabs, so tests
  can be added when an item is created. Switch tab with `ctrl+←`/`ctrl+→`;
  `ctrl+s` saves from any tab. Long lists scroll inside the tab and the
  form fits an 80x24 terminal.
- In the Tree, `x` opens or closes every branch.
- The command palette has "Show full command bar" to switch between the
  compact and the full command bar; the choice is saved as `command_bar` in
  `config.toml`.
- Four kb themes: `kb-neon-rain`, `kb-synth-sunset`, `kb-chrome-haze` and
  `kb-holo-dawn`.
- Six more themes: `rose-pine`, `rose-pine-dawn`, `solarized-light`,
  `tokyo-night-day`, `dracula-alucard` and `nord-light`. There are now nine
  dark and nine light themes.

### Changed

- The default theme is now `kb-neon-rain`. If you have not saved a theme
  you will see it on first run; a theme saved in `config.toml` or set with
  `KB_THEME` is still honoured.
- The Board, the Tree and the Activity view show a compact command bar by
  default; the full one is one palette entry away.
- In the Tree, done items with children start closed, and a search or a
  filter opens every branch as soon as it becomes active.
- Board columns use the rows left over to show the next card cut off,
  instead of empty lines under the "more" marker.
- The theme picker groups themes under Dark and Light headings and scrolls
  on a small terminal, keeping the selection visible.
- Every theme now passes contrast checks for text and indicators on its
  background and surface colors, so some colors were adjusted keeping their
  hue. The human, agent and warning colors are distinct on every theme.

### Fixed

- An unknown command is reported as unknown, with the closest command as a
  suggestion, instead of a generic "wrong number of arguments" error.

## [1.1.0] - 2026-11-11

Acceptance criteria are fully manageable from the CLI and the TUI, the form
keeps long text readable, the item detail opens in a large popup and the
help is split into tabs.

### Added

- `kb accept <id> edit <n> "<text>"` replaces the text of a criterion and
  unchecks it, since the new text has to be verified again; a story that
  was done steps back.
- `kb accept <id> rm <n>` removes a criterion and the following ones shift
  up; removing the last unchecked criterion can close the story.
- `kb log`, `kb show` and the Activity view describe edited and removed
  criteria; `kb guide` and `docs/cli-json.md` document the new commands and
  the fields of the `acceptance` event.
- The criteria popup (`c`) adds a criterion with `n`, edits the selected
  one with `e` and deletes it with `d` after a `y`/`n` confirmation; the
  input line wraps in the popup.
- The create form has an `acceptance` field: `enter` adds a criterion,
  `backspace` on an empty input removes the last one, and text still in
  the input is added when the form is saved.
- `ctrl+s` saves the form from any field.

### Changed

- The item detail opens with `enter` in a centered popup at 75% of the
  screen instead of a side panel, so the Board and the Tree keep their full
  width. It scrolls with the arrows, `pgup`/`pgdown` and the mouse wheel;
  `e`, `ctrl+e`, `c`, `a` and `l` act on the item from the popup, a click
  outside closes it, and it closes with a notice if the item is deleted.
- In the form, the title and the description wrap in the popup width; the
  description shows up to six lines, the last ones with a leading `…` when
  longer, and the cursor stays visible.
- `enter` on the description opens `$EDITOR`; on the other fields it still
  saves.
- The help (`?`) is split into tabs: the current view, Detail, Criteria and
  Form. `tab` and the arrows switch tab, long tabs scroll, and every tab
  reads in full on an 80x24 terminal.
- With the detail popup or the help open, keys that do not apply there do
  nothing, `q` included; `ctrl+c` still quits.
- The `c` key and its palette action are now "Manage acceptance criteria".

## [1.0.2] - 2026-11-10

The release is automated: pushing a `v*` tag checks the code, builds the
static binaries and publishes them on GitHub with the checksums and the
notes taken from this changelog.

### Added

- A release pipeline on a `v*` tag: check, cross builds, `checksums.txt`
  and a GitHub release whose notes are the section of this changelog.
- A check on every push to the `develop` branch.

### Changed

- `make cross` names the binaries `kb-<version>-<os>-<arch>`, so the
  release assets are versioned without extra steps.
- The setup and the check steps are shared by the check and the release
  workflows through one composite action.

## [1.0.1] - 2026-11-10

The same code as 1.0.0, published from a cleaned-up repository.

### Changed

- The repository holds only the source, its documentation and the MIT
  licence.
- The README opens with screenshots of the Board view in a dark and a light
  theme.
- 1.0.0 is retracted: `go install` and `go get` pick 1.0.1 or later.

## [1.0.0] - 2026-11-10 [YANKED]

The first release: the core, the CLI for coding agents and the TUI for the
developer. MCP support is planned after v1.

### Board and rules

- One board per project in SQLite (`.kanban/kb.db`, WAL, immediate
  transactions), shared by every git worktree of the repository.
- Five columns (Backlog, To do, In progress, Review, Done) and a fixed
  hierarchy: epic > story > task or bug > subtask.
- Entry requirements for every column: a move is refused with the full list
  of what is missing (open children, failing or pending tests, blockers,
  unchecked acceptance criteria).
- Parents follow their children automatically, advancing and regressing
  within their own requirements.
- Relations `blocks`, `relates` and `duplicates`, with `blocks` cycles
  rejected and blocking inherited from ancestors.
- Claims with a configurable TTL (`claim_ttl`, default 15 minutes), renewed
  by any write of the holder; an expired claim on work in progress is
  flagged as stale.
- Fibonacci story points, TDD phase (red, green, refactor), named tests and
  acceptance criteria on items.
- Optimistic versions on every write and an append-only event log.

### CLI

- Commands: `init`, `add`, `show`, `list`, `tree`, `edit` (flags or YAML in
  `$EDITOR`), `move`, `link`, `unlink`, `next`, `claim`, `release`, `touch`,
  `phase`, `test`, `accept`, `note`, `rm`, `log` (with `--follow`), `export`,
  `import`, `guide` and `version`.
- `kb next --claim` hands out the best available task atomically, also
  across processes.
- Stable `--json` output (see `docs/cli-json.md`) and exit codes: 0 ok,
  1 error, 2 not found, 3 requirements not met, 4 version conflict,
  5 claimed by another actor.
- `kb import` creates or, with `--update`, revises a whole YAML plan in one
  transaction, resolving refs; `--dry-run` previews it.
- `kb guide` prints the agent instructions, ready for CLAUDE.md or AGENTS.md.
- `kb export` writes a readable `.kanban/board.md`.

### TUI

- Board, Tree and Activity views, refreshed live within a second of any
  write from another process.
- Board: cards with type, priority, points, TDD phase, tests, assignee,
  BLOCKED and STALE badges; a detail panel with markdown description,
  children, tests, criteria, links and history, scrollable with pgup/pgdown.
- Move with `shift+←/→` (an alert explains refused moves), reorder with
  `shift+↑/↓`, sort the columns by date, ID or story points with `s`.
- Create and edit items in a form (`ctrl+e` opens `$EDITOR`), delete with a
  confirmation, claim and release, check acceptance criteria, manage links
  and jump to linked items.
- Search with `/`, filter types, blocked or stale items with `f`, show or
  hide epics and stories with `p`, and a command palette on `ctrl+p`.
- Tree: collapsible hierarchy with progress bars and summed points; `n`
  adds a child of the suggested type.
- Activity: each actor with its item, phase, tests and last write beside
  the event stream; release a stale claim with `a`.
- Eight themes (Tokyo Night, Catppuccin Mocha and Latte, Gruvbox dark and
  light, Dracula, Nord, Solarized dark) with a live-preview picker, painted
  opaque for translucent terminals. Arrow keys only, no vim bindings.
- Personal preferences (theme, epics and stories, sort order) saved in
  `$XDG_CONFIG_HOME/kb/config.toml`.

### Build

- Static binaries without cgo; `make cross` builds linux and darwin for
  amd64 and arm64. `kb version` reports the git version, or the module
  version when installed with `go install`.
