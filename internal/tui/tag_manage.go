package tui

import (
	"fmt"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/CptPhenomeno/kb/internal/core"
)

// tagsMode is what the Manage tags popup is doing; rename and create add
// their own modes next to the list and the delete confirmation.
type tagsMode int

const (
	tagsList          tagsMode = iota // browsing the tags
	tagsConfirmDelete                 // asking to delete the selected tag
)

// tagsState is the open Manage tags popup.
type tagsState struct {
	cursor int
	mode   tagsMode
}

// actionManageTags is the palette entry that opens the Manage tags popup.
const actionManageTags = "Manage tags"

// selectedTag is the tag under the cursor, or nil when there is none.
func (m *Model) selectedTag() *core.Tag {
	t := m.tagsPopup
	if t == nil || len(m.tags) == 0 {
		return nil
	}
	t.cursor = max(0, min(t.cursor, len(m.tags)-1))
	return &m.tags[t.cursor]
}

func (m *Model) tagsKey(key string) tea.Cmd {
	t := m.tagsPopup
	tag := m.selectedTag()
	if t.mode == tagsConfirmDelete {
		switch key {
		case "enter", "y":
			t.mode = tagsList
			if tag == nil {
				return nil
			}
			name, actor := tag.Name, m.opts.Actor
			return m.tagWrite("DELETE FAILED", name, func() error {
				_, err := m.be.DeleteTag(m.ctx, actor, name)
				return err
			})
		case "esc", "n":
			t.mode = tagsList
		}
		return nil
	}
	switch key {
	case "esc":
		m.tagsPopup = nil
	case "up":
		t.cursor = max(0, t.cursor-1)
	case "down":
		t.cursor = min(max(0, len(m.tags)-1), t.cursor+1)
	case "left", "right":
		if tag == nil {
			return nil
		}
		i := slices.Index(core.TagColors, tag.Color)
		d := 1
		if key == "left" {
			d = len(core.TagColors) - 1
		}
		color := core.TagColors[(i+d+len(core.TagColors))%len(core.TagColors)]
		// repaint at once; the saves catch up one at a time
		tag.Color = color
		m.tagColors[core.TagKey(tag.Name)] = color
		if m.wantedColors == nil {
			m.wantedColors = map[string]wantedColor{}
		}
		m.wantedColors[core.TagKey(tag.Name)] = wantedColor{tag.Name, color}
		return m.saveNextColor()
	case "d":
		if tag != nil {
			t.mode = tagsConfirmDelete
		}
	}
	return nil
}

// wantedColor is the color the user chose for a tag and that is not saved yet.
type wantedColor struct {
	name  string
	color core.TagColor
}

// tagColorSavedMsg reports a finished color save.
type tagColorSavedMsg struct {
	key   string
	color core.TagColor
	err   error
}

// saveNextColor starts the save of a wanted color unless one is in flight:
// there is at most one color save at a time, so the last choice is the one
// that ends up stored whatever the scheduling.
func (m *Model) saveNextColor() tea.Cmd {
	if m.savingColor || len(m.wantedColors) == 0 {
		return nil
	}
	var key string
	for k := range m.wantedColors { // any order: each tag keeps only its last choice
		if key == "" || k < key {
			key = k
		}
	}
	w, actor := m.wantedColors[key], m.opts.Actor
	m.savingColor = true
	return func() tea.Msg {
		_, err := m.be.SetTagColor(m.ctx, actor, w.name, string(w.color))
		return tagColorSavedMsg{key, w.color, err}
	}
}

// colorSaved handles a finished save: a failure drops the wanted color and
// shows an alert; otherwise the next wanted color is saved, or the board is
// reloaded when nothing is left.
func (m *Model) colorSaved(msg tagColorSavedMsg) tea.Cmd {
	m.savingColor = false
	if w, ok := m.wantedColors[msg.key]; ok && (msg.err != nil || w.color == msg.color) {
		delete(m.wantedColors, msg.key)
	}
	if msg.err != nil {
		m.alert = &alertState{title: "RECOLOR FAILED", headline: "tag " + msg.key, detail: msg.err.Error(), footer: "The board is reloaded."}
	}
	if next := m.saveNextColor(); next != nil {
		return next
	}
	return m.load()
}

