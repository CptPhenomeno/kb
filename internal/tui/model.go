package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/CptPhenomeno/kb/internal/core"
)

// Backend is what the TUI reads. The real one wraps the core service and
// the database; tests use an in-memory fake.
type Backend interface {
	Board(ctx context.Context) (*core.Snapshot, error)
	Log(ctx context.Context, f core.LogFilter) ([]core.Event, error)
	DataVersion(ctx context.Context) (int64, error)
	Move(ctx context.Context, actor, id string, to core.Status, ifVersion int) (*core.MoveResult, error)
	Reorder(ctx context.Context, actor, id string, index, ifVersion int) (*core.Item, error)
	ArrangeColumn(ctx context.Context, actor string, status core.Status, ids []string) error
	SetCriterion(ctx context.Context, actor, id string, n int, done bool, ifVersion int) (*core.Item, error)
	AddCriterion(ctx context.Context, actor, id, text string, ifVersion int) (*core.Item, error)
	EditCriterion(ctx context.Context, actor, id string, n int, text string, ifVersion int) (*core.Item, error)
	Remove(ctx context.Context, actor, id string, recursive bool) ([]string, error)
	Claim(ctx context.Context, actor, id string, ifVersion int) (*core.Item, error)
	Release(ctx context.Context, actor, id string, ifVersion int) (*core.Item, error)
	Create(ctx context.Context, actor string, it core.Item) (*core.Item, error)
	Edit(ctx context.Context, actor, id string, p core.Patch, ifVersion int) (*core.Item, error)
	Link(ctx context.Context, actor string, r core.Relation) error
	Unlink(ctx context.Context, actor string, r core.Relation) error
}

// Options configure the TUI.
type Options struct {
	Project string           // Shown in the header.
	Actor   string           // Who is using the TUI.
	Now     func() time.Time // Clock used for claim expiry.
	Theme   string           // Name of the color theme; "" means the default.
	Loc     *time.Location   // Time zone of the times shown; nil means local.
	// SaveTheme stores the theme chosen in the picker; nil disables saving.
	SaveTheme func(name string) error
	// ShowParents is the saved choice of showing epics and stories; nil
	// means the default (shown).
	ShowParents *bool
	// SaveShowParents stores the p toggle; nil disables saving.
	SaveShowParents func(show bool) error
	// BoardSort is the saved order of the board columns, e.g. "date-desc"
	// (the default), "id-asc", "points-desc" or "manual".
	BoardSort string
	// SaveBoardSort stores the order chosen in the sort popup or set by a
	// manual reorder; nil disables saving.
	SaveBoardSort func(order string) error
}

type viewKind int

const (
	viewBoard viewKind = iota
	viewTree
	viewActivity
	viewCount
)

func (v viewKind) String() string { return [...]string{"Board", "Tree", "Activity"}[v] }

// Minimum terminal size the layout supports.
const minWidth, minHeight = 60, 15

// recentEvents is how many events the TUI keeps for the status line and
// the Activity view.
const recentEvents = 200

// Messages.
type (
	tickMsg    struct{}
	versionMsg struct {
		version int64
		err     error
	}
	loadedMsg struct {
		snap    *core.Snapshot
		recent  []core.Event
		history []core.Event
		version int64
		err     error
	}
	historyMsg struct {
		id     string
		events []core.Event
		err    error
	}
	// writeMsg reports the outcome of a write; alert explains a failure.
	writeMsg struct {
		alert  *alertState
		notice string
	}
)

// Model is the Bubble Tea model of the kb TUI.
type Model struct {
	ctx  context.Context
	be   Backend
	opts Options

	width, height int
	view          viewKind
	help          bool

	snap    *core.Snapshot
	recent  []core.Event
	version int64
	loaded  bool
	err     error

	// tickEvery is the polling period of the live refresh; 0 disables it.
	tickEvery time.Duration

	board    boardState
	tree     treeState
	activity activityState

	detailOpen bool
	history    []core.Event
	md         *markdown

	th         *theme
	themeIdx   int
	picker     *pickerState  // open theme picker, or nil
	alert      *alertState   // open alert, or nil
	filterOpen bool          // filter popup open
	palette    *paletteState // open command palette, or nil
	accept     *acceptState  // open acceptance popup, or nil
	confirm    *confirmState // open delete confirmation, or nil
	links      *linksState   // open links popup, or nil
	form       *formState    // open create/edit form, or nil
	sort       *sortState    // open sort popup, or nil
	pendingSel string        // item to select after the next reload
	editing    string        // item whose description is open in $EDITOR

	detailScroll   int    // first detail line shown
	detailScrollID string // item the scroll refers to
	notice         string // shown in the status line until the next key
}

