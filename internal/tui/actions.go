package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/CptPhenomeno/kb/internal/core"
)

// ---- delete ----

// confirmState is an open delete confirmation.
type confirmState struct {
	item        core.Item
	descendants []string
}

func (m *Model) openDelete() {
	it := m.selected()
	if it == nil {
		return
	}
	var desc []string
	for _, d := range m.snap.Descendants(it.ID) {
		desc = append(desc, d.ID)
	}
	m.confirm = &confirmState{item: *it, descendants: desc}
}

func (m *Model) confirmKey(key string) tea.Cmd {
	c := m.confirm
	switch key {
	case "enter", "y":
		m.confirm = nil
		item, recursive, actor := c.item, len(c.descendants) > 0, m.opts.Actor
		return func() tea.Msg {
			if _, err := m.be.Remove(m.ctx, actor, item.ID, recursive); err != nil {
				return writeMsg{alert: m.failure(item, "DELETE FAILED", err)}
			}
			return writeMsg{}
		}
	case "esc", "n":
		m.confirm = nil
	}
	return nil
}

func (m *Model) renderConfirm() string {
	c := m.confirm
	width := min(64, m.width-4)
	iw := width - 6
	lines := []string{
		m.th.warn.Bold(true).Render("DELETE"),
		m.th.bright.Render(fmt.Sprintf("Delete %s %s?", c.item.ID, c.item.Title)),
	}
	if n := len(c.descendants); n > 0 {
		lines = append(lines, "", m.th.warn.Render(plural(n, "descendant is deleted with it:", "descendants are deleted with it:")))
		lines = append(lines, hang("", 0, strings.Join(c.descendants, ", "), m.th.text, iw, 0)...)
	}
	lines = append(lines, "", m.th.muted.Render("The history stays in the event log."), "",
		spread(m.th.chip.Render(" esc ")+" "+m.th.muted.Render("cancel"), m.th.badge.Render(" enter ")+" "+m.th.text.Render("delete"), iw))
	return m.popupBox(lines, width, m.th.warn)
}

var confirmKeys = []keyHint{{"enter", "delete", "delete the item", false}, {"esc", "cancel", "keep the item", false}}

// ---- assign ----

// assign claims the selected item for the TUI user, or releases the claim
// when the user already holds it.
func (m *Model) assign() tea.Cmd {
	it := m.selected()
	if it == nil {
		return nil
	}
	item, actor := *it, m.opts.Actor
	release := it.Claim != nil && it.Claim.Actor == actor
	return func() tea.Msg {
		var err error
		if release {
			_, err = m.be.Release(m.ctx, actor, item.ID, item.Version)
		} else {
			_, err = m.be.Claim(m.ctx, actor, item.ID, item.Version)
		}
		if err != nil {
			title := "CLAIM REFUSED"
			if release {
				title = "RELEASE FAILED"
			}
			return writeMsg{alert: m.failure(item, title, err)}
		}
		return writeMsg{}
	}
}

// ---- links ----

// linkKinds are the relations the links popup can add, from the selected
// item's point of view.
var linkKinds = []string{"blocks", "blocked by", "relates", "duplicates"}

// linksState is the open links popup.
type linksState struct {
	id     string
	cursor int
	adding bool
	kind   int
	target string
}

// linkRow is a row of the links popup: a relation, or the parent or a
// child of the item (hierarchy rows can be visited but not unlinked).
type linkRow struct {
	core.Link
	hierarchy bool
}

// linkRows lists the parent, the children and the relations of an item.
func linkRows(s *core.Snapshot, id string) []linkRow {
	var rows []linkRow
	if it := s.Get(id); it != nil && it.Parent != "" {
		rows = append(rows, linkRow{core.Link{Label: "parent", Other: it.Parent}, true})
	}
	for _, c := range s.Children(id) {
		rows = append(rows, linkRow{core.Link{Label: "child", Other: c.ID}, true})
	}
	for _, l := range core.Links(s, id) {
		rows = append(rows, linkRow{Link: l})
	}
	return rows
}

