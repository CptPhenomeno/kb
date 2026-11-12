package tui

import (
	"context"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/CptPhenomeno/kb/internal/core"
)

var update = flag.Bool("update", false, "rewrite golden files")

var testNow = time.Date(2026, 11, 9, 14, 4, 0, 0, time.UTC)

// fakeBackend serves a board from memory and counts the reloads.
type fakeBackend struct {
	snap       *core.Snapshot
	events     []core.Event
	version    int64
	boardCalls int
	adds       []addCall    // AddCriterion calls, in order
	addErr     error        // returned by AddCriterion
	edits      []editCall   // EditCriterion calls, in order
	editErr    error        // returned by EditCriterion
	removes    []removeCall // RemoveCriterion calls, in order
	removeErr  error        // returned by RemoveCriterion
}

// editCall is one recorded EditCriterion call.
type editCall struct {
	id      string
	n       int
	text    string
	version int
}

// removeCall is one recorded RemoveCriterion call.
type removeCall struct {
	id      string
	n       int
	version int
}

// addCall is one recorded AddCriterion call.
type addCall struct {
	id, text string
	version  int
}

func (f *fakeBackend) Board(context.Context) (*core.Snapshot, error) {
	f.boardCalls++
	return f.snap, nil
}

func (f *fakeBackend) Log(_ context.Context, flt core.LogFilter) ([]core.Event, error) {
	var out []core.Event
	for _, e := range f.events {
		if flt.Item == "" || e.Item == flt.Item {
			out = append(out, e)
		}
	}
	if flt.Limit > 0 && len(out) > flt.Limit {
		out = out[len(out)-flt.Limit:]
	}
	return out, nil
}

func (f *fakeBackend) DataVersion(context.Context) (int64, error) { return f.version, nil }

func (f *fakeBackend) Move(context.Context, string, string, core.Status, int) (*core.MoveResult, error) {
	return nil, errors.New("read-only fake")
}

func (f *fakeBackend) SetCriterion(context.Context, string, string, int, bool, int) (*core.Item, error) {
	return nil, errors.New("read-only fake")
}

func (f *fakeBackend) AddCriterion(_ context.Context, _, id, text string, ifVersion int) (*core.Item, error) {
	f.adds = append(f.adds, addCall{id, text, ifVersion})
	return nil, f.addErr
}

func (f *fakeBackend) EditCriterion(_ context.Context, _, id string, n int, text string, ifVersion int) (*core.Item, error) {
	f.edits = append(f.edits, editCall{id, n, text, ifVersion})
	return nil, f.editErr
}

func (f *fakeBackend) RemoveCriterion(_ context.Context, _, id string, n int, ifVersion int) (*core.Item, error) {
	f.removes = append(f.removes, removeCall{id, n, ifVersion})
	return nil, f.removeErr
}

func (f *fakeBackend) SetTestState(context.Context, string, string, string, core.TestState, int) (*core.Item, error) {
	return nil, errors.New("read-only fake")
}

func (f *fakeBackend) AddTest(context.Context, string, string, string, int) (*core.Item, error) {
	return nil, errors.New("read-only fake")
}

func (f *fakeBackend) EditTest(context.Context, string, string, string, string, int) (*core.Item, error) {
	return nil, f.editErr
}

func (f *fakeBackend) RemoveTest(context.Context, string, string, string, int) (*core.Item, error) {
	return nil, errors.New("read-only fake")
}

func (f *fakeBackend) Remove(context.Context, string, string, bool) ([]string, error) {
	return nil, errors.New("read-only fake")
}

func (f *fakeBackend) Claim(context.Context, string, string, int) (*core.Item, error) {
	return nil, errors.New("read-only fake")
}

func (f *fakeBackend) Release(context.Context, string, string, int) (*core.Item, error) {
	return nil, errors.New("read-only fake")
}

func (f *fakeBackend) Link(context.Context, string, core.Relation) error {
	return errors.New("read-only fake")
}

func (f *fakeBackend) Unlink(context.Context, string, core.Relation) error {
	return errors.New("read-only fake")
}

func (f *fakeBackend) Create(context.Context, string, core.Item) (*core.Item, error) {
	return nil, errors.New("read-only fake")
}

