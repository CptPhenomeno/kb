package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/CptPhenomeno/kb/internal/core"
)

// itemView is the stable JSON shape of an item (SPEC.md §6.4). Changing a
// key is a breaking change for agents: see docs/cli-json.md.
type itemView struct {
	ID          string          `json:"id"`
	Ref         *string         `json:"ref"`
	Type        string          `json:"type"`
	Title       string          `json:"title"`
	Description string          `json:"description"`
	Parent      *string         `json:"parent"`
	Status      string          `json:"status"`
	Priority    string          `json:"priority"`
	Points      *int            `json:"points"`
	Spec        string          `json:"spec"`
	Labels      []string        `json:"labels"`
	Links       []string        `json:"links"`
	Acceptance  []criterionView `json:"acceptance"`
	Tests       []testView      `json:"tests"`
	TDDPhase    *string         `json:"tdd_phase"`
	Assignee    *string         `json:"assignee"`
	Claim       *claimView      `json:"claim"`
	BlockedBy   []string        `json:"blocked_by"`
	Blocks      []string        `json:"blocks"`
	Relates     []string        `json:"relates"`
	Duplicates  []string        `json:"duplicates"`
	Position    int             `json:"position"`
	Version     int             `json:"version"`
	CreatedAt   string          `json:"created_at"`
	CreatedBy   string          `json:"created_by"`
	UpdatedAt   string          `json:"updated_at"`
	UpdatedBy   string          `json:"updated_by"`
	History     []eventView     `json:"history,omitempty"`
}

type criterionView struct {
	Text string `json:"text"`
	Done bool   `json:"done"`
}

type testView struct {
	Name  string `json:"name"`
	State string `json:"state"`
}

type claimView struct {
	Actor     string `json:"actor"`
	ExpiresAt string `json:"expires_at"`
	Stale     bool   `json:"stale"`
}

type eventView struct {
	ID    int64          `json:"id"`
	At    string         `json:"at"`
	Actor string         `json:"actor"`
	Item  string         `json:"item"`
	Kind  string         `json:"kind"`
	Data  map[string]any `json:"data"`
}

// table aligns columns like tabwriter and strips the trailing spaces that
// empty last columns would leave.
type table struct {
	out io.Writer
	buf bytes.Buffer
	tw  *tabwriter.Writer
}

func newTable(w io.Writer) *table {
	t := &table{out: w}
	t.tw = tabwriter.NewWriter(&t.buf, 0, 0, 2, ' ', 0)
	return t
}

func (t *table) Write(p []byte) (int, error) { return t.tw.Write(p) }

func (t *table) Flush() {
	t.tw.Flush()
	for _, line := range strings.SplitAfter(t.buf.String(), "\n") {
		trimmed := strings.TrimRight(line, " \n")
		if strings.HasSuffix(line, "\n") {
			trimmed += "\n"
		}
		io.WriteString(t.out, trimmed)
	}
	t.buf.Reset()
}

func ts(t time.Time) string { return t.UTC().Format(time.RFC3339) }

