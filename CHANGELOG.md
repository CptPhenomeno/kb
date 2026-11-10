# Changelog

All notable changes to kb are listed here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and versions follow
[Semantic Versioning](https://semver.org/).

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

[1.0.1]: https://github.com/CptPhenomeno/kb/releases/tag/v1.0.1