func (f *fakeBackend) Edit(context.Context, string, string, core.Patch, int) (*core.Item, error) {
	return nil, errors.New("read-only fake")
}

func (f *fakeBackend) ArrangeColumn(context.Context, string, core.Status, []string) error {
	return errors.New("read-only fake")
}

func (f *fakeBackend) Reorder(context.Context, string, string, int, int) (*core.Item, error) {
	return nil, errors.New("read-only fake")
}

// item builds a board item for the fixtures.
func item(id string, typ core.Type, status core.Status, parent, title string, opts ...func(*core.Item)) *core.Item {
	it := &core.Item{ID: id, Type: typ, Status: status, Parent: parent, Title: title}
	for _, o := range opts {
		o(it)
	}
	return it
}

func pos(p int) func(*core.Item)             { return func(it *core.Item) { it.Position = p } }
func prio(p core.Priority) func(*core.Item)  { return func(it *core.Item) { it.Priority = p } }
func points(p int) func(*core.Item)          { return func(it *core.Item) { it.Points = p } }
func phase(p core.TDDPhase) func(*core.Item) { return func(it *core.Item) { it.Phase = p } }
func tests(states ...core.TestState) func(*core.Item) {
	return func(it *core.Item) {
		for i, s := range states {
			it.Tests = append(it.Tests, core.Test{Name: "test " + string(rune('a'+i)), State: s})
		}
	}
}
func claimedBy(actor string, expires time.Time) func(*core.Item) {
	return func(it *core.Item) {
		it.Assignee = actor
		it.Claim = &core.Claim{Actor: actor, ExpiresAt: expires}
	}
}

// sampleBoard mirrors the mockup in design/project/Main.dc.html.
func sampleBoard() *fakeBackend {
	items := []*core.Item{
		item("KB-1", core.TypeEpic, core.StatusDoing, "", "Authentication"),
		item("KB-2", core.TypeStory, core.StatusDoing, "KB-1", "Token login"),
		item("KB-3", core.TypeTask, core.StatusReview, "KB-2", "Validate token in middleware", prio(core.PriorityHigh), points(3),
			phase(core.PhaseRefactor), tests(core.TestPassing, core.TestPassing, core.TestPassing), claimedBy("agent-auth", testNow.Add(-time.Hour))),
		item("KB-4", core.TypeTask, core.StatusDoing, "KB-2", "Token refresh", prio(core.PriorityHigh), points(2),
			phase(core.PhaseRed), tests(core.TestFailing, core.TestPending), claimedBy("agent-auth", testNow.Add(12*time.Minute)),
			func(it *core.Item) {
				it.Description = "Exchange a valid refresh token for a new token pair.\n\nThe old refresh token is **revoked** on use."
				it.Spec = "SPEC.md#token-refresh"
				it.Acceptance = []core.Criterion{{Text: "returns a new access + refresh pair"}, {Text: "reused refresh token → 401"}}
			}),
		item("KB-5", core.TypeBug, core.StatusTodo, "KB-2", "Expired token returns 500 instead of 401", prio(core.PriorityHigh), points(1),
			tests(core.TestPending)),
		item("KB-6", core.TypeStory, core.StatusTodo, "", "API keys per tenant"),
		item("KB-7", core.TypeTask, core.StatusTodo, "KB-6", "Key storage schema", prio(core.PriorityHigh), points(3), pos(1)),
		item("KB-8", core.TypeTask, core.StatusTodo, "KB-6", "Key rotation endpoint", points(3), pos(2)),
		item("KB-12", core.TypeTask, core.StatusDoing, "", "Status code counters", points(2), pos(1),
			phase(core.PhaseGreen), tests(core.TestPassing, core.TestPassing, core.TestPending),
			claimedBy("agent-metrics", testNow.Add(-22*time.Minute))),
		item("KB-13", core.TypeSubtask, core.StatusDoing, "KB-12", "Bounded route label"),
		item("KB-15", core.TypeTask, core.StatusBacklog, "", "Load test the gateway with k6", points(3)),
		item("KB-17", core.TypeTask, core.StatusDone, "", "Health check endpoint", prio(core.PriorityLow), points(1),
			tests(core.TestPassing, core.TestPassing)),
	}
	rels := []core.Relation{{Kind: core.RelBlocks, From: "KB-7", To: "KB-8"}, {Kind: core.RelRelates, From: "KB-4", To: "KB-5"}}
	at := func(m int) time.Time { return testNow.Add(time.Duration(m-4) * time.Minute) }
	events := []core.Event{
		{ID: 1, At: at(2), Actor: "agent-auth", Item: "KB-4", Kind: core.EventClaimed},
		{ID: 2, At: at(3), Actor: "agent-auth", Item: "KB-4", Kind: core.EventPhase, Data: map[string]any{"from": "", "to": "red"}},
		{ID: 3, At: at(4), Actor: "agent-auth", Item: "KB-4", Kind: core.EventTest, Data: map[string]any{"name": "refresh rotates the token", "state": "failing"}},
	}
	return &fakeBackend{snap: core.NewSnapshot(items, rels), events: events, version: 1}
}

