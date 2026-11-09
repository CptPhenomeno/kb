package tui

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/CptPhenomeno/kb/internal/core"
)

// formField is one row of the create/edit form: a text input or a choice.
type formField struct {
	label   string
	value   string   // text value, or the current choice
	choices []string // nil for text fields
	locked  bool     // shown but not editable (the type of an existing item)
}

// formState is the open create or edit form.
type formState struct {
	editID  string // "" for a new item
	version int
	fields  []formField
	focus   int
	errs    map[int]string // validation errors by field
	general string         // an error not tied to a field
	info    string         // a confirmation, e.g. after the editor
	saving  bool
}

// Field indexes.
const (
	fTitle = iota
	fType
	fParent
	fPriority
	fPoints
	fLabels
	fSpec
	fDescription
)

var (
	formTypes      = []string{"task", "bug", "story", "epic", "subtask"}
	formPriorities = []string{"low", "medium", "high"}
	formPoints     = []string{"-", "1", "2", "3", "5", "8", "13"}
)

func newForm(it *core.Item) *formState {
	f := &formState{errs: map[int]string{}, fields: []formField{
		{label: "title"},
		{label: "type", value: "task", choices: formTypes},
		{label: "parent"},
		{label: "priority", value: "medium", choices: formPriorities},
		{label: "points", value: "-", choices: formPoints},
		{label: "labels"},
		{label: "spec"},
		{label: "description"},
	}}
	if it != nil {
		f.editID, f.version = it.ID, it.Version
		f.fields[fTitle].value = it.Title
		f.fields[fType].value, f.fields[fType].locked = it.Type.String(), true
		f.fields[fParent].value = it.Parent
		f.fields[fPriority].value = it.Priority.String()
		if it.Points != 0 {
			f.fields[fPoints].value = strconv.Itoa(it.Points)
		}
		f.fields[fLabels].value = strings.Join(it.Labels, ", ")
		f.fields[fSpec].value = it.Spec
		f.fields[fDescription].value = it.Description
	}
	return f
}

// formDoneMsg reports the outcome of a form submission.
type formDoneMsg struct {
	id      string
	errs    map[int]string
	general string
}

// editorDoneMsg returns the description edited in $EDITOR. target is the
// item edited from the board, or "" for the open form.
type editorDoneMsg struct {
	target  string
	version int
	text    string
	err     error
}

func (m *Model) formKey(key, text string) tea.Cmd {
	f := m.form
	if f.saving {
		return nil
	}
	fld := &f.fields[f.focus]
	switch key {
	case "esc":
		m.form = nil
	case "tab", "down":
		f.focus = (f.focus + 1) % len(f.fields)
	case "shift+tab", "up":
		f.focus = (f.focus + len(f.fields) - 1) % len(f.fields)
	case "left", "right":
		if fld.choices != nil && !fld.locked {
			i := slices.Index(fld.choices, fld.value)
			if key == "right" {
				i = (i + 1) % len(fld.choices)
			} else {
				i = (i + len(fld.choices) - 1) % len(fld.choices)
			}
			fld.value = fld.choices[i]
		}
	case "backspace":
		if fld.choices == nil {
			if r := []rune(fld.value); len(r) > 0 {
				fld.value = string(r[:len(r)-1])
			}
		}
	case "ctrl+e":
		return m.editDescription(f.fields[fDescription].value, "", 0)
	case "enter":
		f.saving = true
		return m.submitForm()
	default:
		if fld.choices == nil && text != "" {
			fld.value += text
		}
	}
	return nil
}

// editDescription opens a description in $VISUAL or $EDITOR; target and
// version identify the item when editing from the board.
func (m *Model) editDescription(initial, target string, version int) tea.Cmd {
	tmp, err := os.CreateTemp("", "kb-description-*.md")
	if err != nil {
		return nil
	}
	_, _ = tmp.WriteString(initial)
	tmp.Close()
	editor := os.Getenv("VISUAL")
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}
	if editor == "" {
		editor = "vi"
	}
	cmd := exec.Command("sh", "-c", editor+` "$1"`, "kb-edit", tmp.Name())
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		defer os.Remove(tmp.Name())
		b, rerr := os.ReadFile(tmp.Name())
		if err == nil {
			err = rerr
		}
		return editorDoneMsg{target: target, version: version, text: strings.TrimRight(string(b), "\n"), err: err}
	})
}

// submitForm creates or edits the item through the core; rule errors come
// back next to the field they concern.
func (m *Model) submitForm() tea.Cmd {
	f := m.form
	val := func(i int) string { return strings.TrimSpace(f.fields[i].value) }
	prio, _ := core.ParsePriority(val(fPriority))
	points, _ := strconv.Atoi(val(fPoints))
	var labels []string
	for _, l := range strings.Split(val(fLabels), ",") {
		if l = strings.TrimSpace(l); l != "" {
			labels = append(labels, l)
		}
	}
	actor, editID, version := m.opts.Actor, f.editID, f.version
	desc := f.fields[fDescription].value
	if editID == "" {
		typ, _ := core.ParseType(val(fType))
		it := core.Item{Type: typ, Title: val(fTitle), Parent: val(fParent), Priority: prio, Points: points,
			Labels: labels, Spec: val(fSpec), Description: desc}
		return func() tea.Msg {
			created, err := m.be.Create(m.ctx, actor, it)
			if err != nil {
				return formError(err)
			}
			return formDoneMsg{id: created.ID}
		}
	}
	title, parent, spec := val(fTitle), val(fParent), val(fSpec)
	p := core.Patch{Title: &title, Description: &desc, Parent: &parent, Priority: &prio, Points: &points,
		Spec: &spec, Labels: &labels}
	return func() tea.Msg {
		if _, err := m.be.Edit(m.ctx, actor, editID, p, version); err != nil {
			if errors.Is(err, core.ErrConflict) {
				return formDoneMsg{general: "the item changed meanwhile: close the form (esc) and edit it again"}
			}
			return formError(err)
		}
		return formDoneMsg{id: editID}
	}
}

