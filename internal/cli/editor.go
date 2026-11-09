package cli

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"slices"

	"gopkg.in/yaml.v3"

	"github.com/CptPhenomeno/kb/internal/core"
)

// editDoc is the YAML document shown in $EDITOR. Only these fields can be
// edited this way; status, tests and criteria have their own commands.
type editDoc struct {
	Title       string   `yaml:"title"`
	Description string   `yaml:"description"`
	Priority    string   `yaml:"priority"`
	Points      int      `yaml:"points"`
	Parent      string   `yaml:"parent"`
	Ref         string   `yaml:"ref"`
	Spec        string   `yaml:"spec"`
	Labels      []string `yaml:"labels"`
	Links       []string `yaml:"links"`
}

// editInEditor lets the user edit an item as YAML in $VISUAL or $EDITOR.
// The edit is applied only if the item did not change meanwhile.
func (c *cli) editInEditor(s *Session, id string, ifVersion int) error {
	it, err := s.Service.Get(c.ctx, id)
	if err != nil {
		return err
	}
	if ifVersion == 0 {
		ifVersion = it.Version
	}
	doc := editDoc{Title: it.Title, Description: it.Description, Priority: it.Priority.String(),
		Points: it.Points, Parent: it.Parent, Ref: it.Ref, Spec: it.Spec,
		Labels: nonNil(it.Labels), Links: nonNil(it.Links)}
	body, err := yaml.Marshal(doc)
	if err != nil {
		return err
	}
	header := fmt.Sprintf("# Edit %s (%s, version %d). Save and quit to apply; quit without saving to cancel.\n", it.ID, it.Type, it.Version)
	original := append([]byte(header), body...)

	f, err := os.CreateTemp("", "kb-"+it.ID+"-*.yaml")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(original); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}

	editor := c.app.Getenv("VISUAL")
	if editor == "" {
		editor = c.app.Getenv("EDITOR")
	}
	if editor == "" {
		editor = "vi"
	}
	// Run through the shell so that EDITOR may carry arguments ("code -w").
	cmd := exec.Command("sh", "-c", editor+` "$1"`, "kb-edit", f.Name())
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, c.app.Stderr, c.app.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("editor %q failed: %w; no change applied", editor, err)
	}
	edited, err := os.ReadFile(f.Name())
	if err != nil {
		return err
	}
	if bytes.Equal(edited, original) {
		fmt.Fprintf(c.app.Stdout, "%s: no changes\n", it.ID)
		return nil
	}
	var out editDoc
	if err := yaml.Unmarshal(edited, &out); err != nil {
		return fmt.Errorf("invalid YAML: %v; no change applied", err)
	}
	p, err := patchFrom(it, out)
	if err != nil {
		return err
	}
	updated, err := s.Service.Edit(c.ctx, s.Actor, id, p, ifVersion)
	if err != nil {
		return err
	}
	return c.printItem(s, updated, fmt.Sprintf("%s updated (version %d)", updated.ID, updated.Version))
}

// patchFrom turns the edited document into a patch of the changed fields.
func patchFrom(it *core.Item, d editDoc) (core.Patch, error) {
	var p core.Patch
	if d.Title != it.Title {
		p.Title = &d.Title
	}
	if d.Description != it.Description {
		p.Description = &d.Description
	}
	prio, err := core.ParsePriority(d.Priority)
	if err != nil {
		return p, err
	}
	if prio != it.Priority {
		p.Priority = &prio
	}
	if d.Points != it.Points {
		p.Points = &d.Points
	}
	if d.Parent != it.Parent {
		p.Parent = &d.Parent
	}
	if d.Ref != it.Ref {
		p.Ref = &d.Ref
	}
	if d.Spec != it.Spec {
		p.Spec = &d.Spec
	}
	if !slices.Equal(d.Labels, nonNil(it.Labels)) {
		p.Labels = &d.Labels
	}
	if !slices.Equal(d.Links, nonNil(it.Links)) {
		p.Links = &d.Links
	}
	return p, nil
}
