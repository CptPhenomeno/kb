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
	RemoveCriterion(ctx context.Context, actor, id string, n int, ifVersion int) (*core.Item, error)
	SetTestState(ctx context.Context, actor, id, selector string, state core.TestState, ifVersion int) (*core.Item, error)
	AddTest(ctx context.Context, actor, id, name string, ifVersion int) (*core.Item, error)
	EditTest(ctx context.Context, actor, id, selector, name string, ifVersion int) (*core.Item, error)
	RemoveTest(ctx context.Context, actor, id, selector string, ifVersion int) (*core.Item, error)
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
	helpScroll    int // first help line shown
	helpTab       int // active help tab: 0 is the current view, then helpTabNames

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
	detailID       string // item shown in the detail popup
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
		detailID = m.detailID
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
			if m.snap.Get(m.detailID) == nil {
				m.detailOpen = false
				m.notice = m.detailID + " was deleted"
			} else {
				m.history = msg.history
			}
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
	case testSavedMsg:
		if m.accept != nil && msg.err != "" {
			m.accept.err = msg.err
			return m, nil
		}
		if m.accept != nil {
			m.accept.mode, m.accept.err = inputNone, ""
		}
		m.alert = msg.alert
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
			for field := range m.form.errs { // show the tab holding the error
				if m.form.editID == "" && tabOf(field) != m.form.tab {
					m.form.show(tabOf(field))
				}
				break
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
		case m.detailOpen && m.popupOpen():
		case m.detailOpen:
			m.detailClick(msg.X, msg.Y)
		case m.help:
			m.helpClick(msg.X, msg.Y)
		case m.view == viewBoard:
			return m, m.board.click(m, msg.X, msg.Y)
		case m.view == viewTree:
			return m, m.tree.click(m, msg.X, msg.Y)
		case m.view == viewActivity:
			return m, m.activity.click(m, msg.X, msg.Y)
		}
	case tea.MouseWheelMsg:
		switch {
		case m.detailOpen && m.popupOpen():
		case m.detailOpen:
			m.scrollDetailBy(wheelStep(msg.Button == tea.MouseWheelUp))
		case m.help:
			m.scrollHelpBy(wheelStep(msg.Button == tea.MouseWheelUp))
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

// popupOpen reports whether a popup is drawn over the view, and over the
// detail when it is open: it owns the mouse as well as the keyboard.
func (m *Model) popupOpen() bool {
	return m.alert != nil || m.picker != nil || m.palette != nil || m.accept != nil || m.confirm != nil ||
		m.links != nil || m.form != nil || m.sort != nil || m.filterOpen
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
	if m.detailOpen {
		return m.detailKey(key)
	}
	if m.help {
		return m.helpKey(key)
	}
	switch key {
	case "ctrl+c", "q":
		return tea.Quit
	case "?":
		m.openHelp()
		return nil
	case "ctrl+p", "ctrl+shift+p":
		m.palette = &paletteState{}
		return nil
	case "esc":
		if m.view == viewActivity {
			return m.activity.key(m, key)
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
	if !m.loaded {
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
		if m.detailOpen && !m.help && m.loaded && (m.view == viewBoard || m.view == viewTree) {
			w, h := m.detailSize()
			content = paint(overlay(content, m.renderDetail(w, h), m.width, m.height), m.th.p.Base, m.width, m.height)
		}
		popup, isForm := "", false
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
			popup, isForm = m.renderForm(), true
		case m.sort != nil:
			popup = m.renderSort()
		}
		if popup != "" && isForm {
			content = paint(m.overlayBody(content, popup), m.th.p.Base, m.width, m.height)
		} else if popup != "" {
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

var detailKeys = []keyHint{
	{"↑ ↓", "scroll", "scroll the detail by one line (the mouse wheel too)", false},
	{"pgup pgdn", "page", "scroll the detail by one page", false},
	{"e", "edit", "edit the item shown", false},
	{"ctrl+e", "describe", "edit its description in $EDITOR", false},
	{"c", "criteria", "manage its acceptance criteria and tests", false},
	{"a", "assign", "claim or release it", false},
	{"l", "link", "manage its relations", false},
	{"esc/enter", "close", "close the detail popup (a click outside closes it too)", false},
}

var helpKeys = []keyHint{
	{"tab →", "next tab", "show the next help tab", false},
	{"shift+tab ←", "previous tab", "show the previous help tab", false},
	{"↑ ↓", "scroll", "scroll the tab by one line (the mouse wheel too)", false},
	{"pgup pgdn", "page", "scroll the tab by one page", false},
	{"?/esc", "close", "close the help", false},
}

var viewKeys = map[viewKind][]keyHint{
	viewBoard: {
		{"← → ↑ ↓", "navigate", "move between columns and cards", false},
		{"shift ← →", "move", "move to the previous or next column", false},
		{"shift ↑ ↓", "reorder", "reorder the card in its column, starting from the order shown (switches to manual order)", false},
		{"s", "sort", "sort the columns by date, ID or story points, ascending or descending (saved for your user)", false},
		{"enter", "details", "open the detail popup", false},
		{"n", "new", "create an item", false},
		{"e", "edit", "edit the selected item", false},
		{"d", "delete", "delete the selected item", false},
		{"c", "criteria", "manage the acceptance criteria and tests of the selected item", false},
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
		{"enter", "details", "open the detail popup", false},
		{"n", "new child", "create a child of the selected item, with the type the hierarchy suggests", false},
		{"e", "edit", "edit the selected item", false},
		{"d", "delete", "delete the selected item", false},
		{"c", "criteria", "manage the acceptance criteria and tests of the selected item", false},
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
		{"o", "open item", "open the item the actor works on, on the Board with its detail popup", false},
		{"a", "release claim", "release the actor's claim (useful when stale)", false},
		{"pgup pgdn", "scroll events", "scroll the event stream", false},
	},
}

// commandBar renders the key hints of the current view, wrapped to width.
func (m *Model) commandBar() []string {
	viewHints := ready(append(append([]keyHint{}, viewKeys[m.view]...), globalKeys...))
	hints := viewHints
	if m.picker != nil {
		hints = pickerKeys
	}
	switch {
	case m.help:
		hints = helpKeys
	case m.alert != nil:
		hints = alertKeys
	case m.filterOpen:
		hints = filterKeys
	case m.palette != nil:
		hints = paletteKeys
	case m.accept != nil:
		hints = m.accept.hints()
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
	case m.detailOpen && !m.popupOpen():
		hints = detailKeys
	case m.board.searching && m.view != viewActivity:
		hints = searchKeys
	}
	lines := m.wrapHints(hints)
	if m.help || m.detailOpen && !m.popupOpen() {
		// the view under the help or the detail keeps its size: the bar keeps its height
		for n := len(m.wrapHints(viewHints)); len(lines) < n; {
			lines = append(lines, "")
		}
	}
	return lines
}

// wrapHints renders key hints as chips, wrapped to the width of the screen.
func (m *Model) wrapHints(hints []keyHint) []string {
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

// helpTabNames are the help tabs after the one of the current view.
var helpTabNames = []string{"Detail", "Criteria", "Form"}

// openHelp shows the help on the tab of the current view.
func (m *Model) openHelp() {
	m.help = true
	m.setHelpTab(0)
}

// helpKey handles a key while the help is open: tab and shift+tab (or the
// arrows) change the help tab, ? and esc close it, the rest does nothing.
func (m *Model) helpKey(key string) tea.Cmd {
	n := len(helpTabNames) + 1
	switch key {
	case "tab", "right":
		m.setHelpTab((m.helpTab + 1) % n)
	case "shift+tab", "left":
		m.setHelpTab((m.helpTab + n - 1) % n)
	case "up":
		m.scrollHelpBy(-1)
	case "down":
		m.scrollHelpBy(1)
	case "pgup":
		m.scrollHelpBy(-helpWindow(m.helpContentHeight()))
	case "pgdown":
		m.scrollHelpBy(helpWindow(m.helpContentHeight()))
	case "?", "esc":
		m.help = false
	}
	return nil
}

// helpHints returns the title, the opening key and the hints of a help tab.
func (m *Model) helpHints(tab int) (title, opens string, hints []keyHint) {
	switch tab {
	case 0:
		return m.view.String() + " view", "", ready(append(append([]keyHint{}, viewKeys[m.view]...), globalKeys...))
	case 1:
		return "Detail popup", "enter", detailKeys
	case 2:
		return "Criteria and tests popup", "c", append(append(append([]keyHint{}, acceptKeys...), acceptInputKeys...), acceptConfirmKeys...)
	}
	return "Create and edit form", "n, e", formKeys
}

// helpWindow is the number of help lines shown at once in the given height
// when the tab needs scrolling.
func helpWindow(height int) int { return scrollWindowSize(height) }

// helpContentHeight is the room the scrolling part of the help has: the body
// minus the tab bar and the blank line under it.
func (m *Model) helpContentHeight() int { return m.bodyHeight() - 2 }

// helpHintCol is the width of the indent and the key chip before a help text.
const helpHintCol = 16

// helpLines is the content of the active help tab below the tab bar, wrapped
// to width cells.
func (m *Model) helpLines(width int) []string {
	title, opens, hints := m.helpHints(m.helpTab)
	head := m.th.bright.Render(title)
	if opens != "" {
		head += m.th.muted.Render("  (" + opens + ")")
	}
	lines := []string{head, ""}
	for _, h := range hints {
		prefix := "  " + m.th.chip.Render(fmt.Sprintf("%-12s", h.key)) + "  "
		lines = append(lines, hang(prefix, helpHintCol, h.help, m.th.text, width, helpHintCol)...)
	}
	return lines
}

// helpTabBar renders the help tab bar and returns the x range of each label.
func (m *Model) helpTabBar() (bar string, spans [][2]int) {
	names := append([]string{m.view.String()}, helpTabNames...)
	x := 0
	for i, n := range names {
		style := m.th.tabOff
		if i == m.helpTab {
			style = m.th.tabOn
		}
		label := style.Render(" " + n + " ")
		w := lipgloss.Width(label)
		spans = append(spans, [2]int{x, x + w})
		x += w
		bar += label
	}
	return bar + m.th.muted.Render("  tab or ← → switch · ? or esc closes"), spans
}

// setHelpTab opens a help tab from its top.
func (m *Model) setHelpTab(tab int) {
	m.helpTab, m.helpScroll = tab, 0
}

// scrollHelpBy scrolls the help tab by delta lines, never above the top or
// past the last page; a tab that fits does not scroll.
func (m *Model) scrollHelpBy(delta int) {
	total, height := len(m.helpLines(m.width)), m.helpContentHeight()
	if total <= height {
		m.helpScroll = 0
		return
	}
	m.helpScroll = max(0, min(m.helpScroll+delta, total-helpWindow(height)))
}

// helpClick opens the help tab whose label is at (x, y) of the screen; a
// click anywhere else does nothing.
func (m *Model) helpClick(x, y int) {
	if y != 1 {
		return
	}
	_, spans := m.helpTabBar()
	for i, s := range spans {
		if x >= s[0] && x < s[1] {
			m.setHelpTab(i)
			return
		}
	}
}

func (m *Model) renderHelp(height int) string {
	bar, _ := m.helpTabBar()
	body, scroll := m.scrollWindow(m.helpLines(m.width), m.helpScroll, height-2, m.width)
	m.helpScroll = scroll
	return strings.Join(append([]string{bar, ""}, body...), "\n")
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