func (m *Model) linksKey(key, text string) tea.Cmd {
	l := m.links
	if m.snap.Get(l.id) == nil {
		m.links = nil
		return nil
	}
	actor := m.opts.Actor
	if l.adding {
		switch key {
		case "esc":
			l.adding, l.target = false, ""
		case "left":
			l.kind = (l.kind + len(linkKinds) - 1) % len(linkKinds)
		case "right":
			l.kind = (l.kind + 1) % len(linkKinds)
		case "backspace":
			if r := []rune(l.target); len(r) > 0 {
				l.target = string(r[:len(r)-1])
			}
		case "enter":
			target := strings.TrimSpace(l.target)
			if target == "" {
				return nil
			}
			rel := core.Relation{From: l.id, To: target}
			switch linkKinds[l.kind] {
			case "blocks":
				rel.Kind = core.RelBlocks
			case "blocked by":
				rel = core.Relation{Kind: core.RelBlocks, From: target, To: l.id}
			case "relates":
				rel.Kind = core.RelRelates
			default:
				rel.Kind = core.RelDuplicates
			}
			l.adding, l.target = false, ""
			item := *m.snap.Get(l.id)
			return func() tea.Msg {
				if err := m.be.Link(m.ctx, actor, rel); err != nil {
					return writeMsg{alert: m.failure(item, "LINK REFUSED", err)}
				}
				return writeMsg{}
			}
		default:
			l.target += text
		}
		return nil
	}
	rows := linkRows(m.snap, l.id)
	switch key {
	case "esc", "l":
		m.links = nil
	case "up":
		l.cursor = max(0, l.cursor-1)
	case "down":
		l.cursor = min(max(0, len(rows)-1), l.cursor+1)
	case "a":
		l.adding = true
	case "enter":
		if l.cursor < len(rows) {
			m.links = nil
			return m.goTo(rows[l.cursor].Other)
		}
	case "x", "delete", "backspace":
		if l.cursor < len(rows) && rows[l.cursor].hierarchy {
			m.notice = "the hierarchy is not a link: change the parent with e"
			return nil
		}
		if l.cursor < len(rows) {
			rel, item := rows[l.cursor].Rel, *m.snap.Get(l.id)
			l.cursor = max(0, l.cursor-1)
			return func() tea.Msg {
				if err := m.be.Unlink(m.ctx, actor, rel); err != nil {
					return writeMsg{alert: m.failure(item, "UNLINK FAILED", err)}
				}
				return writeMsg{}
			}
		}
	}
	return nil
}

func (m *Model) renderLinks() string {
	l := m.links
	it := m.snap.Get(l.id)
	width := min(68, m.width-4)
	iw := width - 6
	lines := []string{
		spread(m.th.bright.Render("Links"), m.th.muted.Render("esc close"), iw),
		m.th.text.Render(it.ID + " " + it.Title),
		"",
	}
	rows := linkRows(m.snap, l.id)
	if len(rows) == 0 {
		lines = append(lines, m.th.muted.Render("no parent, children or links yet"))
	}
	for i, r := range rows {
		marker, style := "  ", m.th.text
		if i == l.cursor && !l.adding {
			marker, style = m.th.fg(m.th.p.Accent).Render("› "), m.th.fg(m.th.p.Accent).Bold(true)
		}
		left := marker + m.th.muted.Render(fmt.Sprintf("%-14s", r.Label)) + style.Render(r.Other)
		right := ""
		if o := m.snap.Get(r.Other); o != nil {
			left += " " + style.Render(o.Title)
			right = m.th.fg(m.th.statusColor[o.Status]).Render(o.Status.Label())
		}
		lines = append(lines, spread(left, right, iw))
	}
	lines = append(lines, "")
	if l.adding {
		lines = append(lines, m.th.muted.Render("new link: ")+m.th.fg(m.th.p.Accent).Render("‹ "+linkKinds[l.kind]+" › ")+
			m.th.bright.Render(l.target)+m.th.fg(m.th.p.Accent).Render("▌"),
			m.th.muted.Render("← → kind · type an ID or ref · enter adds · esc cancels"))
	} else {
		lines = append(lines, hang("", 0, "enter goes to the item · a adds a link · x removes the selected link (the parent changes with e)", m.th.muted, iw, 0)...)
	}
	return m.popupBox(lines, width, m.th.fg(m.th.p.Accent))
}

var linksKeys = []keyHint{
	{"↑ ↓", "choose", "choose a link", false},
	{"enter", "go to", "go to the linked item", false},
	{"a", "add", "add a link", false},
	{"x", "remove", "remove the selected link", false},
	{"esc", "close", "close the popup", false},
}

var linkAddKeys = []keyHint{
	{"← →", "kind", "choose the kind of link", false},
	{"type", "target", "type the ID or ref of the other item", false},
	{"enter", "add", "add the link", false},
	{"esc", "cancel", "go back to the list", false},
}

// goTo selects an item, relaxing the filters and the search when they
// hide it. The Tree view selects the row, opening its branch; any other
// view switches to the Board.
func (m *Model) goTo(id string) tea.Cmd {
	it := m.snap.Get(id)
	if it == nil {
		return nil
	}
	b := &m.board
	if m.view == viewTree {
		if !m.tree.shown(m, it) {
			b.query, b.searching = "", false
			b.onlyBlocked, b.onlyStale = false, false
		}
		m.tree.reveal(m, id)
		m.tree.restore(m, id)
	} else {
		m.view = viewBoard
		if !b.matches(m, it) {
			b.types[it.Type] = true
			b.query, b.searching = "", false
			b.onlyBlocked, b.onlyStale = false, false
		}
		b.restore(m, id)
	}
	m.detailScroll = 0
	return m.selectionMoved()
}
