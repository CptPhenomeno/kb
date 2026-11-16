package store

import (
	"github.com/CptPhenomeno/kb/internal/core"
)

// ListTags returns every tag with the number of cards using it, by name.
func (tx *Tx) ListTags() ([]core.Tag, error) {
	rows, err := tx.query(`SELECT t.name, t.color, count(ct.item)
		FROM tags t LEFT JOIN card_tags ct ON ct.tag = t.id
		GROUP BY t.id ORDER BY t.key, t.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []core.Tag
	for rows.Next() {
		var t core.Tag
		if err := rows.Scan(&t.Name, &t.Color, &t.Cards); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// FindTag looks a tag up by name, ignoring case with core.TagKey.
func (tx *Tx) FindTag(name string) (core.Tag, bool, error) {
	var t core.Tag
	err := tx.queryRow(`SELECT t.name, t.color, (SELECT count(*) FROM card_tags WHERE tag = t.id)
		FROM tags t WHERE t.key = ?`, core.TagKey(name)).Scan(&t.Name, &t.Color, &t.Cards)
	if isNoRows(err) {
		return core.Tag{}, false, nil
	}
	return t, err == nil, err
}

// LastTagColor returns the color of the most recently created tag.
func (tx *Tx) LastTagColor() (core.TagColor, error) {
	var c core.TagColor
	err := tx.queryRow(`SELECT color FROM tags ORDER BY id DESC LIMIT 1`).Scan(&c)
	if isNoRows(err) {
		return "", nil
	}
	return c, err
}

// InsertTag stores a new tag. It does not check for an existing name with
// another case: the caller (core.Service.AddTag) does, and the unique key
// column rejects a duplicate that slips through with a plain database error.
func (tx *Tx) InsertTag(t *core.Tag) error {
	_, err := tx.exec(`INSERT INTO tags(name, key, color) VALUES (?, ?, ?)`, t.Name, core.TagKey(t.Name), string(t.Color))
	if err != nil {
		return err
	}
	return nil
}

// loadTags fills the Tags of items from card_tags, in card order.
func (tx *Tx) loadTags(items []*core.Item) error {
	if len(items) == 0 {
		return nil
	}
	byID := make(map[string]*core.Item, len(items))
	for _, it := range items {
		byID[it.ID] = it
		it.Tags = nil
	}
	rows, err := tx.query(`SELECT ct.item, t.name FROM card_tags ct JOIN tags t ON t.id = ct.tag
		ORDER BY ct.item, ct.position`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id, name string
		if err := rows.Scan(&id, &name); err != nil {
			return err
		}
		if it := byID[id]; it != nil {
			it.Tags = append(it.Tags, name)
		}
	}
	return rows.Err()
}

// setCardTags replaces the tags of a card with names, in order. A name that
// matches no tag creates it with the color after the most recently created
// one, the rotation core.Service.AddTag uses.
func (tx *Tx) setCardTags(item string, names []string) error {
	if _, err := tx.exec(`DELETE FROM card_tags WHERE item = ?`, item); err != nil {
		return err
	}
	for pos, name := range names {
		var id int64
		err := tx.queryRow(`SELECT id FROM tags WHERE key = ?`, core.TagKey(name)).Scan(&id)
		if isNoRows(err) {
			last, lerr := tx.LastTagColor()
			if lerr != nil {
				return lerr
			}
			res, ierr := tx.exec(`INSERT INTO tags(name, key, color) VALUES (?, ?, ?)`,
				name, core.TagKey(name), string(core.NextTagColor(last)))
			if ierr != nil {
				return ierr
			}
			id, err = res.LastInsertId()
		}
		if err != nil {
			return err
		}
		if _, err := tx.exec(`INSERT OR IGNORE INTO card_tags(item, tag, position) VALUES (?, ?, ?)`, item, id, pos); err != nil {
			return err
		}
	}
	return nil
}

// UpdateTag sets the name, key and color of the tag named name (ignoring
// case). The cards keep using it: card_tags points at the tag id.
func (tx *Tx) UpdateTag(name string, t core.Tag) error {
	_, err := tx.exec(`UPDATE tags SET name = ?, key = ?, color = ? WHERE key = ?`,
		t.Name, core.TagKey(t.Name), string(t.Color), core.TagKey(name))
	return err
}

// DeleteTag removes the tag; its card_tags rows go with it (cascade).
func (tx *Tx) DeleteTag(name string) error {
	_, err := tx.exec(`DELETE FROM tags WHERE key = ?`, core.TagKey(name))
	return err
}

// CardsWithTag lists the IDs of the cards using the tag.
func (tx *Tx) CardsWithTag(name string) ([]string, error) {
	rows, err := tx.query(`SELECT ct.item FROM card_tags ct JOIN tags t ON t.id = ct.tag
		WHERE t.key = ? ORDER BY ct.item`, core.TagKey(name))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}
