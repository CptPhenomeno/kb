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
	editID   string // "" for a new item
	version  int
	fields   []formField
	focus    int
	errs     map[int]string // validation errors by field
	general  string         // an error not tied to a field
	info     string         // a confirmation, e.g. after the editor
	saving   bool
	criteria []string // criteria added so far in the create form
	tests    []string // tests added so far in the create form
	tab      formTab  // active tab of the create form
}

// formTab is a tab of the create form; the edit form has no tabs.
type formTab int

const (
	formDetails formTab = iota
	formCriteria
	formTests
)

var formTabLabels = []string{"Details", "Criteria", "Tests"}

// fTests is the tests input of the create form.
const fTests = fAcceptance + 1

// tabFields lists the field indexes the tab moves among.
func (t formTab) fields() []int {
	switch t {
	case formCriteria:
		return []int{fAcceptance}
	case formTests:
		return []int{fTests}
	}
	return []int{fTitle, fType, fParent, fPriority, fPoints, fLabels, fSpec, fDescription}
}

// show makes tab t active and focuses its first field.
func (f *formState) show(t formTab) {
	f.tab = t
	f.focus = t.fields()[0]
}

// tabOf is the tab a field lives on.
func tabOf(field int) formTab {
	switch {
	case field == fAcceptance:
		return formCriteria
	case field == fTests:
		return formTests
	}
	return formDetails
}

