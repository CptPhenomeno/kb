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
	it := m.board.selected(m)
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
	it := m.board.selected(m)
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

type linkRow struct {
	label, other string
	rel          core.Relation
}

// linkRows lists the relations of an item from its point of view.
func linkRows(s *core.Snapshot, id string) []linkRow {
	var rows []linkRow
	for _, r := range s.Relations {
		switch {
		case r.Kind == core.RelBlocks && r.From == id:
			rows = append(rows, linkRow{"blocks", r.To, r})
		case r.Kind == core.RelBlocks && r.To == id:
			rows = append(rows, linkRow{"blocked by", r.From, r})
		case r.Kind == core.RelRelates && r.From == id:
			rows = append(rows, linkRow{"relates", r.To, r})
		case r.Kind == core.RelRelates && r.To == id:
			rows = append(rows, linkRow{"relates", r.From, r})
		case r.Kind == core.RelDuplicates && r.From == id:
			rows = append(rows, linkRow{"duplicates", r.To, r})
		case r.Kind == core.RelDuplicates && r.To == id:
			rows = append(rows, linkRow{"duplicated by", r.From, r})
		}
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
	case "x", "delete", "backspace":
		if l.cursor < len(rows) {
			rel, item := rows[l.cursor].rel, *m.snap.Get(l.id)
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
		lines = append(lines, m.th.muted.Render("no links yet"))
	}
	for i, r := range rows {
		other := r.other
		if o := m.snap.Get(r.other); o != nil {
			other += " " + o.Title
		}
		marker, style := "  ", m.th.text
		if i == l.cursor && !l.adding {
			marker, style = m.th.fg(m.th.p.Accent).Render("› "), m.th.fg(m.th.p.Accent).Bold(true)
		}
		lines = append(lines, marker+m.th.muted.Render(fmt.Sprintf("%-14s", r.label))+style.Render(truncate(other, iw-16)))
	}
	lines = append(lines, "")
	if l.adding {
		lines = append(lines, m.th.muted.Render("new link: ")+m.th.fg(m.th.p.Accent).Render("‹ "+linkKinds[l.kind]+" › ")+
			m.th.bright.Render(l.target)+m.th.fg(m.th.p.Accent).Render("▌"),
			m.th.muted.Render("← → kind · type an ID or ref · enter adds · esc cancels"))
	} else {
		lines = append(lines, m.th.muted.Render("a adds a link · x removes the selected one"))
	}
	return m.popupBox(lines, width, m.th.fg(m.th.p.Accent))
}

var linksKeys = []keyHint{
	{"↑ ↓", "choose", "choose a link", false},
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
