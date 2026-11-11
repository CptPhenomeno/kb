package tui

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/CptPhenomeno/kb/internal/core"
)

// detailText renders the detail popup alone, as it is under any popup.
func detailText(m *Model) string {
	w, h := m.detailSize()
	return ansi.Strip(m.renderDetail(w, h))
}

// openDetail selects id on a live board and opens its detail popup.
func openDetail(t *testing.T, m *Model, id string) {
	t.Helper()
	selectID(t, m, id)
	press(t, m, "enter")
	if !m.detailOpen || m.selectedID() != id {
		t.Fatalf("detail not open on %s (open %v, selected %s)", id, m.detailOpen, m.selectedID())
	}
}

func TestEOpensTheEditFormOverTheDetail(t *testing.T) {
	m, _ := newLiveModel(t)
	openDetail(t, m, "KB-2")
	press(t, m, "e")
	s := screen(m)
	if m.form == nil || !m.detailOpen || !strings.Contains(s, "Edit KB-2") {
		t.Fatalf("form %v, detail open %v:\n%s", m.form != nil, m.detailOpen, s)
	}
	if !strings.Contains(s, "Beta") {
		t.Fatalf("form not prefilled:\n%s", s)
	}
}

func TestSavingTheFormReturnsToTheUpdatedDetail(t *testing.T) {
	m, svc := newLiveModel(t)
	openDetail(t, m, "KB-2")
	press(t, m, "e")
	typeText(t, m, " v2")
	press(t, m, "enter")
	if it, _ := svc.Get(context.Background(), "KB-2"); it.Title != "Beta v2" {
		t.Fatalf("title = %q, want 'Beta v2'", it.Title)
	}
	if m.form != nil || !m.detailOpen || m.selectedID() != "KB-2" || !strings.Contains(detailText(m), "Beta v2") {
		t.Fatalf("form %v, open %v, selected %s:\n%s", m.form != nil, m.detailOpen, m.selectedID(), screen(m))
	}
	if strings.Contains(screen(m), "Edit KB-2") {
		t.Fatal("the form is still drawn")
	}
}

func TestEscOnTheFormReturnsToTheDetail(t *testing.T) {
	m, _ := newLiveModel(t)
	openDetail(t, m, "KB-2")
	press(t, m, "e", "esc")
	if m.form != nil || !m.detailOpen || strings.Contains(screen(m), "Edit KB-2") {
		t.Fatalf("form %v, open %v:\n%s", m.form != nil, m.detailOpen, screen(m))
	}
	press(t, m, "esc")
	if m.detailOpen {
		t.Fatal("a second esc did not close the detail")
	}
}

func TestCtrlEEditsTheDescriptionAndReturnsToTheUpdatedDetail(t *testing.T) {
	m, svc := newLiveModel(t)
	openDetail(t, m, "KB-2")
	_, cmd := m.Update(keyMsg("ctrl+e"))
	if cmd == nil || m.editing != "KB-2" {
		t.Fatalf("ctrl+e did not start editing KB-2 (editing %q)", m.editing)
	}
	it, _ := svc.Get(context.Background(), "KB-2")
	send(t, m, editorDoneMsg{target: "KB-2", version: it.Version, text: "Written in the editor."})
	if !m.detailOpen || !strings.Contains(detailText(m), "Written in the editor.") {
		t.Fatalf("open %v:\n%s", m.detailOpen, screen(m))
	}
}

func TestCOpensTheCriteriaPopupOverTheDetail(t *testing.T) {
	m, _ := newStoryModel(t)
	openDetail(t, m, "KB-1")
	press(t, m, "c")
	s := screen(m)
	if m.accept == nil || !m.detailOpen || !strings.Contains(s, "Acceptance") || !strings.Contains(s, "[ ] opens the board") {
		t.Fatalf("accept %v, open %v:\n%s", m.accept != nil, m.detailOpen, s)
	}
	press(t, m, "esc")
	if m.accept != nil || !m.detailOpen {
		t.Fatalf("esc: accept %v, open %v", m.accept != nil, m.detailOpen)
	}
}

func TestCheckingACriterionUpdatesTheDetailBehindThePopup(t *testing.T) {
	m, svc := newStoryModel(t)
	openDetail(t, m, "KB-1")
	if strings.Contains(detailText(m), "[x] opens the board") {
		t.Fatal("fixture criterion already checked")
	}
	press(t, m, "c", "space")
	if it, _ := svc.Get(context.Background(), "KB-1"); !it.Acceptance[0].Done {
		t.Fatal("criterion not saved")
	}
	if !m.detailOpen || m.accept == nil || !strings.Contains(detailText(m), "[x] opens the board") {
		t.Fatalf("detail behind the popup is stale:\n%s", detailText(m))
	}
}

