package tui

import (
	"context"
	"strings"
	"testing"

	"github.com/CptPhenomeno/kb/internal/core"
)

// TestConflictReloadsAndNotifies: an agent changes KB-1 after the TUI read
// it; the TUI's move must not overwrite that change, and the user must see
// who changed what.
func TestConflictReloadsAndNotifies(t *testing.T) {
	m, svc := newLiveModel(t)
	if _, err := svc.Claim(context.Background(), "agent-x", "KB-1", 0); err != nil { // the TUI has not reloaded
		t.Fatal(err)
	}
	press(t, m, "shift+left") // the TUI still believes KB-1 is in To do
	if got := status(t, svc, "KB-1"); got != core.StatusDoing {
		t.Fatalf("the stale move overwrote the agent's change: KB-1 is %s", got)
	}
	s := screen(m)
	for _, want := range []string{"CHANGED MEANWHILE", "agent-x", "status: To do → In progress", "assignee"} {
		if !strings.Contains(s, want) {
			t.Errorf("conflict alert lacks %q:\n%s", want, s)
		}
	}
	press(t, m, "enter")
	if it := m.snap.Get("KB-1"); it.Status != core.StatusDoing {
		t.Fatalf("the TUI did not reload: KB-1 shown as %s", it.Status)
	}
}
