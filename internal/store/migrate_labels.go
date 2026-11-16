package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/CptPhenomeno/kb/internal/core"
)

// migrateLabelsToTags is the Go step of migration 0002: it turns the labels
// of items.labels into tags, trimmed and without empty ones. Labels that differ only in case (core.TagKey)
// become one tag with the spelling met first, scanning items by creation
// order and labels by list order. Colors rotate over the tags in the
// alphabetical order of their keys, starting from the first color. Each card
// keeps its labels, in order and without duplicates, as card_tags. The
// migration is a schema upgrade, so it records no events.
func migrateLabelsToTags(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx, `SELECT id, labels FROM items ORDER BY created_at, id`)
	if err != nil {
		return err
	}
	type card struct {
		id   string
		keys []string
	}
	var cards []card
	spelling := map[string]string{} // key -> first spelling met
	for rows.Next() {
		var id, raw string
		if err := rows.Scan(&id, &raw); err != nil {
			rows.Close()
			return err
		}
		var labels []string
		if err := json.Unmarshal([]byte(raw), &labels); err != nil {
			rows.Close()
			return fmt.Errorf("item %s: bad labels %q: %w", id, raw, err)
		}
		c := card{id: id}
		for _, l := range labels {
			// Surrounding whitespace never distinguishes two tags and an
			// empty label has no name to address: drop it, keep the rest.
			if l = strings.TrimSpace(l); l == "" {
				continue
			}
			k := core.TagKey(l)
			if _, ok := spelling[k]; !ok {
				spelling[k] = l
			}
			if !slices.Contains(c.keys, k) {
				c.keys = append(c.keys, k)
			}
		}
		cards = append(cards, c)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()

	keys := make([]string, 0, len(spelling))
	for k := range spelling {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	ids := make(map[string]int64, len(keys))
	var color core.TagColor
	for _, k := range keys {
		color = core.NextTagColor(color)
		res, err := tx.ExecContext(ctx, `INSERT INTO tags(name, key, color) VALUES (?, ?, ?)`,
			spelling[k], k, string(color))
		if err != nil {
			return err
		}
		if ids[k], err = res.LastInsertId(); err != nil {
			return err
		}
	}
	for _, c := range cards {
		for pos, k := range c.keys {
			if _, err := tx.ExecContext(ctx, `INSERT INTO card_tags(item, tag, position) VALUES (?, ?, ?)`,
				c.id, ids[k], pos); err != nil {
				return err
			}
		}
	}
	return nil
}