// newModel builds a model of the given size and runs its initial load.
func newModel(t *testing.T, be *fakeBackend, w, h int) *Model {
	t.Helper()
	m := New(context.Background(), be, Options{Project: "api-gateway", Actor: "@dev",
		Now: func() time.Time { return testNow }, Loc: time.UTC})
	m.tickEvery = 0 // tests deliver ticks by hand
	send(t, m, tea.WindowSizeMsg{Width: w, Height: h})
	runCmd(t, m, m.Init(), 0)
	return m
}

func windowSize(w, h int) tea.WindowSizeMsg { return tea.WindowSizeMsg{Width: w, Height: h} }

// send delivers msg and runs the commands it returns, reporting whether
// the model asked to quit.
func send(t *testing.T, m *Model, msg tea.Msg) bool {
	t.Helper()
	_, cmd := m.Update(msg)
	return runCmd(t, m, cmd, 0)
}

func runCmd(t *testing.T, m *Model, cmd tea.Cmd, depth int) bool {
	t.Helper()
	if cmd == nil {
		return false
	}
	if depth > 20 {
		t.Fatal("command loop")
	}
	switch msg := cmd().(type) {
	case nil:
		return false
	case tea.QuitMsg:
		return true
	case tea.BatchMsg:
		quit := false
		for _, c := range msg {
			quit = runCmd(t, m, c, depth+1) || quit
		}
		return quit
	default:
		_, next := m.Update(msg)
		return runCmd(t, m, next, depth+1)
	}
}

// press sends a key given by name, e.g. "right", "shift+left", "q".
func press(t *testing.T, m *Model, names ...string) bool {
	t.Helper()
	quit := false
	for _, n := range names {
		quit = send(t, m, keyMsg(n)) || quit
	}
	return quit
}

func keyMsg(name string) tea.KeyPressMsg {
	var k tea.KeyPressMsg
	parts := strings.Split(name, "+")
	for _, mod := range parts[:len(parts)-1] {
		switch mod {
		case "shift":
			k.Mod |= tea.ModShift
		case "ctrl":
			k.Mod |= tea.ModCtrl
		}
	}
	switch base := parts[len(parts)-1]; base {
	case "left":
		k.Code = tea.KeyLeft
	case "right":
		k.Code = tea.KeyRight
	case "up":
		k.Code = tea.KeyUp
	case "down":
		k.Code = tea.KeyDown
	case "tab":
		k.Code = tea.KeyTab
	case "enter":
		k.Code = tea.KeyEnter
	case "esc":
		k.Code = tea.KeyEscape
	case "backspace":
		k.Code = tea.KeyBackspace
	case "pgdown":
		k.Code = tea.KeyPgDown
	case "pgup":
		k.Code = tea.KeyPgUp
	case "space":
		k.Code, k.Text = tea.KeySpace, " "
	default:
		k.Code = []rune(base)[0]
		if k.Mod == 0 {
			k.Text = base
		}
	}
	return k
}

// screen returns the rendered view without ANSI styling.
func screen(m *Model) string { return ansi.Strip(m.View().Content) }

func golden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name+".golden")
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("missing golden %s (run go test -update): %v", path, err)
	}
	if got != string(want) {
		t.Fatalf("screen differs from %s:\n--- got\n%s\n--- want\n%s", path, got, want)
	}
}
