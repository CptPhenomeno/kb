# kb JSON output

Every command accepts `--json`. The shapes below are a stable contract for
scripts and coding agents: fields are never renamed or removed within a
major version. New fields may be added, so ignore keys you do not know.

## Item

Printed by `show`, `add`, `edit`, `claim`, `release`, `touch`, `next`,
`phase`, `test`, `accept`, `note`; `list` and `tree` print an array of items.

```json
{
  "id": "KB-3",
  "ref": "auth-login-validate",
  "type": "task",
  "title": "Validate token in middleware",
  "description": "markdown",
  "parent": "KB-2",
  "status": "doing",
  "priority": "high",
  "points": 3,
  "spec": "SPEC.md#authentication",
  "tags": ["api"],
  "links": ["PR #148"],
  "acceptance": [{"text": "invalid token never reaches a handler", "done": false}],
  "tests": [{"name": "rejects expired token", "state": "failing"}],
  "tdd_phase": "red",
  "assignee": "agent-auth",
  "claim": {"actor": "agent-auth", "expires_at": "2026-11-09T14:17:00Z", "stale": false},
  "blocked_by": [],
  "blocks": ["KB-4"],
  "relates": [],
  "duplicates": [],
  "position": 0,
  "version": 5,
  "created_at": "2026-11-09T14:00:00Z",
  "created_by": "@dev",
  "updated_at": "2026-11-09T14:02:00Z",
  "updated_by": "agent-auth",
  "history": []
}
```

| Field | Values |
|-------|--------|
| `type` | `epic`, `story`, `task`, `subtask`, `bug` |
| `status` | `backlog`, `todo`, `doing`, `review`, `done` |
| `priority` | `high`, `medium`, `low` |
| `points` | `1 2 3 5 8 13` or `null`; always `null` for epics, stories and subtasks, which can only show derived points |
| `tags` | the names of the item's tags, in the order of the card (never `null`); the colors are in `kb tag list --json` |
| `tdd_phase` | `red`, `green`, `refactor` or `null` |
| `tests[].state` | `pending`, `failing`, `passing` |
| `ref`, `parent`, `assignee`, `claim` | `null` when unset |
| `blocked_by` | open blockers of the item or of one of its ancestors |
| `claim.stale` | `true` when the item is in `doing` and its claim expired (an agent went silent) |
| `history` | only in `show`: the item's events, oldest first |

Timestamps are RFC 3339 in UTC. Lists are never `null`.

Points are an own value of tasks and bugs only; setting them on another type
exits 3 (`rule`). The points of an epic or a story shown by `kb show`, `kb
tree`, the TUI and the points sort are derived: the sum of the points of its
descendant tasks and bugs. They are not a field of the item, so `points`
stays `null` in the JSON.

Before 1.3 the item had a `labels` field. It is now `tags` and `labels` no
longer appears in the output. `--label` and the `labels` field of `plan.yaml`
are still accepted as deprecated aliases of `--tag` and `tags`; the warning
goes to stderr, so the JSON on stdout stays valid.

## Move

`kb move <id> <status> --json`:

```json
{"id": "KB-4", "from": "doing", "to": "review", "position": 0, "version": 7,
 "derived": [{"id": "KB-2", "from": "doing", "to": "review"}]}
```

`derived` lists the parents that moved automatically with the item.

## Other commands

| Command | Output |
|---------|--------|
| `link`, `unlink` | `{"ok": true, "from": "KB-3", "kind": "blocks", "to": "KB-4"}` |
| `rm` | `{"removed": ["KB-7", "KB-6"]}` (deletion order) |
| `log` | JSON Lines: one event per line |

## Tag

Printed by `kb tag add`, `color`, `rename` and `delete`; `kb tag list` prints
an array, by name, `[]` when the board has no tags.

```json
{"name": "api", "color": "red", "cards": 2}
```

