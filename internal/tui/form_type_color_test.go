package tui

import (
	"strconv"
	"strings"
	"testing"

	"github.com/CptPhenomeno/kb/internal/core"
	"github.com/charmbracelet/x/ansi"
)

// typeLine returns the raw (styled) form line that shows the type field,
// starting at its label so the board behind the popup is left out.
func typeLine(t *testing.T, m *Model) string {
	t.Helper()
	for _, l := range strings.Split(m.View().Content, "\n") {
		if i := strings.Index(l, "type        "); i >= 0 {
			return l[i:]
		}
	}
	t.Fatalf("no type line in:\n%s", screen(m))
	return ""
}

// fgSeq is the SGR foreground sequence of the board color of a type.
func fgSeq(m *Model, typ core.Type) string {
	r, g, b, _ := m.th.typeColor[typ].RGBA()
	return "38;2;" + strconv.Itoa(int(r>>8)) + ";" + strconv.Itoa(int(g>>8)) + ";" + strconv.Itoa(int(b>>8))
}

func TestCreateFormTypeUsesTheTypeColor(t *testing.T) {
	m, _ := newLiveModel(t)
	press(t, m, "n")
	press(t, m, "down", "down") // leave the type field: it is shown unfocused
	line := typeLine(t, m)
	if !strings.Contains(line, fgSeq(m, core.TypeTask)+"mtask") {
		t.Fatalf("task is not drawn in the task color %s: %q", fgSeq(m, core.TypeTask), line)
	}
}

func TestFocusedTypeBetweenArrowsKeepsTheTypeColor(t *testing.T) {
	m, _ := newLiveModel(t)
	press(t, m, "n")
	press(t, m, "down") // focus type
	line := typeLine(t, m)
	if !strings.Contains(ansi.Strip(line), "‹ task ›") {
		t.Fatalf("type is not focused:\n%s", screen(m))
	}
	if !strings.Contains(line, fgSeq(m, core.TypeTask)) {
		t.Fatalf("focused task is not in the task color: %q", line)
	}
}

func TestEditFormFixedTypeUsesTheTypeColorWithMutedFixed(t *testing.T) {
	m, svc := newLiveModel(t)
	selectID(t, m, "KB-2")
	it, _ := svc.Get(t.Context(), "KB-2")
	press(t, m, "e")
	line := typeLine(t, m)
	name := it.Type.String()
	if !strings.Contains(ansi.Strip(line), name+" (fixed)") {
		t.Fatalf("no fixed type:\n%s", screen(m))
	}
	if !strings.Contains(line, fgSeq(m, it.Type)+"m"+name) {
		t.Fatalf("fixed type %s not in its color: %q", name, line)
	}
	i := strings.Index(line, "(fixed)")
	if i < 0 || strings.Contains(line[:i][strings.LastIndex(line[:i], "\x1b["):], fgSeq(m, it.Type)) {
		t.Fatalf("(fixed) is not muted: %q", line)
	}
}
