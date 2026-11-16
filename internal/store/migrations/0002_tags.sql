-- Tags are stored once and reused by cards. This migration only creates the
-- schema; moving the existing labels into tags is a later step.
--
-- tags.id grows with creation order. tags.key is the case-folded name
-- (core.TagKey), computed in Go so that comparison follows Unicode folding
-- and not only SQLite's ASCII NOCASE; name keeps the original spelling.

CREATE TABLE tags (
    id    INTEGER PRIMARY KEY AUTOINCREMENT,
    name  TEXT NOT NULL,
    key   TEXT NOT NULL UNIQUE,
    color TEXT NOT NULL
);

CREATE TABLE card_tags (
    item     TEXT    NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    tag      INTEGER NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    position INTEGER NOT NULL,
    PRIMARY KEY (item, tag)
);

CREATE INDEX card_tags_tag ON card_tags(tag);
