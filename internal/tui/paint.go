package tui

import (
	"fmt"
	"image/color"
	"regexp"
	"strings"
)

// Translucent terminals usually apply their opacity only to cells drawn
// with the default background. kb paints every cell with an explicit theme
// color, so the board stays opaque and readable whatever the terminal's
// opacity.

// bgSeq is the SGR sequence that sets bg as a 24-bit background color.
func bgSeq(c color.Color) string {
	r, g, b, _ := c.RGBA()
	return fmt.Sprintf("\x1b[48;2;%d;%d;%dm", r>>8, g>>8, b>>8)
}

var sgrPattern = regexp.MustCompile(`\x1b\[([0-9;:]*)m`)

// resetsBackground reports whether an SGR sequence with these parameters
// clears the background color.
func resetsBackground(params string) bool {
	if params == "" {
		return true
	}
	for i, p := range strings.Split(params, ";") {
		if (i == 0 && (p == "0" || p == "")) || p == "49" {
			return true
		}
	}
	return false
}

// keepBackground re-applies seq after every sequence that clears the
// background, so styled spans cannot punch holes into it.
func keepBackground(s, seq string) string {
	return sgrPattern.ReplaceAllStringFunc(s, func(m string) string {
		if resetsBackground(m[2 : len(m)-1]) {
			return m + seq
		}
		return m
	})
}

// paintLine gives a single line background bg and pads it to width.
func paintLine(line string, bg color.Color, width int) string {
	seq := bgSeq(bg)
	line = seq + keepBackground(line, seq)
	if pad := width - displayWidth(line); pad > 0 {
		line += strings.Repeat(" ", pad)
	}
	return line + "\x1b[m"
}

// paint gives the whole screen background bg: every line is painted and
// padded to width, and missing lines up to height are added.
func paint(content string, bg color.Color, width, height int) string {
	lines := strings.Split(content, "\n")
	for len(lines) < height {
		lines = append(lines, "")
	}
	for i, l := range lines {
		lines[i] = paintLine(l, bg, width)
	}
	return strings.Join(lines, "\n")
}

// paintBlock paints each line of a block, used for cards and panels.
func paintBlock(lines []string, bg color.Color, width int) string {
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = paintLine(l, bg, width)
	}
	return strings.Join(out, "\n")
}
