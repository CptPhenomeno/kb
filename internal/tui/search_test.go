package tui

import (
	"strings"
	"testing"

	"github.com/CptPhenomeno/kb/internal/core"
)

func searchItem(typ core.Type, title string, opts ...func(*core.Item)) *core.Item {
	return item("KB-9", typ, core.StatusTodo, "", title, opts...)
}

func TestSearchWordsCombineInAnd(t *testing.T) {
	it := searchItem(core.TypeTask, "Token refresh", withTags("api"))
	for q, want := range map[string]bool{
		"token":             true,
		"token refresh":     true,
		"  token   api  ":   true,
		"token zzz":         false,
		"zzz token":         false,
		"token tag:api":     true,
		"token tag:ui":      false,
		"refresh type:task": true,
	} {
		if got := matchSearch(q, it); got != want {
			t.Errorf("matchSearch(%q) = %v, want %v", q, got, want)
		}
	}
}

func TestPlainWordMatchesATagByContains(t *testing.T) {
	it := searchItem(core.TypeTask, "Unrelated", withTags("Infrastructure"))
	if !matchSearch("struct", it) {
		t.Error("plain word must match a tag by contains, without case")
	}
	it.Assignee = "Agent-Auth"
	for _, q := range []string{"agent-au", "kb-9", "UNREL"} {
		if !matchSearch(q, it) {
			t.Errorf("plain word %q must match assignee, ID or title", q)
		}
	}
}

func TestTagWordMatchesExactlyWithoutCase(t *testing.T) {
	it := searchItem(core.TypeTask, "x", withTags("Api"))
	for _, q := range []string{"tag:api", "tag:API", "tag:Api"} {
		if !matchSearch(q, it) {
			t.Errorf("%q must match tag Api", q)
		}
	}
	if matchSearch("tag:ui", it) || matchSearch("tag:", it) {
		t.Error("tag:ui and an empty tag: must not match")
	}
}

func TestTagWordDoesNotMatchASubstringOfATag(t *testing.T) {
	it := searchItem(core.TypeTask, "x", withTags("infrastructure"))
	if matchSearch("tag:infra", it) || matchSearch("tag:structure", it) {
		t.Error("tag: must compare the whole tag name")
	}
}

func TestTypeWordKeepsOnlyThatType(t *testing.T) {
	if !matchSearch("type:bug", searchItem(core.TypeBug, "x")) || !matchSearch("type:BUG", searchItem(core.TypeBug, "x")) {
		t.Error("type:bug must match a bug, without case")
	}
	if matchSearch("type:bug", searchItem(core.TypeTask, "x")) {
		t.Error("type:bug must not match a task")
	}
}

func TestUnknownTypeWordMatchesNothing(t *testing.T) {
	for _, typ := range core.Types() {
		if matchSearch("type:nope", searchItem(typ, "x")) || matchSearch("type:", searchItem(typ, "x")) {
			t.Errorf("an unknown type must match nothing, matched %s", typ)
		}
	}
}

func TestBoardTypeWordNarrowsWithinTheFilterTypes(t *testing.T) {
	m := newModel(t, sampleBoard(), 160, 48)
	press(t, m, "/", "t", "y", "p", "e", ":", "t", "a", "s", "k", "enter")
	s := screen(m)
	if !strings.Contains(s, "Token refresh") || strings.Contains(s, "Expired token") || strings.Contains(s, "Token login") {
		t.Fatalf("type:task must keep only tasks:\n%s", s)
	}
	// subtask is not among the types chosen with f, so type:subtask shows nothing
	m2 := newModel(t, sampleBoard(), 160, 48)
	press(t, m2, "/")
	for _, r := range "type:subtask" {
		press(t, m2, string(r))
	}
	press(t, m2, "enter")
	if s := screen(m2); strings.Contains(s, "Bounded route label") {
		t.Fatalf("type: must not widen the f filter:\n%s", s)
	}
}

func TestTreeKeepsTheAncestorsOfATagMatch(t *testing.T) {
	be := sampleBoard()
	for _, it := range be.snap.Items() {
		if it.ID == "KB-4" {
			it.Tags = []string{"Api"}
		}
	}
	m := newModel(t, be, 160, 48)
	press(t, m, "tab") // Tree view
	press(t, m, "/")
	for _, r := range "tag:api" {
		press(t, m, string(r))
	}
	press(t, m, "enter")
	if got := strings.Join(treeIDs(m), " "); got != "KB-1 KB-2 KB-4" {
		t.Fatalf("rows = %s, want the tag match and its ancestors", got)
	}
}
