package main

import (
	"fmt"
	"math/bits"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// tuiColors mirror runColors (blue, magenta, yellow, green, red, cyan) so a
// run keeps its color between the text and interactive views.
var tuiColors = []lipgloss.Color{"4", "5", "3", "2", "1", "6"}

const minPanelH = 12

type graphModel struct {
	runs      []graphRun
	metrics   []string
	sel       int
	zoom      bool
	pods      bool
	w, h      int
	sharedMax int64
	hidden    map[int]bool // run index -> hidden via 1-9
}

func newGraphModel(runs []graphRun, metrics []string, pods bool) graphModel {
	return graphModel{runs: runs, metrics: metrics, pods: pods, sharedMax: sharedElapsed(runs), hidden: map[int]bool{}}
}

func (m graphModel) Init() tea.Cmd { return nil }

func (m graphModel) cols() int {
	if m.zoom || m.w < 110 {
		return 1
	}
	return 2
}

func (m graphModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
	case tea.KeyMsg:
		step := 0
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "q", "esc": // back out of a zoom first, then quit
			if !m.zoom {
				return m, tea.Quit
			}
			m.zoom = false
		case "enter", "z":
			m.zoom = !m.zoom
		case "p":
			m.pods = !m.pods
		case "1", "2", "3", "4", "5", "6", "7", "8", "9":
			if i := int(msg.String()[0] - '1'); i < len(m.runs) {
				hidden := map[int]bool{}
				for k, v := range m.hidden {
					hidden[k] = v
				}
				hidden[i] = !hidden[i]
				m.hidden = hidden
			}
		case "right", "l", "tab":
			step = 1
		case "left", "h", "shift+tab":
			step = -1
		case "down", "j":
			step = m.cols()
		case "up", "k":
			step = -m.cols()
		}
		if n := m.sel + step; step != 0 && n >= 0 && n < len(m.metrics) {
			m.sel = n
		}
	}
	return m, nil
}

func fmtElapsed(sec int64) string {
	switch {
	case sec >= 3600:
		return fmt.Sprintf("%dh%02dm", sec/3600, sec%3600/60)
	case sec >= 60:
		return fmt.Sprintf("%dm%02ds", sec/60, sec%60)
	default:
		return fmt.Sprintf("%ds", sec)
	}
}

// brailleBit is the bit for the dot at (x, y) within a 2×4 braille cell.
// overlapColor marks cells where two runs' lines coincide.
const overlapColor = lipgloss.Color("15")

// graphRange is the shared y range for a metric: 0 (or the minimum, if
// negative) up to the maximum plus headroom, across every row.
func graphRange(rows []graphRow) (lo, hi float64) {
	hi = rows[0].pts[0][1]
	for _, r := range rows {
		rmin, _, rmax := seriesStats(r.pts)
		lo, hi = min(lo, rmin), max(hi, rmax)
	}
	return lo, max(hi*1.05, lo+1e-9)
}

// visible drops rows of runs hidden with 1-9.
func (m graphModel) visible(rows []graphRow) []graphRow {
	var out []graphRow
	for _, r := range rows {
		if !m.hidden[r.color] {
			out = append(out, r)
		}
	}
	return out
}

var brailleBit = [2][4]rune{{0x01, 0x02, 0x04, 0x40}, {0x08, 0x10, 0x20, 0x80}}

