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
