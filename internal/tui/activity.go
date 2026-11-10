package tui

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/CptPhenomeno/kb/internal/core"
)

// activityState is the selection of the Activity view.
type activityState struct {
	cursor   int
	only     string // show only this actor's events; "" shows all
	evScroll int    // events skipped from the newest
}

// actorState is how an actor stands with its claims.
type actorState int

const (
	actorActive actorState = iota // holds an active claim
	actorStale                    // holds an expired claim on an item in progress
	actorIdle                     // no claim in progress
)

// actorRow is an actor of the Activity view.
type actorRow struct {
	name  string
	state actorState
	item  *core.Item // the item the actor works on, or nil
	last  time.Time  // last write among the recent events; zero if none
}

// actors lists who wrote recently or holds a claim: active claims first,
// then stale ones, then the others; most recent writes first within each.
func (a *activityState) actors(m *Model) []actorRow {
	if m.snap == nil {
		return nil
	}
	now := m.opts.Now()
	rows := map[string]*actorRow{}
	get := func(name string) *actorRow {
		if rows[name] == nil {
			rows[name] = &actorRow{name: name, state: actorIdle}
		}
		return rows[name]
	}
	for _, e := range m.recent {
		if e.Actor == "" || e.Actor == "kb" {
			continue
		}
		if r := get(e.Actor); e.At.After(r.last) {
			r.last = e.At
		}
	}
	for _, it := range m.snap.Items() {
		if it.Claim == nil || it.Status == core.StatusDone {
			continue // a claim left on finished work is not work
		}
		st := actorIdle
		switch {
		case core.ClaimState(it, now) == core.ClaimActive:
			st = actorActive
		case core.IsStale(it, now):
			st = actorStale
		default:
			continue // an expired claim on an item no longer in progress
		}
		// Prefer the best claim state, then an item in progress over one
		// waiting in review.
		r := get(it.Claim.Actor)
		better := r.item == nil || st < r.state ||
			(st == r.state && it.Status == core.StatusDoing && r.item.Status != core.StatusDoing)
		if better {
			r.state, r.item = st, it
		}
	}
	out := make([]actorRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, *r)
	}
	slices.SortFunc(out, func(x, y actorRow) int {
		if c := cmp.Compare(x.state, y.state); c != 0 {
			return c
		}
		if c := y.last.Compare(x.last); c != 0 {
			return c
		}
		return cmp.Compare(x.name, y.name)
	})
	return out
}

// current returns the selected actor.
func (a *activityState) current(m *Model) (actorRow, bool) {
	rows := a.actors(m)
	if len(rows) == 0 {
		return actorRow{}, false
	}
	return rows[max(0, min(a.cursor, len(rows)-1))], true
}

// item is the item the selected actor works on, or nil.
func (a *activityState) item(m *Model) *core.Item {
	if r, ok := a.current(m); ok {
		return r.item
	}
	return nil
}

// events returns the recent events shown, newest first.
func (a *activityState) events(m *Model) []core.Event {
	var out []core.Event
	for i := len(m.recent) - 1; i >= 0; i-- {
		if e := m.recent[i]; a.only == "" || e.Actor == a.only {
			out = append(out, e)
		}
	}
	return out
}

func (a *activityState) key(m *Model, key string) tea.Cmd {
	rows := a.actors(m)
	switch key {
	case "up":
		a.cursor = max(0, a.cursor-1)
	case "down":
		a.cursor = max(0, min(len(rows)-1, a.cursor+1))
	case "enter":
		if r, ok := a.current(m); ok {
			a.only, a.evScroll = r.name, 0
		}
	case "esc":
		a.only, a.evScroll = "", 0
	case "pgdown":
		a.evScroll = min(a.evScroll+max(1, m.bodyHeight()-4), max(0, len(a.events(m))-1))
	case "pgup":
		a.evScroll = max(0, a.evScroll-max(1, m.bodyHeight()-4))
	case "o":
		r, ok := a.current(m)
		if !ok || r.item == nil {
			m.notice = "the selected actor works on no item"
			return nil
		}
		m.view = viewBoard
		m.detailOpen = true
		return m.goTo(r.item.ID)
	case "a":
		r, ok := a.current(m)
		if !ok || r.item == nil {
			m.notice = "the selected actor holds no claim"
			return nil
		}
		item, actor, holder := *r.item, m.opts.Actor, r.name
		return func() tea.Msg {
			if _, err := m.be.Release(m.ctx, actor, item.ID, item.Version); err != nil {
				return writeMsg{alert: m.failure(item, "RELEASE FAILED", err)}
			}
			return writeMsg{notice: fmt.Sprintf("released %s (claim of %s)", item.ID, holder)}
		}
	}
	return nil
}

// ---- rendering ----

// actorsWidth is the width of the actors column.
func (m *Model) actorsWidth() int { return max(40, min(56, m.width*36/100)) }

func (a *activityState) filterLabel() string {
	if a.only != "" {
		return "live · following " + a.only
	}
	return "live · following all actors"
}

func (a *activityState) render(m *Model, height int) string {
	aw := m.actorsWidth()
	ew := m.width - aw - colGap
	return lipgloss.JoinHorizontal(lipgloss.Top, a.renderActors(m, aw, height), strings.Repeat(" ", colGap), a.renderEvents(m, ew, height))
}

// actorCardHeight is the height of an actor card, borders included.
const actorCardHeight = 5