// formError places an error next to the field it is about.
func formError(err error) formDoneMsg {
	msg := err.Error()
	var re *core.RuleError
	if errors.As(err, &re) {
		msg = re.Error()
	}
	lower := strings.ToLower(msg)
	field := -1
	switch {
	case strings.Contains(lower, "title"):
		field = fTitle
	case strings.Contains(lower, "parent"), strings.Contains(lower, "placed under"):
		field = fParent
	case strings.Contains(lower, "points"):
		field = fPoints
	case strings.Contains(lower, "tdd phase"), strings.Contains(lower, "tests are not allowed"):
		field = fType
	}
	if field < 0 {
		return formDoneMsg{general: msg}
	}
	return formDoneMsg{errs: map[int]string{field: msg}}
}

func (m *Model) renderForm() string {
	f := m.form
	width := min(76, m.width-4)
	iw := width - 6
	title := "New item"
	if f.editID != "" {
		title = "Edit " + f.editID
	}
	lines := []string{spread(m.th.bright.Render(title), m.th.muted.Render("esc cancel"), iw), ""}
	for i, fld := range f.fields {
		label := m.th.muted.Render(fmt.Sprintf("%-12s", fld.label))
		value := fld.value
		focused := i == f.focus
		switch {
		case fld.choices != nil && fld.locked:
			value = m.th.muted.Render(value + " (fixed)")
		case fld.choices != nil && focused:
			value = m.th.fg(m.th.p.Accent).Render("‹ ") + m.th.bright.Render(value) + m.th.fg(m.th.p.Accent).Render(" ›")
		case fld.choices != nil:
			value = m.th.text.Render(value)
		case i == fDescription:
			first, _, more := strings.Cut(value, "\n")
			value = m.th.text.Render(first)
			if more {
				value += m.th.muted.Render(" …")
			}
			if focused {
				value += m.th.fg(m.th.p.Accent).Render("▌") + m.th.muted.Render("  ctrl+e: $EDITOR")
			}
		case focused:
			value = m.th.bright.Render(value) + m.th.fg(m.th.p.Accent).Render("▌")
		default:
			value = m.th.text.Render(value)
		}
		marker := "  "
		if focused {
			marker = m.th.fg(m.th.p.Accent).Render("› ")
		}
		lines = append(lines, truncate(marker+label+value, iw))
		if e, ok := f.errs[i]; ok {
			lines = append(lines, hang(strings.Repeat(" ", 14)+m.th.warn.Render("✗ "), 16, e, m.th.warn, iw, 16)...)
		}
	}
	if f.general != "" {
		lines = append(lines, "", m.th.warn.Render("✗ ")+m.th.warn.Render(f.general))
	}
	if f.info != "" {
		lines = append(lines, "", m.th.fg(m.th.p.Green).Render("✓ "+f.info))
	}
	status := "enter saves"
	if f.saving {
		status = "saving…"
	}
	lines = append(lines, "", m.th.muted.Render(status+" · tab or ↑ ↓ move · ← → change a choice"))
	return m.popupBox(lines, width, m.th.fg(m.th.p.Accent))
}

var formKeys = []keyHint{
	{"↑ ↓", "field", "move between fields (also tab)", false},
	{"← →", "choice", "change a choice field", false},
	{"ctrl+e", "editor", "edit the description in $EDITOR", false},
	{"enter", "save", "create or save the item", false},
	{"esc", "cancel", "close without saving", false},
}

// editorDone takes back a description from the editor: into the form, or
// saved at once for an item edited from the board.
func (m *Model) editorDone(msg editorDoneMsg) tea.Cmd {
	if msg.target == "" {
		if m.form == nil {
			return nil
		}
		if msg.err != nil {
			m.form.general = "editor failed: " + msg.err.Error()
			return nil
		}
		m.form.fields[fDescription].value = msg.text
		m.form.info = fmt.Sprintf("description updated (%s) · enter saves", plural(len(strings.Split(msg.text, "\n")), "line", "lines"))
		return nil
	}
	m.editing = ""
	it := m.snap.Get(msg.target)
	switch {
	case it == nil:
		return nil
	case msg.err != nil:
		m.alert = &alertState{title: "EDITOR FAILED", headline: it.ID + " " + it.Title, detail: msg.err.Error(),
			footer: "The description was not changed."}
		return nil
	case msg.text == it.Description:
		m.notice = it.ID + " description unchanged"
		return nil
	}
	item, text, actor := *it, msg.text, m.opts.Actor
	item.Version = msg.version
	return func() tea.Msg {
		if _, err := m.be.Edit(m.ctx, actor, item.ID, core.Patch{Description: &text}, item.Version); err != nil {
			return writeMsg{alert: m.failure(item, "SAVE FAILED", err)}
		}
		return writeMsg{notice: item.ID + " description saved"}
	}
}