func TestAClaimsTheItemShownAndTheDetailShowsTheAssignee(t *testing.T) {
	m, svc := newLiveModel(t)
	openDetail(t, m, "KB-2")
	press(t, m, "a")
	if it, _ := svc.Get(context.Background(), "KB-2"); it.Claim == nil || it.Claim.Actor != "@dev" {
		t.Fatalf("a did not claim KB-2: %+v", it.Claim)
	}
	if !m.detailOpen || !strings.Contains(detailText(m), "assignee  @dev") {
		t.Fatalf("detail lacks the assignee:\n%s", screen(m))
	}
	press(t, m, "a")
	if it, _ := svc.Get(context.Background(), "KB-2"); it.Claim != nil {
		t.Fatal("a again did not release KB-2")
	}
	if strings.Contains(detailText(m), "assignee") {
		t.Fatalf("detail still shows the assignee:\n%s", detailText(m))
	}
}

func TestLOpensTheLinksPopupOverTheDetail(t *testing.T) {
	m, svc := newLiveModel(t)
	if err := svc.Link(context.Background(), "@dev", core.Relation{Kind: core.RelRelates, From: "KB-1", To: "KB-3"}); err != nil {
		t.Fatal(err)
	}
	reload(t, m)
	openDetail(t, m, "KB-1")
	press(t, m, "l")
	if m.links == nil || !m.detailOpen || !strings.Contains(screen(m), "Gamma") {
		t.Fatalf("links %v, open %v:\n%s", m.links != nil, m.detailOpen, screen(m))
	}
	press(t, m, "esc")
	if m.links != nil || !m.detailOpen {
		t.Fatal("esc did not return to the detail")
	}
}

func TestFollowingALinkShowsTheLinkedItemInTheDetail(t *testing.T) {
	m, svc := newLiveModel(t)
	if err := svc.Link(context.Background(), "@dev", core.Relation{Kind: core.RelRelates, From: "KB-1", To: "KB-3"}); err != nil {
		t.Fatal(err)
	}
	reload(t, m)
	openDetail(t, m, "KB-1")
	m.detailScroll = 3
	press(t, m, "l", "enter")
	if m.links != nil || !m.detailOpen || m.selectedID() != "KB-3" || m.detailScroll != 0 {
		t.Fatalf("links %v, open %v, selected %s, scroll %d", m.links != nil, m.detailOpen, m.selectedID(), m.detailScroll)
	}
	if !strings.Contains(detailText(m), "Gamma") {
		t.Fatalf("detail does not show KB-3:\n%s", detailText(m))
	}
}

func TestAVersionConflictShowsTheAlertOverTheDetail(t *testing.T) {
	m, svc := newLiveModel(t)
	openDetail(t, m, "KB-2")
	it, _ := svc.Get(context.Background(), "KB-2")
	_, cmd := m.Update(keyMsg("ctrl+e"))
	if cmd == nil {
		t.Fatal("ctrl+e did not start the editor")
	}
	title := "Changed by an agent"
	if _, err := svc.Edit(context.Background(), "agent-x", "KB-2", core.Patch{Title: &title}, 0); err != nil {
		t.Fatal(err)
	}
	send(t, m, editorDoneMsg{target: "KB-2", version: it.Version, text: "Stale description."})
	s := screen(m)
	if m.alert == nil || !m.detailOpen || !strings.Contains(s, "CHANGED MEANWHILE") {
		t.Fatalf("alert %v, open %v:\n%s", m.alert != nil, m.detailOpen, s)
	}
	press(t, m, "esc")
	if m.alert != nil || !m.detailOpen || !strings.Contains(detailText(m), title) {
		t.Fatalf("closing the alert: alert %v, open %v:\n%s", m.alert != nil, m.detailOpen, screen(m))
	}
}

func TestPopupsOverTheDetailTakeTheMouse(t *testing.T) {
	m := longDetail(t)
	press(t, m, "c")
	if m.accept == nil {
		t.Fatal("criteria popup not open")
	}
	send(t, m, tea.MouseWheelMsg{X: 80, Y: 15, Button: tea.MouseWheelDown})
	send(t, m, tea.MouseClickMsg{X: 2, Y: 2, Button: tea.MouseLeft})
	if m.detailScroll != 0 || !m.detailOpen || m.accept == nil {
		t.Fatalf("mouse reached the detail under the popup: scroll %d, open %v", m.detailScroll, m.detailOpen)
	}
}

func TestCriteriaPopupOverTheDetailGolden(t *testing.T) {
	be := sampleBoard()
	for _, it := range be.snap.Items() {
		if it.ID == "KB-4" {
			it.Acceptance = []core.Criterion{{Text: "returns a new access + refresh pair", Done: true},
				{Text: "reused refresh token → 401"}, {Text: "an expired refresh token → 401 with a clear message"}}
		}
	}
	m := newModel(t, be, 160, 48)
	selectID(t, m, "KB-4")
	press(t, m, "enter", "c", "down")
	golden(t, "detail_accept_160x48", screen(m))
}