// New returns the model; run it with Run or a tea.Program.
func New(ctx context.Context, be Backend, opts Options) *Model {
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.Loc == nil {
		opts.Loc = time.Local
	}
	m := &Model{ctx: ctx, be: be, opts: opts, tickEvery: 300 * time.Millisecond, md: newMarkdown()}
	if opts.Theme != "" {
		if i := themeIndex(opts.Theme); i >= 0 {
			m.themeIdx = i
		} else {
			m.notice = fmt.Sprintf("unknown theme %q, using %s (themes: %s)", opts.Theme, Themes[0].Name, strings.Join(ThemeNames(), ", "))
		}
	}
	m.th = newTheme(Themes[m.themeIdx])
	m.board.types = defaultTypes()
	m.board.sort = parseBoardSort(opts.BoardSort)
	if opts.ShowParents != nil && !*opts.ShowParents {
		m.board.types[core.TypeEpic], m.board.types[core.TypeStory] = false, false
	}
	return m
}

// Init loads the board and starts the live refresh.
func (m *Model) Init() tea.Cmd { return tea.Batch(m.load(), m.tick()) }

func (m *Model) tick() tea.Cmd {
	if m.tickEvery == 0 {
		return nil
	}
	return tea.Tick(m.tickEvery, func(time.Time) tea.Msg { return tickMsg{} })
}

// checkVersion asks whether another process wrote to the board.
func (m *Model) checkVersion() tea.Cmd {
	return func() tea.Msg {
		v, err := m.be.DataVersion(m.ctx)
		return versionMsg{version: v, err: err}
	}
}

// load reads the board, the recent events and the open item's history. The
// version is read first, so no write can slip between it and the data.
func (m *Model) load() tea.Cmd {
	detailID := ""
	if m.detailOpen {
		detailID = m.selectedID()
	}
	return func() tea.Msg {
		var msg loadedMsg
		if msg.version, msg.err = m.be.DataVersion(m.ctx); msg.err != nil {
			return msg
		}
		if msg.snap, msg.err = m.be.Board(m.ctx); msg.err != nil {
			return msg
		}
		if msg.recent, msg.err = m.be.Log(m.ctx, core.LogFilter{Limit: recentEvents}); msg.err != nil {
			return msg
		}
		if detailID != "" {
			msg.history, msg.err = m.be.Log(m.ctx, core.LogFilter{Item: detailID})
		}
		return msg
	}
}

func (m *Model) loadHistory(id string) tea.Cmd {
	return func() tea.Msg {
		evs, err := m.be.Log(m.ctx, core.LogFilter{Item: id})
		return historyMsg{id: id, events: evs, err: err}
	}
}

// Update handles a message.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.board.ensureVisible(m)
		m.tree.ensureVisible(m)
	case tickMsg:
		return m, tea.Batch(m.checkVersion(), m.tick())
	case versionMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		if !m.loaded || msg.version != m.version {
			return m, m.load()
		}
	case loadedMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		prevBoard, prevTree := m.board.selectedID(m), m.tree.selectedID(m)
		if m.pendingSel != "" {
			switch m.view {
			case viewTree:
				prevTree = m.pendingSel
			default:
				prevBoard = m.pendingSel
			}
			m.pendingSel = ""
		}
		m.snap, m.recent, m.version, m.loaded, m.err = msg.snap, msg.recent, msg.version, true, nil
		if m.detailOpen {
			m.history = msg.history
		}
		m.tree.reveal(m, prevTree)
		m.board.restore(m, prevBoard)
		m.tree.restore(m, prevTree)
	case writeMsg:
		m.alert = msg.alert
		if msg.notice != "" {
			m.notice = msg.notice
		}
		return m, m.load()
	case formDoneMsg:
		if m.form == nil {
			return m, nil
		}
		m.form.saving = false
		if msg.id == "" {
			m.form.errs, m.form.general = msg.errs, msg.general
			if m.form.errs == nil {
				m.form.errs = map[int]string{}
			}
			return m, nil
		}
		m.form, m.pendingSel = nil, msg.id
		return m, m.load()
	case editorDoneMsg:
		return m, m.editorDone(msg)
	case historyMsg:
		if msg.err == nil && msg.id == m.selectedID() {
			m.history = msg.events
		}
	case tea.KeyPressMsg:
		return m, m.handleKey(msg)
	case tea.MouseClickMsg:
		switch {
		case m.help:
		case m.view == viewBoard:
			return m, m.board.click(m, msg.X, msg.Y)
		case m.view == viewTree:
			return m, m.tree.click(m, msg.X, msg.Y)
		case m.view == viewActivity:
			return m, m.activity.click(m, msg.X, msg.Y)
		}
	case tea.MouseWheelMsg:
		switch {
		case m.help:
		case m.view == viewBoard:
			return m, m.board.wheel(m, msg.X, msg.Button == tea.MouseWheelUp)
		case m.view == viewTree:
			return m, m.tree.wheel(m, msg.X, msg.Button == tea.MouseWheelUp)
		case m.view == viewActivity:
			return m, m.activity.wheel(m, msg.X, msg.Button == tea.MouseWheelUp)
		}
	}
	return m, nil
}

