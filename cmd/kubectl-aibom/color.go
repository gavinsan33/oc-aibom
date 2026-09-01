package main

import (
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"

	"golang.org/x/term"
)

const (
	ansiReset     = "\x1b[0m"
	ansiBold      = "\x1b[1m"
	ansiRed       = "\x1b[31m"
	ansiGreen     = "\x1b[32m"
	ansiYellow    = "\x1b[33m"
	ansiBlue      = "\x1b[34m"
	ansiMagenta   = "\x1b[35m"
	ansiCyan      = "\x1b[36m"
	ansiDefaultFG = "\x1b[39m"
)

// runColors cycles through distinct colors for each AIBOM being compared in
// diff/compare, so a value in the table can be traced back to its source
// run at a glance without re-reading the header every time.
var runColors = []string{ansiBlue, ansiMagenta, ansiYellow, ansiGreen, ansiRed, ansiCyan}

func runColor(i int) string { return runColors[i%len(runColors)] }

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

// boldColorize applies both bold and code in a single escape/reset pair.
func boldColorize(code, s string) string {
	if !colorEnabled {
		return s
	}
	return ansiBold + code + s + ansiReset
}

func bold(s string) string { return colorize(ansiBold, s) }
func cyan(s string) string { return colorize(ansiCyan, s) }

// colorBySign colorizes s green/red/default-fg depending on the sign of v.
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

// cell is one table cell: visible is the plain, uncolored text used to
// compute column widths, and rendered is what actually gets printed (which
// may wrap visible in ANSI codes). Column widths must be computed from
// visible, not rendered — text/tabwriter counts escape-code bytes as
// visible width, which misaligns columns whenever some cells in a column
// are colorized and others aren't (e.g. a bold header over plain data).
type cell struct {
	visible  string
	rendered string
}

func plainCell(s string) cell { return cell{visible: s, rendered: s} }

func coloredCell(visible, rendered string) cell { return cell{visible: visible, rendered: rendered} }

func plainRow(values ...string) []cell {
	row := make([]cell, len(values))
	for i, v := range values {
		row[i] = plainCell(v)
	}
	return row
}

func headerRow(values ...string) []cell {
	row := make([]cell, len(values))
	for i, v := range values {
		row[i] = coloredCell(v, bold(v))
	}
	return row
}

// labelHeaderCell renders a diff/compare table's leading column header
// (FIELD, METRIC) bold and cyan, distinguishing it from the AIBOM-name
// columns that follow.
func labelHeaderCell(s string) cell { return coloredCell(s, boldColorize(ansiCyan, s)) }

// labelCell renders a diff/compare data row's leading cell (a field or
// metric name) cyan, matching labelHeaderCell so the whole column reads as
// one unit.
func labelCell(s string) cell { return coloredCell(s, cyan(s)) }

// runHeaderCell renders the i-th AIBOM name in a diff/compare header bold
// and in that run's color (see runColor), so its data cells below can be
// traced back to it at a glance.
func runHeaderCell(i int, name string) cell {
	return coloredCell(name, boldColorize(runColor(i), name))
}

// writeTable prints rows padded to align columns, using each cell's visible
// text to compute widths so ANSI-colorized cells never throw off alignment.
func writeTable(w io.Writer, rows [][]cell) {
	var widths []int
	for _, row := range rows {
		for i, c := range row {
			for len(widths) <= i {
				widths = append(widths, 0)
			}
			if l := utf8.RuneCountInString(c.visible); l > widths[i] {
				widths[i] = l
			}
		}
	}
	for _, row := range rows {
		for i, c := range row {
			fmt.Fprint(w, c.rendered)
			if i < len(row)-1 {
				fmt.Fprint(w, strings.Repeat(" ", widths[i]-utf8.RuneCountInString(c.visible)+2))
			}
		}
		fmt.Fprintln(w)
	}
}