func (a *activityState) renderActors(m *Model, w, height int) string {
	rows := a.actors(m)
	lines := []string{spread(m.th.bright.Render("ACTORS"), m.th.muted.Render(fmt.Sprint(len(rows))), w)}
	if len(rows) == 0 {
		lines = append(lines, m.th.muted.Render("no activity yet"))
	}
	fit := max(1, (height-1)/actorCardHeight)
	first := max(0, min(a.cursor-fit+1, len(rows)-fit))
	for i := first; i < len(rows) && i < first+fit; i++ {
		lines = append(lines, strings.Split(a.renderActor(m, rows[i], w, i == a.cursor), "\n")...)
	}
	return paintBlock(fitLines(strings.Join(lines, "\n"), w, height), m.th.p.Base, w)
}

func (a *activityState) renderActor(m *Model, r actorRow, w int, selected bool) string {
	now := m.opts.Now()
	iw := w - 4
	var state string
	switch {
	case r.state == actorActive:
		state = m.th.muted.Render("active · claim " + shortDuration(r.item.Claim.ExpiresAt.Sub(now)) + " left")
	case r.state == actorStale:
		state = m.th.stale.Render("stale · claim expired " + shortDuration(now.Sub(r.item.Claim.ExpiresAt)) + " ago")
	case core.IsHumanActor(r.name):
		state = m.th.muted.Render("human")
	default:
		state = m.th.muted.Render("idle")
	}
	work := m.th.muted.Render("no claimed item")
	detail := ""
	if it := r.item; it != nil {
		work = m.th.muted.Render(it.ID) + " " + m.th.text.Render(it.Title)
		var parts []string
		if it.Phase != core.PhaseNone {
			parts = append(parts, m.th.fg(m.th.phaseColor[it.Phase]).Render("● "+strings.ToUpper(it.Phase.String())))
		}
		if len(it.Tests) > 0 {
			parts = append(parts, m.th.muted.Render(fmt.Sprintf("tests %d/%d", it.PassingTests(), len(it.Tests))))
		}
		detail = strings.Join(parts, "  ")
	}
	last := "no recent writes"
	if !r.last.IsZero() {
		d := now.Sub(r.last)
		switch {
		case r.state == actorStale:
			last = "silent for " + shortDuration(d)
		case d < time.Minute:
			last = "last write just now"
		default:
			last = "last write " + shortDuration(d) + " ago"
		}
	}
	lastStyle := m.th.muted
	if r.state == actorStale {
		lastStyle = m.th.warn
	}
	body := []string{
		spread(m.th.actor(r.name).Bold(true).Render(r.name), state, iw),
		truncate(work, iw),
		spread(detail, lastStyle.Render(last), iw),
	}
	border := m.th.p.Overlay
	switch {
	case selected:
		border = m.th.p.Accent
	case r.state == actorStale:
		border = m.th.p.Warn
	}
	style := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(border).
		BorderBackground(m.th.p.Base).Background(m.th.p.Surface).Width(w).Padding(0, 1)
	return style.Render(paintBlock(body, m.th.p.Surface, iw))
}

func (a *activityState) renderEvents(m *Model, w, height int) string {
	label := "all actors"
	if a.only != "" {
		label = "only " + a.only
	}
	lines := []string{spread(m.th.bright.Render("EVENTS")+m.th.muted.Render(" · "+label), m.th.muted.Render("newest first"), w)}
	evs := a.events(m)
	if len(evs) == 0 {
		lines = append(lines, m.th.muted.Render("no events"))
	}
	if a.evScroll > 0 {
		lines = append(lines, m.th.muted.Render(fmt.Sprintf("↑ %d newer · pgup", min(a.evScroll, len(evs)))))
	}
	whoW := 4
	for _, e := range evs {
		whoW = max(whoW, displayWidth(e.Actor))
	}
	whoW = min(whoW, 18)
	for _, e := range evs[min(a.evScroll, len(evs)):] {
		text := m.th.text
		if e.Kind == core.EventClaimExpired {
			text = m.th.warn
		}
		item := e.Item
		if item == "" {
			item = "—"
		}
		prefix := m.th.muted.Render(e.At.In(m.opts.Loc).Format("15:04")) + "  " + m.th.actor(e.Actor).Render(cell(e.Actor, whoW)) +
			"  " + m.th.muted.Render(cell(item, 6)) + "  "
		lines = append(lines, prefix+text.Render(core.DescribeEvent(e)))
	}
	return paintBlock(fitLines(strings.Join(lines, "\n"), w, height), m.th.p.Base, w)
}

// ---- mouse ----

func (a *activityState) click(m *Model, x, y int) tea.Cmd {
	if x >= m.actorsWidth() || y < 2 {
		return nil
	}
	rows := a.actors(m)
	fit := max(1, (m.bodyHeight()-1)/actorCardHeight)
	first := max(0, min(a.cursor-fit+1, len(rows)-fit))
	if i := first + (y-2)/actorCardHeight; i < len(rows) {
		a.cursor = i
	}
	return nil
}

func (a *activityState) wheel(m *Model, x int, up bool) tea.Cmd {
	if x >= m.actorsWidth() {
		if up {
			a.evScroll = max(0, a.evScroll-3)
		} else {
			a.evScroll = min(a.evScroll+3, max(0, len(a.events(m))-1))
		}
		return nil
	}
	if up {
		return a.key(m, "up")
	}
	return a.key(m, "down")
}
