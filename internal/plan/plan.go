package plan

import (
	"fmt"
	"io"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/CptPhenomeno/kb/internal/core"
)

// fileDoc is the top level of plan.yaml.
type fileDoc struct {
	Spec   *string     `yaml:"spec"`
	Status *string     `yaml:"status"`
	Items  []*fileItem `yaml:"items"`
}

// fileItem is one item of plan.yaml, possibly with nested children.
type fileItem struct {
	Ref         string      `yaml:"ref"`
	Type        string      `yaml:"type"`
	Title       *string     `yaml:"title"`
	Description *string     `yaml:"description"`
	Priority    *string     `yaml:"priority"`
	Points      *int        `yaml:"points"`
	Spec        *string     `yaml:"spec"`
	Status      *string     `yaml:"status"`
	Parent      string      `yaml:"parent"`
	Labels      *[]string   `yaml:"labels"`
	Links       *[]string   `yaml:"links"`
	Acceptance  *[]string   `yaml:"acceptance"`
	Tests       *[]string   `yaml:"tests"`
	Blocks      []string    `yaml:"blocks"`
	Relates     []string    `yaml:"relates"`
	Duplicates  []string    `yaml:"duplicates"`
	Children    []*fileItem `yaml:"children"`

	line int
}

var itemKeys = map[string]bool{
	"ref": true, "type": true, "title": true, "description": true, "priority": true, "points": true,
	"spec": true, "status": true, "parent": true, "labels": true, "links": true, "acceptance": true,
	"tests": true, "blocks": true, "relates": true, "duplicates": true, "children": true,
}

// UnmarshalYAML decodes an item, rejecting unknown keys (strict decoding
// does not reach custom unmarshalers) and remembering the item's line.
func (f *fileItem) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind == yaml.MappingNode {
		for i := 0; i+1 < len(n.Content); i += 2 {
			if k := n.Content[i]; !itemKeys[k.Value] {
				return fmt.Errorf("line %d: unknown field %q", k.Line, k.Value)
			}
		}
	}
	type plain fileItem
	if err := n.Decode((*plain)(f)); err != nil {
		return err
	}
	f.line = n.Line
	return nil
}

// Parse reads a plan. name is used in positions ("plan.yaml:12"). A
// malformed document is a plain error; invalid values are collected into a
// single core.RuleError listing every problem with its position.
func Parse(r io.Reader, name string) (*core.ImportPlan, error) {
	dec := yaml.NewDecoder(r)
	dec.KnownFields(true)
	var doc fileDoc
	if err := dec.Decode(&doc); err != nil {
		if err == io.EOF {
			return nil, fmt.Errorf("%s: empty plan", name)
		}
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	c := converter{name: name}
	if doc.Status != nil {
		st, err := core.ParseStatus(*doc.Status)
		if err != nil {
			c.problem(fmt.Sprintf("%s: default status: %v", name, err))
		}
		c.status = &st
	}
	c.spec = doc.Spec
	for _, it := range doc.Items {
		c.convert(it, "", false)
	}
	if len(c.problems) > 0 {
		return nil, core.NewRuleError(fmt.Sprintf("%s has %d problems", name, len(c.problems)), c.problems...)
	}
	return &core.ImportPlan{Items: c.items}, nil
}

type converter struct {
	name     string
	spec     *string
	status   *core.Status
	items    []*core.ImportItem
	problems []string
}

func (c *converter) problem(msg string) { c.problems = append(c.problems, msg) }

func (c *converter) convert(f *fileItem, parentRef string, nested bool) {
	pos := fmt.Sprintf("%s:%d", c.name, f.line)
	label := pos
	if f.Ref != "" {
		label += " (" + f.Ref + ")"
	}
	it := &core.ImportItem{Ref: strings.TrimSpace(f.Ref), Pos: pos, ParentRef: parentRef,
		Title: f.Title, Description: f.Description, Points: f.Points, Labels: f.Labels, Links: f.Links,
		Acceptance: f.Acceptance, Tests: f.Tests, Blocks: f.Blocks, Relates: f.Relates, Duplicates: f.Duplicates,
		Spec: f.Spec, Status: c.status}
	if it.Ref == "" {
		c.problem(label + ": ref is required")
	}
	if f.Type != "" {
		t, err := core.ParseType(f.Type)
		if err != nil {
			c.problem(fmt.Sprintf("%s: %v", label, err))
		}
		it.Type = &t
	}
	if f.Priority != nil {
		p, err := core.ParsePriority(*f.Priority)
		if err != nil {
			c.problem(fmt.Sprintf("%s: %v", label, err))
		}
		it.Priority = &p
	}
	if f.Status != nil {
		st, err := core.ParseStatus(*f.Status)
		if err != nil {
			c.problem(fmt.Sprintf("%s: %v", label, err))
		}
		it.Status = &st
	}
	if it.Spec == nil {
		it.Spec = c.spec
	}
	if f.Parent != "" {
		if nested {
			c.problem(label + ": parent is only allowed on top-level items (children take the enclosing item)")
		}
		it.ParentRef = f.Parent
	}
	c.items = append(c.items, it)
	for _, ch := range f.Children {
		c.convert(ch, it.Ref, true)
	}
}
