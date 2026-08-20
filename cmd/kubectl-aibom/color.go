package main

import (
	"os"
	"strings"

	"golang.org/x/term"
)

// ANSI codes are chosen to all be the same byte length (5 bytes: "\x1b[3Xm")
// so that, wherever we colorize every cell in a text/tabwriter column, the
// invisible overhead is uniform across that column and doesn't throw off
// tabwriter's width-based padding (it counts escape-code bytes as visible
// width, so mixing colored and uncolored cells in the same column would
// misalign them; coloring an entire column with equal-length codes does
// not, since it just inflates that column's padding uniformly).
const (
	ansiReset     = "\x1b[0m"
	ansiBold      = "\x1b[1m"
	ansiRed       = "\x1b[31m"
	ansiGreen     = "\x1b[32m"
	ansiYellow    = "\x1b[33m"
	ansiDefaultFG = "\x1b[39m"
)

var colorEnabled bool

// initColor decides once, after flags are parsed, whether to emit ANSI
// codes: --no-color and the NO_COLOR convention (https://no-color.org/)
// both force it off, and it's auto-disabled when stdout isn't a terminal
// (e.g. piped to a file or `less` without -r) so redirected output stays
// plain text.
func initColor(disabled bool) {
	if disabled {
		colorEnabled = false
		return
	}
	if _, set := os.LookupEnv("NO_COLOR"); set {
		colorEnabled = false
		return
	}
	colorEnabled = term.IsTerminal(int(os.Stdout.Fd()))
}

func colorize(code, s string) string {
	if !colorEnabled {
		return s
	}
	return code + s + ansiReset
}

func bold(s string) string { return colorize(ansiBold, s) }

// boldRow wraps each cell individually (not the joined line) so the
// literal tab bytes between cells stay outside any escape sequence.
func boldRow(cells ...string) string {
	bolded := make([]string, len(cells))
	for i, c := range cells {
		bolded[i] = bold(c)
	}
	return strings.Join(bolded, "\t")
}

// colorBySign colorizes s green/red/default-fg depending on the sign of v,
// using equal-length codes (see the const block above) so a whole column
// of these stays aligned under tabwriter.
func colorBySign(v float64, s string) string {
	switch {
	case v > 0:
		return colorize(ansiGreen, s)
	case v < 0:
		return colorize(ansiRed, s)
	default:
		return colorize(ansiDefaultFG, s)
	}
}

func yellow(s string) string { return colorize(ansiYellow, s) }
func red(s string) string    { return colorize(ansiRed, s) }
func green(s string) string  { return colorize(ansiGreen, s) }
