-- Initial schema of the kb board.
-- Lists that are always read together with their item (labels, links,
-- tests, acceptance criteria) are stored as JSON arrays.

CREATE TABLE meta (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

CREATE TABLE items (
    id               TEXT PRIMARY KEY,
    ref              TEXT UNIQUE,
    type             TEXT NOT NULL,
    title            TEXT NOT NULL,
    description      TEXT NOT NULL DEFAULT '',
    parent           TEXT REFERENCES items(id),
    status           TEXT NOT NULL,
    priority         TEXT NOT NULL,
    points           INTEGER NOT NULL DEFAULT 0,
    spec             TEXT NOT NULL DEFAULT '',
    labels           TEXT NOT NULL DEFAULT '[]',
    links            TEXT NOT NULL DEFAULT '[]',
    acceptance       TEXT NOT NULL DEFAULT '[]',
    tests            TEXT NOT NULL DEFAULT '[]',
    tdd_phase        TEXT NOT NULL DEFAULT '',
    assignee         TEXT NOT NULL DEFAULT '',
    claim_actor      TEXT,
    claim_expires_at TEXT,
    position         INTEGER NOT NULL,
    version          INTEGER NOT NULL,
    created_at       TEXT NOT NULL,
    created_by       TEXT NOT NULL,
    updated_at       TEXT NOT NULL,
    updated_by       TEXT NOT NULL
);

CREATE INDEX items_parent ON items(parent);
CREATE INDEX items_status_position ON items(status, position);

CREATE TABLE relations (
    kind    TEXT NOT NULL,
    from_id TEXT NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    to_id   TEXT NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    PRIMARY KEY (kind, from_id, to_id)
);

CREATE TABLE events (
    id    INTEGER PRIMARY KEY AUTOINCREMENT,
    at    TEXT NOT NULL,
    actor TEXT NOT NULL,
    item  TEXT NOT NULL,
    kind  TEXT NOT NULL,
    data  TEXT NOT NULL DEFAULT '{}'
);

CREATE INDEX events_item ON events(item, id);