func (m *Model) handleKey(k tea.KeyPressMsg) tea.Cmd {
	m.notice = ""
	return m.dispatch(k.String(), k.Text)
}

// dispatch handles a key given by name; the palette runs actions through it.
func (m *Model) dispatch(key, text string) tea.Cmd {
	if key == "ctrl+c" {
		return tea.Quit
	}
	if m.alert != nil {
		if key == "enter" || key == "esc" {
			m.alert = nil
		}
		return nil
	}
	if m.picker != nil {
		return m.pickerKey(key)
	}
	if m.palette != nil {
		return m.paletteKey(key, text)
	}
	if m.accept != nil {
		return m.acceptKey(key, text)
	}
	if m.confirm != nil {
		return m.confirmKey(key)
	}
	if m.links != nil {
		return m.linksKey(key, text)
	}
	if m.form != nil {
		return m.formKey(key, text)
	}
	if m.sort != nil {
		return m.sortKey(key)
	}
	if m.filterOpen {
		return m.filterKey(key)
	}
	if m.board.searching && m.view != viewActivity {
		return m.searchKey(key, text)
	}
	switch key {
	case "ctrl+c", "q":
		return tea.Quit
	case "?":
		m.help = !m.help
		return nil
	case "ctrl+p", "ctrl+shift+p":
		m.palette = &paletteState{}
		return nil
	case "esc":
		switch {
		case m.help:
			m.help = false
		case m.view == viewActivity:
			return m.activity.key(m, key)
		case m.detailOpen:
			m.detailOpen = false
		}
		return nil
	case "t":
		m.openPicker()
		return nil
	case "tab":
		m.view = (m.view + 1) % viewCount
		return nil
	case "shift+tab":
		m.view = (m.view + viewCount - 1) % viewCount
		return nil
	}
	if m.help || !m.loaded {
		return nil
	}
	switch m.view {
	case viewBoard:
		return m.board.key(m, key)
	case viewTree:
		return m.tree.key(m, key)
	case viewActivity:
		return m.activity.key(m, key)
	}
	return nil
}

// setTheme switches to the theme at index i.
func (m *Model) setTheme(i int) {
	m.themeIdx = i
	m.th = newTheme(Themes[i])
	m.md.reset()
}

// View renders the whole screen.
func (m *Model) View() tea.View {
	content := paint(m.render(), m.th.p.Base, m.width, m.height)
	if m.width >= minWidth && m.height >= minHeight {
		popup := ""
		switch {
		case m.alert != nil:
			popup = m.renderAlert()
		case m.picker != nil:
			popup = m.renderPicker()
		case m.filterOpen:
			popup = m.renderFilter()
		case m.palette != nil:
			popup = m.renderPalette()
		case m.accept != nil && m.snap.Get(m.accept.id) != nil:
			popup = m.renderAccept()
		case m.confirm != nil:
			popup = m.renderConfirm()
		case m.links != nil && m.snap.Get(m.links.id) != nil:
			popup = m.renderLinks()
		case m.form != nil:
			popup = m.renderForm()
		case m.sort != nil:
			popup = m.renderSort()
		}
		if popup != "" {
			content = paint(overlay(content, popup, m.width, m.height), m.th.p.Base, m.width, m.height)
		}
	}
	v := tea.NewView(content)
	v.BackgroundColor = m.th.p.Base
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	v.WindowTitle = "kb — " + m.opts.Project
	return v
}

