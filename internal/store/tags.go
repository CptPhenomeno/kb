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

// InsertTag stores a new tag. A name that exists with any case is a
// core.ErrConflict.
func (tx *Tx) InsertTag(t *core.Tag) error {
	_, err := tx.exec(`INSERT INTO tags(name, key, color) VALUES (?, ?, ?)`, t.Name, core.TagKey(t.Name), string(t.Color))
	if err != nil {
		return err
	}
	return nil
}