// applyWantedColors repaints the colors chosen and not saved yet over a
// freshly loaded snapshot, so a reload never steps back to an older color.
func (m *Model) applyWantedColors() {
	for key, w := range m.wantedColors {
		m.tagColors[key] = w.color
		for i := range m.tags {
			if core.TagKey(m.tags[i].Name) == key {
				m.tags[i].Color = w.color
			}
		}
	}
}

// tagWrite runs a tag write in a command and reloads the board after it; a
// failure shows an alert naming the tag.
func (m *Model) tagWrite(title, name string, write func() error) tea.Cmd {
	return func() tea.Msg {
		if err := write(); err != nil {
			return writeMsg{alert: &alertState{title: title, headline: "tag " + name, detail: err.Error(), footer: "The board is reloaded."}}
		}
		return writeMsg{}
	}
}

func (m *Model) renderTags() string {
	t := m.tagsPopup
	width := min(64, m.width-4)
	iw := width - 6
	lines := []string{spread(m.th.bright.Render("Manage tags"), m.th.muted.Render("esc close"), iw), ""}
	if len(m.tags) == 0 {
		lines = append(lines, m.th.muted.Render("no tags yet"))
	}
	nameW := 0
	for _, tg := range m.tags {
		nameW = max(nameW, len([]rune(tg.Name)))
	}
	nameW = min(nameW+2, iw-24)
	for i, tg := range m.tags {
		marker := "  "
		if i == t.cursor {
			marker = m.th.fg(m.th.p.Accent).Render("› ")
		}
		label := m.th.tagLabel(m.tagColor(tg.Name), tg.Name, nameW)
		pad := strings.Repeat(" ", max(0, nameW-lipgloss.Width(label)))
		lines = append(lines, spread(marker+label+pad+m.th.muted.Render(string(tg.Color)),
			m.th.muted.Render(plural(tg.Cards, "card", "cards")), iw))
	}
	lines = append(lines, "")
	if tag := m.selectedTag(); t.mode == tagsConfirmDelete && tag != nil {
		loss := "No card uses it."
		if tag.Cards > 0 {
			loss = lossText(tag.Cards)
		}
		lines = append(lines, m.th.warn.Bold(true).Render("DELETE")+" "+m.th.bright.Render("Delete ")+
			m.th.tagLabel(m.tagColor(tag.Name), tag.Name, iw-12)+m.th.bright.Render("?"),
			m.th.warn.Render(loss),
			spread(m.th.chip.Render(" esc ")+" "+m.th.muted.Render("cancel"), m.th.badge.Render(" enter ")+" "+m.th.text.Render("delete"), iw))
		return m.popupBox(lines, width, m.th.warn)
	}
	lines = append(lines, m.th.muted.Render("↑ ↓ move · ← → color · d delete · esc close"))
	return m.popupBox(lines, width, m.th.fg(m.th.p.Accent))
}

// lossText says how many cards lose a tag that is deleted.
func lossText(n int) string {
	if n == 1 {
		return "1 card loses it."
	}
	return fmt.Sprintf("%d cards lose it.", n)
}

var tagsKeys = []keyHint{
	{"↑ ↓", "move", "choose a tag", false},
	{"← →", "color", "change the color of the selected tag; it is saved at once", false},
	{"d", "delete", "delete the selected tag from the board and every card, after confirmation", false},
	{"esc", "close", "close the popup", false},
}

var tagsConfirmKeys = []keyHint{
	{"enter", "delete", "delete the tag and remove it from its cards", false},
	{"esc", "cancel", "keep the tag", false},
}