func (m *Model) render() string {
	if m.width == 0 {
		return ""
	}
	if m.width < minWidth || m.height < minHeight {
		return fmt.Sprintf("Terminal too small: kb needs at least %d×%d (now %d×%d).", minWidth, minHeight, m.width, m.height)
	}
	bar := m.commandBar()
	bodyH := m.height - 2 - len(bar)
	var body string
	switch {
	case !m.loaded && m.err == nil:
		body = m.th.muted.Render("Loading the board…")
	case m.err != nil && !m.loaded:
		body = m.th.warn.Render("Cannot read the board: " + m.err.Error())
	case m.help:
		body = m.renderHelp(bodyH)
	case m.view == viewBoard:
		body = m.board.render(m, bodyH)
	case m.view == viewTree:
		body = m.tree.render(m, bodyH)
	default:
		body = m.activity.render(m, bodyH)
	}
	lines := []string{m.header()}
	lines = append(lines, fitLines(body, m.width, bodyH)...)
	lines = append(lines, m.statusLine())
	lines = append(lines, bar...)
	return strings.Join(lines, "\n")
}

// header renders the top line: project, views, filters and counters.
func (m *Model) header() string {
	tabs := make([]string, 0, viewCount)
	for v := viewBoard; v < viewCount; v++ {
		if v == m.view {
			tabs = append(tabs, m.th.tabOn.Render(" "+v.String()+" "))
		} else {
			tabs = append(tabs, m.th.tabOff.Render(" "+v.String()+" "))
		}
	}
	left := m.th.logo.Render(" kb ") + " " + m.th.bright.Render(m.opts.Project) + "  " + strings.Join(tabs, "")
	switch m.view {
	case viewBoard:
		left += "  " + m.th.muted.Render(m.board.filterLabel())
	case viewTree:
		left += "  " + m.th.muted.Render(m.tree.filterLabel(m))
	case viewActivity:
		left += "  " + m.th.muted.Render(m.activity.filterLabel())
	}
	right := ""
	if m.snap != nil {
		agents, stale := m.claimCounts()
		right = m.th.muted.Render(fmt.Sprintf("%d items", len(m.snap.Items())))
		if agents > 0 {
			right += "  " + m.th.fg(m.th.p.Agent).Render(plural(agents, "agent active", "agents active"))
		}
		if stale > 0 {
			right += "  " + m.th.warn.Render(fmt.Sprintf("%d stale", stale))
		}
	}
	if m.err != nil && m.loaded {
		right = m.th.warn.Render("refresh failed: "+m.err.Error()) + "  " + right
	}
	return spread(left, right, m.width)
}

// claimCounts returns the agents holding an active claim and the number
// of stale claims.
func (m *Model) claimCounts() (agents, stale int) {
	now := m.opts.Now()
	seen := map[string]bool{}
	for _, it := range m.snap.Items() {
		if core.ClaimState(it, now) == core.ClaimActive && it.Status != core.StatusDone {
			if a := it.Claim.Actor; !core.IsHumanActor(a) && !seen[a] {
				seen[a] = true
				agents++
			}
		}
		if core.IsStale(it, now) {
			stale++
		}
	}
	return agents, stale
}

// statusLine shows the latest event.
func (m *Model) statusLine() string {
	if m.board.searching && m.view != viewActivity {
		n := m.board.shownCount(m)
		if m.view == viewTree {
			n = m.tree.matchCount(m)
		}
		return m.th.fg(m.th.p.Accent).Render("/ ") + m.th.bright.Render(m.board.query) + m.th.fg(m.th.p.Accent).Render("▌") +
			"  " + m.th.muted.Render(plural(n, "match", "matches"))
	}
	if m.notice != "" {
		return truncate(m.th.fg(m.th.p.Accent).Render(m.notice), m.width)
	}
	if len(m.recent) == 0 {
		return m.th.muted.Render("no activity yet")
	}
	e := m.recent[len(m.recent)-1]
	line := m.th.muted.Render(e.At.In(m.opts.Loc).Format("15:04")) + "  " + m.th.actor(e.Actor).Render(e.Actor) + "  " +
		m.th.muted.Render(e.Item) + "  " + m.th.text.Render(core.DescribeEvent(e))
	return truncate(line, m.width)
}

// keyHint is one entry of the command bar and of the help. soon marks a
// key whose feature has not landed yet: it is not advertised.
type keyHint struct {
	key, label, help string
	soon             bool
}

var globalKeys = []keyHint{
	{"tab", "next view", "switch to the next view (shift+tab: previous)", false},
	{"ctrl+p", "commands", "open the command palette (ctrl+shift+p too, where the terminal lets it through)", false},
	{"t", "theme", "choose the color theme, with a live preview (saved for your user)", false},
	{"?", "help", "show or hide this help", false},
	{"q", "quit", "quit kb", false},
}

