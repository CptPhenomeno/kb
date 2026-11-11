package tui

import (
	"strings"
	"testing"
)

func assertFits(t *testing.T, lines []string, w int) {
	t.Helper()
	for _, l := range lines {
		if displayWidth(l) > w {
			t.Errorf("line %q is wider than %d", l, w)
		}
	}
}

func TestWrapInputWrapsAtWidth(t *testing.T) {
	got := wrapInput("hello big world", 8, 0)
	want := []string{"hello ", "big ", "world"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("got %q, want %q", got, want)
	}
	got = wrapInput("abcdefghij", 4, 0)
	if strings.Join(got, "|") != "abcd|efgh|ij" {
		t.Errorf("long word: got %q", got)
	}
	if strings.Join(got, "") != "abcdefghij" {
		t.Errorf("characters lost: %q", got)
	}
}

func TestWrapInputCursorFitsOnFullLine(t *testing.T) {
	got := wrapInput("abcd", 4, 0)
	if len(got) != 2 || got[0] != "abcd" || got[1] != "" {
		t.Errorf("got %q, want [abcd \"\"]", got)
	}
	got = wrapInput("abc", 4, 0)
	if len(got) != 1 || displayWidth(got[0])+1 > 4 {
		t.Errorf("got %q", got)
	}
	if got := wrapInput("", 4, 0); len(got) != 1 || got[0] != "" {
		t.Errorf("empty: got %q", got)
	}
}

func TestWrapInputKeepsTrailingSpaces(t *testing.T) {
	got := wrapInput("ab  ", 10, 0)
	if len(got) != 1 || got[0] != "ab  " {
		t.Errorf("got %q", got)
	}
	got = wrapInput("abc ", 4, 0)
	if strings.Join(got, "") != "abc " {
		t.Errorf("space lost: %q", got)
	}
	assertFits(t, got, 4)
}

func TestWrapInputRespectsNewlines(t *testing.T) {
	got := wrapInput("one\n\ntwo\n", 10, 0)
	want := "one||two|"
	if strings.Join(got, "|") != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestWrapInputClipsWithLeadingEllipsis(t *testing.T) {
	got := wrapInput("l1\nl2\nl3\nl4\nl5", 6, 3)
	if len(got) != 3 || !strings.HasPrefix(got[0], "…") {
		t.Fatalf("got %q", got)
	}
	if got[1] != "l4" || got[2] != "l5" {
		t.Errorf("last lines wrong: %q", got)
	}
	got = wrapInput("abcdefgh", 4, 2)
	if !strings.HasPrefix(got[0], "…") || got[1] != "" {
		t.Errorf("got %q", got)
	}
	assertFits(t, got, 4)
	if got := wrapInput("a\nb", 6, 2); strings.Contains(strings.Join(got, ""), "…") {
		t.Errorf("nothing hidden but got %q", got)
	}
}

func TestWrapInputWideCharactersDoNotOverflow(t *testing.T) {
	for _, text := range []string{"日本語のテキスト入力", "a日b本c語 😀😀😀 x"} {
		for w := 1; w <= 7; w++ {
			got := wrapInput(text, w, 0)
			if w >= 2 {
				assertFits(t, got, w)
			}
			if strings.ReplaceAll(strings.Join(got, ""), " ", "") != strings.ReplaceAll(text, " ", "") {
				t.Errorf("w=%d: characters lost: %q", w, got)
			}
		}
	}
	got := wrapInput("日本語", 4, 0)
	if strings.Join(got, "|") != "日本|語" {
		t.Errorf("got %q", got)
	}
}

func TestWrapInputClippedFirstLineDropsLeadingCells(t *testing.T) {
	got := wrapInput("zz\nthe refresh token\nend", 17, 2)
	if got[0] != "…he refresh token" {
		t.Errorf("got %q, want the leading cell replaced by the ellipsis", got[0])
	}
	got = wrapInput("日本語のテキスト日本語の", 8, 3)
	if !strings.HasPrefix(got[0], "…キスト") || displayWidth(got[0]) > 8 {
		t.Errorf("wide leading character: got %q", got[0])
	}
}

func TestWrapInputSpaceAtWrapPointDoesNotStartNextLine(t *testing.T) {
	got := wrapInput("abcd efgh", 4, 0)
	if strings.Join(got, "|") != "abcd|efgh|" {
		t.Errorf("got %q", got)
	}
	got = wrapInput("abcd ", 4, 0)
	if strings.Join(got, "|") != "abcd|" {
		t.Errorf("typed space on a full line: got %q, want the cursor on a new empty line", got)
	}
	got = wrapInput("one two three four", 9, 0)
	for _, l := range got {
		if strings.HasPrefix(l, " ") {
			t.Errorf("line starts with a space: %q", got)
		}
	}
	got = wrapInput("ab  cd", 10, 0)
	if len(got) != 1 || got[0] != "ab  cd" {
		t.Errorf("inner spaces must stay: %q", got)
	}
}