// renderChart draws one metric's rows as overlaid braille lines in a w×h
// block: y axis labels on the left, an elapsed-time axis underneath. X is
// elapsed seconds since each run's start on the shared 0..sharedMax range, y
// is lo..hi (see graphRange) so hiding a run doesn't rescale the chart. Every
// row's dots are merged into one cell grid (a library that redraws per
// dataset would wipe earlier runs). A terminal cell has one color, so where
// runs share one: dots that coincide draw in overlapColor, otherwise the run
// with the most dots in the cell wins.
func renderChart(rows []graphRow, lo, hi float64, unit string, sharedMax int64, w, h int) string {
	labels := [3]string{fmtSeriesValue(unit, hi), fmtSeriesValue(unit, (lo+hi)/2), fmtSeriesValue(unit, lo)}
	labelW := 0
	for _, l := range labels {
		labelW = max(labelW, len(l))
	}
	pw, ph := w-labelW-1, h-2 // plot area in cells
	if pw < 4 || ph < 2 {
		return ""
	}

	masks := map[int][]rune{} // run color index -> its dots per cell
	dw, dh := pw*2, ph*4
	set := func(x, y, ci int) {
		mk := masks[ci]
		if mk == nil {
			mk = make([]rune, pw*ph)
			masks[ci] = mk
		}
		mk[(y/4)*pw+x/2] |= brailleBit[x%2][y%4]
	}
	for _, r := range rows {
		px, py := 0, 0
		for i, p := range r.pts {
			x := int(float64(int64(p[0])-r.start) / float64(sharedMax) * float64(dw-1))
			y := int((1 - (p[1]-lo)/(hi-lo)) * float64(dh-1))
			x, y = max(0, min(dw-1, x)), max(0, min(dh-1, y))
			if i > 0 { // Bresenham from the previous point
				dx, dy := abs(x-px), -abs(y-py)
				sx, sy, err := 1, 1, dx+dy
				if px > x {
					sx = -1
				}
				if py > y {
					sy = -1
				}
				for cx, cy := px, py; ; {
					set(cx, cy, r.color)
					if cx == x && cy == y {
						break
					}
					if e2 := 2 * err; e2 >= dy {
						err += dy
						cx += sx
					} else if e2 <= dx {
						err += dx
						cy += sy
					}
				}
			} else {
				set(x, y, r.color)
			}
			px, py = x, y
		}
	}

	var cis []int
	for ci := range masks {
		cis = append(cis, ci)
	}
	sort.Ints(cis) // deterministic tie-breaks
	cell := func(i int) (lipgloss.Color, rune) {
		var union rune
		best, bestN, overlap := 0, 0, false
		for _, ci := range cis {
			mk := masks[ci][i]
			if mk == 0 {
				continue
			}
			overlap = overlap || union&mk != 0
			union |= mk
			if n := bits.OnesCount32(uint32(mk)); n >= bestN {
				best, bestN = ci, n
			}
		}
		if overlap {
			return overlapColor, union
		}
		return tuiColors[best%len(tuiColors)], union
	}

	axis := lipgloss.NewStyle().Faint(true)
	var b strings.Builder
	for row := 0; row < ph; row++ {
		lbl := ""
		switch row {
		case 0:
			lbl = labels[0]
		case ph / 2:
			lbl = labels[1]
		case ph - 1:
			lbl = labels[2]
		}
		b.WriteString(axis.Render(fmt.Sprintf("%*s│", labelW, lbl)))
		for col := 0; col < pw; col++ {
			if c, m := cell(row*pw + col); m != 0 {
				b.WriteString(lipgloss.NewStyle().Foreground(c).Render(string(0x2800 + m)))
			} else {
				b.WriteByte(' ')
			}
		}
		b.WriteByte('\n')
	}
	b.WriteString(axis.Render(strings.Repeat(" ", labelW) + "└" + strings.Repeat("─", pw)))
	b.WriteByte('\n')
	// x labels at 0, 1/4 ... end, each left-aligned at its tick (last right-aligned)
	xl := []byte(strings.Repeat(" ", pw))
	for i := 0; i <= 4; i++ {
		l := fmtElapsed(sharedMax * int64(i) / 4)
		at := pw * i / 4
		if i == 4 {
			at = pw - len(l)
		}
		if at >= 0 && at+len(l) <= pw && (i == 0 || at > 0) {
			copy(xl[at:], l)
		}
	}
	b.WriteString(axis.Render(strings.Repeat(" ", labelW+1) + string(xl)))
	return b.String()
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func (m graphModel) panel(k string, selected bool, w, h int) string {
	all, unit := graphRows(m.runs, k, m.pods)
	lo, hi := graphRange(all)
	border := lipgloss.Color("8")
	if selected {
		border = lipgloss.Color("6")
	}
	title := lipgloss.NewStyle().Bold(true).Render(graphMetricLabel(k))
	body := title + "\n" + renderChart(m.visible(all), lo, hi, unit, m.sharedMax, max(10, w-2), max(4, h-3))
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(border).
		Width(w - 2).Height(h - 2).Render(body)
}

// statsLines is the selected metric's min/avg/max per row, capped so a
// --pods run with many GPUs can't push the charts off screen.
func (m graphModel) statsLines() []string {
	all, unit := graphRows(m.runs, m.metrics[m.sel], m.pods)
	rows := m.visible(all)
	const maxLines = 5
	var out []string
	for i, r := range rows {
		if i == maxLines {
			out = append(out, fmt.Sprintf(" … %d more (zoom out of --pods with p)", len(rows)-maxLines))
			break
		}
		rmin, ravg, rmax := seriesStats(r.pts)
		out = append(out, fmt.Sprintf(" %s  min %s  avg %s  max %s%s",
			lipgloss.NewStyle().Foreground(tuiColors[r.color%len(tuiColors)]).Render("■ "+r.label),
			fmtSeriesValue(unit, rmin), fmtSeriesValue(unit, ravg), fmtSeriesValue(unit, rmax), r.peak))
	}
	return out
}

func (m graphModel) View() string {
	if m.w == 0 || len(m.metrics) == 0 {
		return ""
	}
	var legend []string
	for i, r := range m.runs {
		st := lipgloss.NewStyle().Foreground(tuiColors[i%len(tuiColors)]).Bold(true)
		if m.hidden[i] {
			st = st.Faint(true).Strikethrough(true)
		}
		legend = append(legend, st.Render(fmt.Sprintf("%d ■ %s", i+1, r.Name)))
	}
	if len(m.runs) > 1 {
		legend = append(legend, lipgloss.NewStyle().Foreground(overlapColor).Render("■ overlap"))
	}
	header := lipgloss.NewStyle().Bold(true).Render("oc aibom graph") + "  " + strings.Join(legend, "  ") +
		lipgloss.NewStyle().Faint(true).Render(fmt.Sprintf("   0 → %s elapsed", fmtElapsed(m.sharedMax)))
	stats := m.statsLines()
	helpText := " ←↑↓→ select · enter zoom · p per-pod/GPU · 1-9 hide/show run · q quit"
	if m.zoom {
		helpText = " ←↑↓→ select · enter/q back · p per-pod/GPU · 1-9 hide/show run · ctrl+c quit"
	}
	help := lipgloss.NewStyle().Faint(true).Render(helpText)
	footer := strings.Join(append(stats, help), "\n")
	bodyH := m.h - 1 - len(stats) - 1
	if bodyH < 6 {
		return header + "\n" + help
	}

	var body string
	if m.zoom {
		body = m.panel(m.metrics[m.sel], true, m.w, bodyH)
	} else {
		cols := m.cols()
		perCol := max(1, bodyH/minPanelH)
		panelH := bodyH / perCol
		page := m.sel / (cols * perCol)
		var grid []string
		for r := 0; r < perCol; r++ {
			var line []string
			for c := 0; c < cols; c++ {
				idx := page*cols*perCol + r*cols + c
				pw := m.w / cols
				if c == cols-1 {
					pw = m.w - pw*(cols-1)
				}
				if idx >= len(m.metrics) {
					line = append(line, strings.Repeat(" ", pw))
					continue
				}
				line = append(line, m.panel(m.metrics[idx], idx == m.sel, pw, panelH))
			}
			grid = append(grid, lipgloss.JoinHorizontal(lipgloss.Top, line...))
		}
		body = strings.Join(grid, "\n")
	}
	return header + "\n" + body + "\n" + footer
}

func runGraphTUI(runs []graphRun, metrics []string, pods bool) error {
	_, err := tea.NewProgram(newGraphModel(runs, metrics, pods), tea.WithAltScreen()).Run()
	return err
}
