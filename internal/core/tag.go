package core

import (
	"context"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

// TagColor is the name of a tag color. Each theme paints the names with
// its own values; the core only knows the names and their order.
type TagColor string

// TagColors are the allowed colors, in rotation order.
var TagColors = []TagColor{"red", "orange", "yellow", "green", "cyan", "blue", "purple", "pink"}

// MaxTagNameLen is the longest tag name, in characters.
const MaxTagNameLen = 24

// Tag is a named, colored tag together with the number of cards using it.
type Tag struct {
	Name  string   `json:"name"`
	Color TagColor `json:"color"`
	Cards int      `json:"cards"`
}

// ParseTagColor returns the color named s (exact, lower case) or a rule
// error listing the allowed colors.
func ParseTagColor(s string) (TagColor, error) {
	for _, c := range TagColors {
		if string(c) == s {
			return c, nil
		}
	}
	names := make([]string, len(TagColors))
	for i, c := range TagColors {
		names[i] = string(c)
	}
	return "", NewRuleError("unknown tag color " + quote(s) + ": use " + strings.Join(names, ", "))
}

// NextTagColor is the color following last in the rotation; an empty last
// (no tag yet) gives the first color.
func NextTagColor(last TagColor) TagColor {
	for i, c := range TagColors {
		if c == last {
			return TagColors[(i+1)%len(TagColors)]
		}
	}
	return TagColors[0]
}

// ValidateTagName trims name and checks it: not empty, no comma, at most
// MaxTagNameLen characters. It returns the trimmed name.
func ValidateTagName(name string) (string, error) {
	name = strings.TrimSpace(name)
	switch {
	case name == "":
		return "", NewRuleError("tag name cannot be empty")
	case strings.Contains(name, ","):
		return "", NewRuleError("tag name cannot contain a comma")
	case utf8.RuneCountInString(name) > MaxTagNameLen:
		return "", NewRuleError("tag name is longer than 24 characters")
	}
	return name, nil
}

// TagKey is the case-insensitive identity of a tag name: two names are the
// same tag exactly when their keys are equal. It follows Unicode simple
// folding (the semantics of strings.EqualFold), so every layer, including
// the storage, agrees on what "the same tag" means.
func TagKey(name string) string {
	var b strings.Builder
	for _, r := range name {
		min := r
		for f := unicode.SimpleFold(r); f != r; f = unicode.SimpleFold(f) {
			if f < min {
				min = f
			}
		}
		b.WriteRune(min)
	}
	return b.String()
}

func quote(s string) string { return "\"" + s + "\"" }

// ---- service ----

// Tags returns every tag with its card count, sorted by name.
func (s *Service) Tags(ctx context.Context) ([]Tag, error) {
	var out []Tag
	err := s.Store.Transact(ctx, func(tx Tx) (err error) { out, err = tx.ListTags(); return err })
	return out, err
}

// AddTag creates a tag. An empty color takes the one after the color of
// the most recently created tag. A name that exists with any case is a
// conflict.
func (s *Service) AddTag(ctx context.Context, actor, name, color string) (*Tag, error) {
	name, err := ValidateTagName(name)
	if err != nil {
		return nil, err
	}
	var c TagColor
	if color != "" {
		if c, err = ParseTagColor(color); err != nil {
			return nil, err
		}
	}
	var out *Tag
	err = s.write(ctx, actor, func(o *op) error {
		if existing, found, err := o.tx.FindTag(name); err != nil {
			return err
		} else if found {
			return conflictf("tag %s already exists", existing.Name)
		}
		if c == "" {
			last, err := o.tx.LastTagColor()
			if err != nil {
				return err
			}
			c = NextTagColor(last)
		}
		t := Tag{Name: name, Color: c}
		if err := o.tx.InsertTag(&t); err != nil {
			return err
		}
		out = &t
		return nil
	})
	return out, err
}

// canonTags prepares the tags of a card for storing: names are trimmed and
// validated, a repeated name (ignoring case) is kept once in its first
// position, and a name that matches an existing tag takes that tag's
// spelling. Missing tags are not created here: the store creates them in
// the same transaction as the card, with the rotation of AddTag.
func (o *op) canonTags(names []string) ([]string, error) {
	out := make([]string, 0, len(names))
	seen := map[string]bool{}
	for _, n := range names {
		n, err := ValidateTagName(n)
		if err != nil {
			return nil, err
		}
		k := TagKey(n)
		if seen[k] {
			continue
		}
		seen[k] = true
		if t, found, err := o.tx.FindTag(n); err != nil {
			return nil, err
		} else if found {
			n = t.Name
		}
		out = append(out, n)
	}
	return out, nil
}

// hasTag reports whether tags contains name, ignoring case.
func hasTag(tags []string, name string) bool {
	k := TagKey(name)
	return slices.ContainsFunc(tags, func(t string) bool { return TagKey(t) == k })
}

// sameTags reports whether two tag lists name the same tags in the same
// order, ignoring case.
func sameTags(a, b []string) bool {
	return slices.EqualFunc(a, b, func(x, y string) bool { return TagKey(x) == TagKey(y) })
}

// SetTagColor changes the color of a tag, found ignoring case. Cards are
// untouched and get no event: the color is not part of a card.
func (s *Service) SetTagColor(ctx context.Context, actor, name, color string) (*Tag, error) {
	c, err := ParseTagColor(color)
	if err != nil {
		return nil, err
	}
	var out *Tag
	err = s.write(ctx, actor, func(o *op) error {
		t, err := o.mustTag(name)
		if err != nil {
			return err
		}
		t.Color = c
		if err := o.tx.UpdateTag(t.Name, t); err != nil {
			return err
		}
		out = &t
		return nil
	})
	return out, err
}

// RenameTag gives a tag a new name, also when only the case changes. A name
// that is another existing tag is a conflict: tags never merge. Every card
// using the tag gets an updated event with its tags before and after.
func (s *Service) RenameTag(ctx context.Context, actor, oldName, newName string) (*Tag, error) {
	newName, err := ValidateTagName(newName)
	if err != nil {
		return nil, err
	}
	var out *Tag
	err = s.write(ctx, actor, func(o *op) error {
		t, err := o.mustTag(oldName)
		if err != nil {
			return err
		}
		if other, found, err := o.tx.FindTag(newName); err != nil {
			return err
		} else if found && TagKey(other.Name) != TagKey(t.Name) {
			return conflictf("tag %s already exists", other.Name)
		}
		items, err := o.cardsWithTag(t.Name)
		if err != nil {
			return err
		}
		prev := t.Name
		t.Name = newName
		if err := o.tx.UpdateTag(prev, t); err != nil {
			return err
		}
		out = &t
		k := TagKey(prev)
		return o.retag(items, func(tags []string) []string {
			r := slices.Clone(tags)
			for i, n := range r {
				if TagKey(n) == k {
					r[i] = newName
				}
			}
			return r
		}, func() error { return nil })
	})
	return out, err
}

// DeleteTag removes a tag from the board and from every card using it,
// writing an updated event on each of those cards. It returns the deleted
// tag, with the number of cards it was on.
func (s *Service) DeleteTag(ctx context.Context, actor, name string) (*Tag, error) {
	var out *Tag
	err := s.write(ctx, actor, func(o *op) error {
		t, err := o.mustTag(name)
		if err != nil {
			return err
		}
		items, err := o.cardsWithTag(t.Name)
		if err != nil {
			return err
		}
		out = &t
		k := TagKey(t.Name)
		return o.retag(items, func(tags []string) []string {
			return slices.DeleteFunc(slices.Clone(tags), func(n string) bool { return TagKey(n) == k })
		}, func() error { return o.tx.DeleteTag(t.Name) })
	})
	return out, err
}

func (o *op) mustTag(name string) (Tag, error) {
	t, found, err := o.tx.FindTag(strings.TrimSpace(name))
	if err != nil {
		return Tag{}, err
	}
	if !found {
		return Tag{}, wrapf(ErrNotFound, "tag %s", strings.TrimSpace(name))
	}
	return t, nil
}

// cardsWithTag loads the cards using the tag, as they are now.
func (o *op) cardsWithTag(name string) ([]*Item, error) {
	ids, err := o.tx.CardsWithTag(name)
	if err != nil {
		return nil, err
	}
	items := make([]*Item, 0, len(ids))
	for _, id := range ids {
		it, err := o.tx.GetItem(id)
		if err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, nil
}

// retag changes the tags of each card with f and records the change like
// Edit does (version bump, updated_at/by, an updated event with the tags
// before and after). apply runs once, after the cards were read and before
// they are saved, for the change of the tag itself.
func (o *op) retag(items []*Item, f func([]string) []string, apply func() error) error {
	if err := apply(); err != nil {
		return err
	}
	for _, it := range items {
		changes := map[string]any{}
		after := f(it.Tags)
		setList(changes, "tags", &it.Tags, &after)
		if len(changes) == 0 {
			continue
		}
		if err := o.save(it); err != nil {
			return err
		}
		if err := o.event(it.ID, EventUpdated, map[string]any{"changes": changes}); err != nil {
			return err
		}
	}
	return nil
}
