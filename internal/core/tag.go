package core

import (
	"context"
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
		if _, found, err := o.tx.FindTag(name); err != nil {
			return err
		} else if found {
			return wrapf(ErrConflict, "tag %s already exists", name)
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