func optional(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// newItemView builds the JSON view of an item; snap supplies relations.
func newItemView(snap *core.Snapshot, it *core.Item, now time.Time) itemView {
	v := itemView{
		ID: it.ID, Ref: optional(it.Ref), Type: it.Type.String(), Title: it.Title,
		Description: it.Description, Parent: optional(it.Parent), Status: it.Status.String(),
		Priority: it.Priority.String(), Spec: it.Spec, Labels: nonNil(it.Labels), Links: nonNil(it.Links),
		Acceptance: []criterionView{}, Tests: []testView{}, TDDPhase: optional(it.Phase.String()),
		Assignee: optional(it.Assignee), BlockedBy: []string{}, Blocks: []string{}, Relates: []string{},
		Duplicates: []string{}, Position: it.Position, Version: it.Version,
		CreatedAt: ts(it.CreatedAt), CreatedBy: it.CreatedBy, UpdatedAt: ts(it.UpdatedAt), UpdatedBy: it.UpdatedBy,
	}
	if it.Points != 0 {
		p := it.Points
		v.Points = &p
	}
	for _, c := range it.Acceptance {
		v.Acceptance = append(v.Acceptance, criterionView(c))
	}
	for _, t := range it.Tests {
		v.Tests = append(v.Tests, testView{Name: t.Name, State: t.State.String()})
	}
	if it.Claim != nil {
		v.Claim = &claimView{Actor: it.Claim.Actor, ExpiresAt: ts(it.Claim.ExpiresAt),
			Stale: core.ClaimState(it, now) == core.ClaimStale}
	}
	if _, by := core.Blockers(snap, it.ID); by != nil {
		v.BlockedBy = by
	}
	for _, r := range snap.Relations {
		switch {
		case r.Kind == core.RelBlocks && r.From == it.ID:
			v.Blocks = append(v.Blocks, r.To)
		case r.Kind == core.RelRelates && r.From == it.ID:
			v.Relates = append(v.Relates, r.To)
		case r.Kind == core.RelRelates && r.To == it.ID:
			v.Relates = append(v.Relates, r.From)
		case r.Kind == core.RelDuplicates && r.From == it.ID:
			v.Duplicates = append(v.Duplicates, r.To)
		}
	}
	for _, l := range [][]string{v.Blocks, v.Relates, v.Duplicates} {
		slices.SortFunc(l, core.CompareIDs)
	}
	return v
}

func newEventView(e core.Event) eventView {
	data := e.Data
	if data == nil {
		data = map[string]any{}
	}
	return eventView{ID: e.ID, At: ts(e.At), Actor: e.Actor, Item: e.Item, Kind: string(e.Kind), Data: data}
}

func nonNil(xs []string) []string {
	if xs == nil {
		return []string{}
	}
	return xs
}

// printJSON writes v as indented JSON.
func printJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// ---- text rendering ----

var prioMark = map[core.Priority]string{core.PriorityHigh: "▲▲", core.PriorityMedium: "▲", core.PriorityLow: "·"}

func pointsText(p int) string {
	if p == 0 {
		return "-"
	}
	return fmt.Sprintf("%dpt", p)
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func joinOrDash(xs []string) string {
	if len(xs) == 0 {
		return "-"
	}
	return strings.Join(xs, ", ")
}

// flags lists the warning badges of an item.
func flags(snap *core.Snapshot, it *core.Item, now time.Time) string {
	var f []string
	if blocked, by := core.Blockers(snap, it.ID); blocked {
		f = append(f, "BLOCKED by "+strings.Join(by, ","))
	}
	if core.ClaimState(it, now) == core.ClaimStale {
		f = append(f, "STALE")
	}
	return strings.Join(f, " ")
}

// writeList prints items as an aligned table.
func writeList(w io.Writer, snap *core.Snapshot, items []*core.Item, now time.Time) {
	tw := newTable(w)
	for _, it := range items {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s %s\t%s\t%s\t%s\t%s\n", it.ID, it.Type, it.Status,
			prioMark[it.Priority], it.Priority, pointsText(it.Points), it.Title, orDash(it.Assignee),
			flags(snap, it, now))
	}
	tw.Flush()
}

// writeItem prints the full detail of an item, history included.
func writeItem(w io.Writer, snap *core.Snapshot, it *core.Item, history []core.Event, now time.Time) {
	v := newItemView(snap, it, now)
	fmt.Fprintf(w, "%s  %s  %s\n\n", it.ID, it.Type, it.Title)
	tw := newTable(w)
	row := func(k, val string) { fmt.Fprintf(tw, "%s\t%s\n", k, val) }
	row("status", fmt.Sprintf("%s (%s)", it.Status, it.Status.Label()))
	row("priority", fmt.Sprintf("%s %s", prioMark[it.Priority], it.Priority))
	row("points", pointsText(it.Points))
	parent := "-"
	if p := snap.Get(it.Parent); p != nil {
		parent = p.ID + " " + p.Title
	}
	row("parent", parent)
	assignee := orDash(it.Assignee)
	if it.Claim != nil {
		assignee += fmt.Sprintf(" (claim %s, expires %s)", core.ClaimState(it, now), it.Claim.ExpiresAt.UTC().Format("2006-01-02 15:04 UTC"))
	}
	row("assignee", assignee)
	if it.Ref != "" {
		row("ref", it.Ref)
	}
	row("spec", orDash(it.Spec))
	row("labels", joinOrDash(it.Labels))
	row("links", joinOrDash(it.Links))
	if it.Type.IsLeafType() {
		row("tdd phase", orDash(it.Phase.String()))
	}
	row("blocked by", joinOrDash(v.BlockedBy))
	row("blocks", joinOrDash(v.Blocks))
	row("relates", joinOrDash(v.Relates))
	if len(v.Duplicates) > 0 {
		row("duplicates", joinOrDash(v.Duplicates))
	}
	if done, total := core.Progress(snap, it); total > 0 && it.Type.IsParentType() {
		row("progress", fmt.Sprintf("%d/%d children done", done, total))
	}
	row("version", fmt.Sprint(it.Version))
	tw.Flush()

	if d := strings.TrimSpace(it.Description); d != "" {
		fmt.Fprintf(w, "\n%s\n", d)
	}
	if len(it.Tests) > 0 {
		fmt.Fprintf(w, "\ntests %d/%d passing\n", it.PassingTests(), len(it.Tests))
		for i, t := range it.Tests {
			fmt.Fprintf(w, "  %d %s %s (%s)\n", i+1, testMark[t.State], t.Name, t.State)
		}
	}
	if len(it.Acceptance) > 0 {
		fmt.Fprintf(w, "\nacceptance %d/%d checked\n", it.CheckedCriteria(), len(it.Acceptance))
		for i, c := range it.Acceptance {
			box := "[ ]"
			if c.Done {
				box = "[x]"
			}
			fmt.Fprintf(w, "  %d %s %s\n", i+1, box, c.Text)
		}
	}
	if len(history) > 0 {
		fmt.Fprintln(w, "\nhistory")
		tw := newTable(w)
		for _, e := range history {
			fmt.Fprintf(tw, "  %s\t%s\t%s\n", e.At.UTC().Format("2006-01-02 15:04"), e.Actor, core.DescribeEvent(e))
		}
		tw.Flush()
	}
}

var testMark = map[core.TestState]string{core.TestPassing: "✓", core.TestFailing: "✗", core.TestPending: "○"}

// writeTree prints the hierarchy below the given roots.
func writeTree(w io.Writer, snap *core.Snapshot, roots []*core.Item, now time.Time) {
	tw := newTable(w)
	var walk func(it *core.Item, depth int)
	walk = func(it *core.Item, depth int) {
		progress := "-"
		if done, total := core.Progress(snap, it); total > 0 {
			unit := "tests"
			if it.Type.IsParentType() {
				unit = "done"
			}
			progress = fmt.Sprintf("%d/%d %s", done, total, unit)
		}
		pts := pointsText(it.Points)
		if it.Type == core.TypeEpic {
			pts = pointsText(core.PointsSum(snap, it.ID))
		}
		fmt.Fprintf(tw, "%s%s\t%s\t%s\t%s\t%s\t%s\t%s\n", strings.Repeat("  ", depth), it.ID, it.Type,
			it.Status, it.Title, progress, pts, flags(snap, it, now))
		for _, c := range snap.Children(it.ID) {
			walk(c, depth+1)
		}
	}
	for _, r := range roots {
		walk(r, 0)
	}
	tw.Flush()
}