// step moves the focus by delta among the fields of the active tab.
func (f *formState) step(delta int) {
	fs := f.tab.fields()
	i := slices.Index(fs, f.focus)
	f.focus = fs[(i+delta+len(fs))%len(fs)]
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
	fAcceptance // create form only
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
	if it == nil {
		f.fields = append(f.fields, formField{label: "acceptance"}, formField{label: "tests"})
	} else {
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
	switch key {
	case "esc":
		m.form = nil
		return nil
	case "ctrl+s":
		f.saving = true
		return m.submitForm()
	case "ctrl+right", "ctrl+left":
		if f.editID == "" {
			d := 1
			if key == "ctrl+left" {
				d = len(formTabLabels) - 1
			}
			f.show((f.tab + formTab(d)) % formTab(len(formTabLabels)))
		}
		return nil
	case "tab", "down":
		f.step(1)
		return nil
	case "shift+tab", "up":
		f.step(-1)
		return nil
	}
	fld := &f.fields[f.focus]
	switch key {
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
		if list := f.listOf(f.focus); list != nil && fld.value == "" {
			if n := len(*list); n > 0 {
				*list = (*list)[:n-1]
			}
		} else if fld.choices == nil {
			if r := []rune(fld.value); len(r) > 0 {
				fld.value = string(r[:len(r)-1])
			}
		}
	case "ctrl+e":
		return m.editDescription(f.fields[fDescription].value, "", 0)
	case "enter":
		switch f.focus {
		case fDescription:
			return m.editDescription(fld.value, "", 0)
		case fAcceptance, fTests:
			f.addPending(f.focus)
			return nil
		}
		f.saving = true
		return m.submitForm()
	default:
		if fld.choices == nil && text != "" {
			fld.value += text
		}
	}
	return nil
}

// listOf is the list an input of the create form adds to, or nil for the
// other fields.
func (f *formState) listOf(field int) *[]string {
	switch field {
	case fAcceptance:
		return &f.criteria
	case fTests:
		return &f.tests
	}
	return nil
}

// addPending moves the text typed in a list input, if not blank, to its list.
func (f *formState) addPending(field int) {
	list := f.listOf(field)
	if list == nil || len(f.fields) <= field {
		return
	}
	in := &f.fields[field]
	if text := strings.TrimSpace(in.value); text != "" {
		*list = append(*list, text)
		in.value = ""
	}
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
	f.addPending(fAcceptance)
	f.addPending(fTests)
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
		for _, text := range f.criteria {
			it.Acceptance = append(it.Acceptance, core.Criterion{Text: text})
		}
		for _, name := range f.tests {
			it.Tests = append(it.Tests, core.Test{Name: name})
		}
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
	case strings.Contains(lower, "acceptance criterion"):
		field = fAcceptance
	case strings.HasPrefix(lower, "test "):
		field = fTests
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
	lines := []string{spread(m.th.bright.Render(title), m.th.muted.Render("esc cancel"), iw)}
	if f.editID == "" {
		lines = append(lines, spread(m.renderFormTabs(), m.th.muted.Render("ctrl+← → switch tab"), iw))
	}
	lines = append(lines, "")
	for i, fld := range f.fields {
		if f.editID == "" && tabOf(i) != f.tab {
			continue
		}
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
		case focused:
			value = m.th.bright.Render(value) + m.th.fg(m.th.p.Accent).Render("▌")
		default:
			value = m.th.text.Render(value)
		}
		marker := "  "
		if focused {
			marker = m.th.fg(m.th.p.Accent).Render("› ")
		}
		if i == fTitle || i == fDescription || i == fAcceptance || i == fTests {
			lines = append(lines, m.renderWrappedField(marker+label, fld.value, i == fDescription, focused, iw)...)
		} else {
			lines = append(lines, truncate(marker+label+value, iw))
		}
		if list := f.listOf(i); list != nil {
			for _, c := range *list {
				lines = append(lines, hang(strings.Repeat(" ", 14)+m.th.muted.Render("• "), 16, c, m.th.text, iw, 16)...)
			}
		}
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
	status := "enter or ctrl+s saves · tab or ↑ ↓ move · ← → change a choice"
	switch {
	case f.saving:
		status = "saving… · tab or ↑ ↓ move · ← → change a choice"
	case f.focus == fDescription:
		status = "enter $EDITOR · ctrl+s saves · tab or ↑ ↓ move"
	case f.focus == fAcceptance || f.focus == fTests:
		status = "enter adds · ctrl+s saves · backspace on empty removes the last · tab or ↑ ↓ move"
	}
	lines = append(lines, "", m.th.muted.Render(status))
	return m.popupBox(lines, width, m.th.fg(m.th.p.Accent))
}

// renderFormTabs shows the tab labels, the active one highlighted.
func (m *Model) renderFormTabs() string {
	out := make([]string, len(formTabLabels))
	for i, l := range formTabLabels {
		if formTab(i) == m.form.tab {
			out[i] = m.th.fg(m.th.p.Accent).Bold(true).Render("[" + l + "]")
		} else {
			out[i] = m.th.muted.Render(" " + l + " ")
		}
	}
	return strings.Join(out, " ")
}

// descriptionLines is the most lines of the description the form shows.
const descriptionLines = 6

// renderWrappedField shows a long text field (title or description) wrapped
// in the value column, which starts after head. The description is capped at
// descriptionLines: when longer, the last lines show, led by "…". The cursor
// follows the last line when the field has the focus.
func (m *Model) renderWrappedField(head, value string, capped, focused bool, iw int) []string {
	limit := 0
	if capped {
		limit = descriptionLines
	}
	in := wrapInput(value, iw-14, limit)
	if !focused && len(in) > 1 && in[len(in)-1] == "" {
		in = in[:len(in)-1]
	}
	style := m.th.text
	if focused {
		style = m.th.bright
	}
	lines := make([]string, len(in))
	for i, l := range in {
		lead := strings.Repeat(" ", 14)
		if i == 0 {
			lead = head
		}
		lines[i] = lead + style.Render(l)
	}
	if focused {
		lines[len(lines)-1] += m.th.fg(m.th.p.Accent).Render("▌")
	}
	return lines
}

var formKeys = []keyHint{
	{"↑ ↓", "field", "move between fields (also tab)", false},
	{"← →", "choice", "change a choice field", false},
	{"ctrl+e", "editor", "edit the description in $EDITOR", false},
	{"ctrl+← ctrl+→", "tab", "switch between the Details, Criteria and Tests tabs (create form)", false},
	{"ctrl+s", "save", "create or save the item from any field", false},
	{"enter", "save · $EDITOR · add", "save the item, or open $EDITOR on the description, or add a criterion on acceptance", false},
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
		m.form.info = fmt.Sprintf("description updated (%s) · ctrl+s saves", plural(len(strings.Split(msg.text, "\n")), "line", "lines"))
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
