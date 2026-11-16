-- Tags are stored once and reused by cards. This file creates the schema and
-- clears the points of stories (they are derived from their tasks); moving
-- the existing items.labels into tags needs Unicode case folding, so the Go
-- step migrateLabelsToTags runs in the same transaction right after it.
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

-- Stories no longer carry points of their own.
UPDATE items SET points = 0 WHERE type = 'story' AND points <> 0;