var viewKeys = map[viewKind][]keyHint{
	viewBoard: {
		{"← → ↑ ↓", "navigate", "move between columns and cards", false},
		{"shift ← →", "move", "move to the previous or next column", false},
		{"shift ↑ ↓", "reorder", "reorder the card in its column, starting from the order shown (switches to manual order)", false},
		{"s", "sort", "sort the columns by date, ID or story points, ascending or descending (saved for your user)", false},
		{"enter", "details", "open the detail panel (esc closes it, pgup/pgdown scroll it)", false},
		{"n", "new", "create an item", false},
		{"e", "edit", "edit the selected item", false},
		{"d", "delete", "delete the selected item", false},
		{"c", "check", "check the acceptance criteria of the selected item", false},
		{"ctrl+e", "describe", "edit the description of the selected item in $EDITOR", false},
		{"a", "assign", "claim or release the selected item", false},
		{"l", "link", "manage relations", false},
		{"p", "epics & stories", "show or hide epics and stories (saved for your user)", false},
		{"/", "search", "search by ID, title, label or assignee", false},
		{"f", "filter", "choose the types shown, or only blocked or stale items", false},
	},
	viewTree: {
		{"↑ ↓", "navigate", "move between rows", false},
		{"← →", "collapse / expand", "collapse or expand a branch (on a leaf, ← goes to the parent)", false},
		{"enter", "details", "open the detail panel (esc closes it, pgup/pgdown scroll it)", false},
		{"n", "new child", "create a child of the selected item, with the type the hierarchy suggests", false},
		{"e", "edit", "edit the selected item", false},
		{"d", "delete", "delete the selected item", false},
		{"c", "check", "check the acceptance criteria of the selected item", false},
		{"ctrl+e", "describe", "edit the description of the selected item in $EDITOR", false},
		{"a", "assign", "claim or release the selected item", false},
		{"l", "link", "manage relations", false},
		{"/", "search", "search by ID, title, label or assignee; ancestors of a match stay shown", false},
		{"f", "filter", "only blocked or stale items, with their ancestors (types apply to the Board only)", false},
	},
	viewActivity: {
		{"↑ ↓", "select actor", "select an actor", false},
		{"enter", "only this actor", "show only the selected actor's events", false},
		{"esc", "all actors", "show every actor's events", false},
		{"o", "open item", "open the item the actor works on, on the Board with its detail", false},
		{"a", "release claim", "release the actor's claim (useful when stale)", false},
		{"pgup pgdn", "scroll events", "scroll the event stream", false},
	},
}

// commandBar renders the key hints of the current view, wrapped to width.
func (m *Model) commandBar() []string {
	hints := ready(append(append([]keyHint{}, viewKeys[m.view]...), globalKeys...))
	if m.picker != nil {
		hints = pickerKeys
	}
	switch {
	case m.alert != nil:
		hints = alertKeys
	case m.filterOpen:
		hints = filterKeys
	case m.palette != nil:
		hints = paletteKeys
	case m.accept != nil:
		hints = acceptKeys
	case m.confirm != nil:
		hints = confirmKeys
	case m.links != nil && m.links.adding:
		hints = linkAddKeys
	case m.links != nil:
		hints = linksKeys
	case m.form != nil:
		hints = formKeys
	case m.sort != nil:
		hints = sortKeys
	case m.board.searching && m.view != viewActivity:
		hints = searchKeys
	}
	var lines []string
	line, lineW := "", 0
	for _, h := range hints {
		chip := m.th.chip.Render(h.key) + " " + m.th.muted.Render(h.label)
		w := lipgloss.Width(chip)
		if lineW > 0 && lineW+3+w > m.width {
			lines = append(lines, line)
			line, lineW = "", 0
		}
		if lineW > 0 {
			line += "   "
			lineW += 3
		}
		line += chip
		lineW += w
	}
	return append(lines, line)
}

func (m *Model) renderHelp(height int) string {
	var b strings.Builder
	b.WriteString(m.th.bright.Render("Keys") + m.th.muted.Render("  ("+m.view.String()+" view; ? or esc closes)") + "\n\n")
	for _, h := range ready(append(append([]keyHint{}, viewKeys[m.view]...), globalKeys...)) {
		fmt.Fprintf(&b, "  %s  %s\n", m.th.chip.Render(fmt.Sprintf("%-12s", h.key)), m.th.text.Render(h.help))
	}
	return b.String()
}

// ready drops the hints of features that have not landed yet.
func ready(hints []keyHint) []keyHint {
	out := hints[:0]
	for _, h := range hints {
		if !h.soon {
			out = append(out, h)
		}
	}
	return out
}

func plural(n int, one, many string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, one)
	}
	return fmt.Sprintf("%d %s", n, many)
}
