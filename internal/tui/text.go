package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// displayWidth is the number of terminal cells s occupies, ignoring styles.
func displayWidth(s string) int { return lipgloss.Width(s) }

// truncate cuts a styled string to at most w cells, ending with "…".
func truncate(s string, w int) string {
	if w <= 0 {
		return ""
	}
	return ansi.Truncate(s, w, "…")
}

// spread puts left and right on one line of width w, truncating left first.
func spread(left, right string, w int) string {
	rw := displayWidth(right)
	if rw >= w {
		return truncate(right, w)
	}
	left = truncate(left, w-rw-1)
	gap := w - displayWidth(left) - rw
	return left + strings.Repeat(" ", max(1, gap)) + right
}

// center pads s to be centered in w cells.
func center(s string, w int) string {
	s = truncate(s, w)
	pad := (w - displayWidth(s)) / 2
	return strings.Repeat(" ", max(0, pad)) + s
}

// wrap splits plain text into at most maxLines lines of width w; the last
// line ends with "…" when text does not fit.
func wrap(text string, w, maxLines int) []string {
	var lines []string
	line := ""
	for _, word := range strings.Fields(text) {
		for displayWidth(word) > w { // hard-cut words longer than a line
			if line != "" {
				lines, line = append(lines, line), ""
			}
			lines = append(lines, ansi.Cut(word, 0, w))
			word = ansi.Cut(word, w, displayWidth(word))
		}
		switch {
		case line == "":
			line = word
		case displayWidth(line)+1+displayWidth(word) <= w:
			line += " " + word
		default:
			lines, line = append(lines, line), word
		}
	}
	if line != "" {
		lines = append(lines, line)
	}
	if len(lines) > maxLines {
		lines = lines[:maxLines]
		lines[maxLines-1] = truncate(lines[maxLines-1]+" …", w)
		if !strings.HasSuffix(lines[maxLines-1], "…") {
			lines[maxLines-1] = truncate(lines[maxLines-1], w-1) + "…"
		}
	}
	return lines
}

// fitLines returns exactly h lines of block, each cut to w cells.
func fitLines(block string, w, h int) []string {
	lines := strings.Split(block, "\n")
	if len(lines) > h {
		lines = lines[:h]
	}
	for i, l := range lines {
		lines[i] = truncate(l, w)
	}
	for len(lines) < h {
		lines = append(lines, "")
	}
	return lines
}

// wrapInput splits text being typed into lines of at most w cells. Unlike
// wrap it keeps every typed space and starts a new line at each "\n", so
// joining the lines reproduces the text apart from the newlines. A word that
// does not fit moves to the next line; a word longer than a line is cut.
//
// The lines are plain: the caller appends the cursor to the last one. A cell
// is always free for it, because when the last line would be exactly w cells
// wide an empty line is added. With maxLines > 0 and more lines than that,
// only the last maxLines are returned and the first starts with "…"; with
// maxLines <= 0 there is no cap. The result is never empty.
func wrapInput(text string, w, maxLines int) []string {
	if w < 1 {
		return []string{""}
	}
	var lines []string
	for _, para := range strings.Split(text, "\n") {
		lines = append(lines, wrapParagraph(para, w)...)
	}
	if displayWidth(lines[len(lines)-1]) >= w {
		lines = append(lines, "")
	}
	if maxLines > 0 && len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
		lines[0] = "…" + ansi.Truncate(lines[0], w-1, "")
	}
	return lines
}

// wrapParagraph wraps one line of typed text, keeping its spaces.
func wrapParagraph(para string, w int) []string {
	var lines []string
	var line []rune
	cur, brk := 0, 0 // cells in line; rune count up to and including its last space
	for _, r := range para {
		rw := displayWidth(string(r))
		for cur+rw > w && len(line) > 0 {
			cut := len(line)
			if brk > 0 && r != ' ' {
				cut = brk
			}
			lines = append(lines, string(line[:cut]))
			line = append([]rune(nil), line[cut:]...)
			cur, brk = displayWidth(string(line)), 0
			for i, c := range line {
				if c == ' ' {
					brk = i + 1
				}
			}
		}
		line = append(line, r)
		cur += rw
		if r == ' ' {
			brk = len(line)
		}
	}
	return append(lines, string(line))
}