`cards` is the number of cards that carry the tag; after `delete` it is the
number of cards the tag was removed from. `color` is one of
red, orange, yellow, green, cyan, blue, purple, pink. A tag name has at most
24 characters and no comma, and is compared ignoring case.

| Command | Does | Exit codes |
|---------|------|------------|
| `kb tag list` | all tags with color and card count | 0 |
| `kb tag add <name> [--color C]` | creates the tag; without `--color` the color follows the rotation | 3 (`rule`) invalid name or color; 4 (`conflict`) the tag exists |
| `kb tag color <name> <color>` | changes the color on every card | 2 (`not_found`); 3 (`rule`) unknown color |
| `kb tag rename <old> <new>` | renames the tag on every card, also only to change the case | 2 (`not_found`); 3 (`rule`) invalid name; 4 (`conflict`) another tag has that name |
| `kb tag delete <name>` | deletes the tag and removes it from every card | 2 (`not_found`) |

With `-q` the tag commands print only the tag name, one per line.

## Event

```json
{"id": 14, "at": "2026-11-09T14:02:00Z", "actor": "agent-auth", "item": "KB-3",
 "kind": "test", "data": {"name": "rejects expired token", "state": "failing"}}
```

`kind` is one of `created`, `updated`, `moved`, `claimed`, `released`,
`claim_expired`, `linked`, `unlinked`, `phase`, `test`, `acceptance`,
`note`, `deleted`. Moves made by propagation carry `"derived": true`.

The `data` of a `test` event describes one test:

| Fields | Meaning |
|--------|---------|
| `name`, `state` | the test after an outcome is recorded |
| `"added": true` | `test <id> add`: the test was appended |
| `"edited": true`, `"old"` | `test <id> edit`: `name` is the new name, `old` the previous one; `state` is `pending` because a rename resets the outcome |
| `"removed": true` | `test <id> rm`: `name` and `state` are those of the removed test; later tests shift up |

The `data` of an `acceptance` event describes one criterion (`n` is its
1-based position):

| Fields | Meaning |
|--------|---------|
| `text`, `done` | the criterion after a check or uncheck |
| `"added": true` | `accept <id> add`: the criterion was appended |
| `"edited": true`, `"old"` | `accept <id> edit`: `text` is the new text, `old` the previous one; `done` is `false` because an edit unchecks |
| `"removed": true` | `accept <id> rm`: `text` and `done` are those of the removed criterion; later criteria shift up |

Criterion text that is blank, or a duplicate of another criterion of the
same item, exits 3 (`rule`); a missing `n` exits 2 (`not_found`).

## Next

`kb next [--type epic,story,task,bug] [--parent ID] [--tag T] [--claim]`
prints one item (shape above). Available means in todo, doing or review,
not blocked, and (for tasks and bugs)
without a valid claim, even your own. The order is review, then doing, then todo; then priority; then the heaviest chain of
story points the item blocks; then its own points, its position and its
id. `--type` defaults to task and bug; `subtask` exits 3 (`rule`). With
`--claim` only tasks and bugs are considered: stories and epics are
ignored, exit 3 (`rule`) if `--type` leaves neither, and a task in review
stays in review. Exit 2 (`not_found`) means nothing is available.

## Errors

With `--json`, errors are printed on stdout and the exit code is set:

```json
{"error": "requirements", "message": "cannot move KB-2 to done",
 "unmet": ["child KB-4 is doing", "acceptance 0/2 checked"]}
```

| `error` | Exit code | Meaning |
|---------|-----------|---------|
| `error` | 1 | generic error or wrong usage |
| `not_found` | 2 | item, test or relation not found; `next` found nothing |
| `requirements` | 3 | move or claim refused; `unmet` lists every missing condition |
| `rule` | 3 | other rule violated (hierarchy, cycle, duplicate ref) |
| `conflict` | 4 | `--if-version` did not match; a tag with that name already exists |
| `claimed` | 5 | another actor holds a valid claim |

The `unmet` messages are stable and can be matched by agents.
