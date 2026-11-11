package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/CptPhenomeno/kb/internal/core"
)

// selected returns the item selected in the current view: a card on the
// Board, a row of the Tree, the item of the selected actor in Activity.
func (m *Model) selected() *core.Item {
	switch m.view {
	case viewTree:
		return m.tree.selected(m)
	case viewActivity:
		return m.activity.item(m)
	default:
		return m.board.selected(m)
	}
}

// selectedID returns the ID of the selected item, or "".
func (m *Model) selectedID() string {
	if it := m.selected(); it != nil {
		return it.ID
	}
	return ""
}

// bodyHeight is the number of rows between the header and the status line.
func (m *Model) bodyHeight() int { return m.height - 2 - len(m.commandBar()) }

// itemKey handles the keys that act on the selected item, the same in the
// Board and the Tree view.
func (m *Model) itemKey(key string) tea.Cmd {
	it := m.selected()
	switch key {
	case "pgdown", "pgup":
		if m.detailOpen {
			page := max(1, m.bodyHeight()-6)
			if key == "pgup" {
				page = -page
			}
			m.detailScroll = max(0, m.detailScroll+page)
		}
	case "f":
		m.filterOpen = true
	case "/":
		m.board.searching = true
	case "c":
		if it != nil {
			m.accept = &acceptState{id: it.ID}
		}
	case "d":
		m.openDelete()
	case "ctrl+e":
		if it != nil {
			m.editing = it.ID
			return m.editDescription(it.Description, it.ID, it.Version)
		}
	case "e":
		if it != nil {
			m.form = newForm(it)
		}
	case "a":
		return m.assign()
	case "l":
		if it != nil {
			m.links = &linksState{id: it.ID}
		}
	case "enter":
		if m.detailOpen {
			m.detailOpen = false
		} else if it != nil {
			m.detailOpen = true
			m.history = nil
			return m.loadHistory(it.ID)
		}
	}
	return nil
}

// selectionMoved reloads the history shown in the open detail popup after
// the selection changed.
func (m *Model) selectionMoved() tea.Cmd {
	if !m.detailOpen {
		return nil
	}
	m.history = nil
	return m.loadHistory(m.selectedID())
}
