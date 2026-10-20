package main

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/mattn/go-runewidth"
	"k8s.io/client-go/dynamic"

	"github.com/gsanders/oc-aibom/internal/aibom"
)

// The interactive browser `oc aibom` opens with no arguments: a cursor over
// the `list` table that drills into a scrollable describe view (one AIBOM),
// a side-by-side compare (two or more selected), or the overlaid graph TUI,
// without leaving the terminal.
const (
	modeList = iota
	modePager
	modeGraph
)

// navTabs renders the detail/graph page tabs for the embedded browser views,
// highlighting the active page so d/g read as switching to a separate page
// rather than acting on the current view. active is "detail" or "graph".
func navTabs(active string) string {
	tab := func(key, label, name string) string {
		s := fmt.Sprintf("[%s] %s", key, label)
		if name == active {
			return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6")).Render(s)
		}
		return lipgloss.NewStyle().Faint(true).Render(s)
	}
	return " " + tab("d", "Detail view", "detail") + "   " + tab("g", "Graph view", "graph")
}

const (
	pageDetail = iota
	pageCompare
)

type listModel struct {
	items  []aibom.AIBOM
	client dynamic.Interface
	allNS  bool
	sortBy string

	cursor   int
	selected map[int]bool
	w, h     int
	hoff     int // horizontal scroll offset, reset on every mode change
	mode     int
	stripeBG string // escape for zebra-striping alternate list rows ("" = none)

	// focus is the AIBOM(s) the pager or graph currently shows, so g (graph)
	// and d (detail) can switch between the two views of the same subject.
	focus []aibom.AIBOM

	// pager: captured describe/compare output, scrolled by line offset.
	pageKind    int // pageDetail or pageCompare
	pageChanged bool
	pageTitle   string
	pageLines   []string
	pageOff     int

	// graph: the existing graph TUI, composed as a sub-model so q/esc backs
	// out to the list instead of quitting.
	gm graphModel

	status string // transient line (telemetry notes, errors)
}

func (m listModel) Init() tea.Cmd { return nil }

func (m listModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
		if m.mode == modeGraph {
			nm, _ := m.gm.Update(msg)
			m.gm = nm.(graphModel)
		}
		// Keep the scroll offsets valid for the new size, then force a full
		// repaint so shrinking the window doesn't leave stale lines behind.
		m.hoff = max(0, min(m.hoff, m.contentWidth()-m.hViewW()))
		m.pageOff = max(0, min(m.pageOff, len(m.pageLines)-m.pagerBodyH()))
		return m, tea.ClearScreen
	case tea.KeyMsg:
		switch m.mode {
		case modeGraph:
			return m.updateGraph(msg)
		case modePager:
			return m.updatePager(msg)
		default:
			return m.updateList(msg)
		}
	}
	return m, nil
}

func (m listModel) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.status = ""
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.items)-1 {
			m.cursor++
		}
	case "home":
		m.cursor = 0
	case "end", "G":
		m.cursor = max(0, len(m.items)-1)
	case " ":
		if len(m.items) > 0 {
			if m.selected[m.cursor] {
				delete(m.selected, m.cursor)
			} else {
				m.selected[m.cursor] = true
			}
		}
	case "enter":
		if len(m.items) == 0 {
			break
		}
		if sel := m.selectedItems(); len(sel) >= 2 {
			(&m).openCompare(sel)
		} else {
			(&m).openDetail(m.items[m.cursor])
		}
	case "g":
		if len(m.items) > 0 {
			(&m).openGraph(m.graphTargets())
		}
	case "esc":
		m.selected = map[int]bool{}
	case "left", "right", "h", "l":
		(&m).hscroll(msg.String())
	}
	return m, nil
}

func (m listModel) updatePager(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	maxOff := max(0, len(m.pageLines)-m.pagerBodyH())
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "q", "esc", "backspace":
		m.mode = modeList
		m.hoff = 0
	case "up", "k":
		if m.pageOff > 0 {
			m.pageOff--
		}
	case "down", "j":
		if m.pageOff < maxOff {
			m.pageOff++
		}
	case "pgup", "b":
		m.pageOff = max(0, m.pageOff-m.pagerBodyH())
	case "pgdown", "f", " ":
		m.pageOff = min(maxOff, m.pageOff+m.pagerBodyH())
	case "home":
		m.pageOff = 0
	case "end", "G":
		m.pageOff = maxOff
	case "left", "right", "h", "l":
		(&m).hscroll(msg.String())
	case "g":
		(&m).openGraph(m.focus)
	case "c":
		if m.pageKind == pageCompare {
			m.pageChanged = !m.pageChanged
			(&m).renderPager()
		}
	}
	return m, nil
}

func (m listModel) updateGraph(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "q", "esc":
		if !m.gm.zoom { // let the graph model handle an un-zoom first
			m.mode = modeList
			m.hoff = 0
			return m, nil
		}
	case "d":
		if !m.gm.zoom {
			(&m).detailFromFocus()
			return m, nil
		}
	}
	nm, _ := m.gm.Update(msg)
	m.gm = nm.(graphModel)
	return m, nil
}

// selectedItems returns the checked AIBOMs in display order (which fixes their
// compare/graph colors, as the command-line order does for those commands).
func (m listModel) selectedItems() []aibom.AIBOM {
	var out []aibom.AIBOM
	for i := range m.items {
		if m.selected[i] {
			out = append(out, m.items[i])
		}
	}
	return out
}

// graphTargets is the selection, or the row under the cursor when nothing is
// selected.
func (m listModel) graphTargets() []aibom.AIBOM {
	if sel := m.selectedItems(); len(sel) > 0 {
		return sel
	}
	return []aibom.AIBOM{m.items[m.cursor]}
}

func (m *listModel) openDetail(a aibom.AIBOM) {
	m.focus = []aibom.AIBOM{a}
	m.pageKind = pageDetail
	m.pageOff = 0
	m.renderPager()
}

func (m *listModel) openCompare(items []aibom.AIBOM) {
	m.focus = items
	m.pageKind = pageCompare
	m.pageOff = 0
	m.renderPager()
}

// renderPager (re)captures the current focus into the pager for the active
// pageKind. Called on open and whenever the compare --changed toggle flips.
func (m *listModel) renderPager() {
	var b strings.Builder
	if m.pageKind == pageDetail {
		a := m.focus[0]
		ctx := context.Background()
		vr := aibom.Verify(ctx, m.client, a)
		sr := aibom.VerifySeries(ctx, m.client, a, vr)
		printDescribe(&b, a, vr, sr, false)
		m.pageTitle = "describe " + a.Name
	} else {
		printCompare(&b, m.focus, m.pageChanged)
		m.pageTitle = fmt.Sprintf("compare (%d AIBOMs)", len(m.focus))
	}
	m.pageLines = strings.Split(strings.TrimRight(b.String(), "\n"), "\n")
	m.pageOff = min(m.pageOff, max(0, len(m.pageLines)-m.pagerBodyH()))
	m.hoff = 0
	m.mode = modePager
}

// hscroll shifts the horizontal offset for the list/pager views (the graph
// view does its own navigation). Content is often wider than the terminal,
// especially when comparing many AIBOMs side by side.
func (m *listModel) hscroll(key string) {
	const step = 8
	switch key {
	case "left", "h":
		m.hoff = max(0, m.hoff-step)
	case "right", "l":
		m.hoff += step
		m.hoff = max(0, min(m.hoff, m.contentWidth()-m.hViewW()))
	}
}

// hViewW is the on-screen width available to horizontally scrollable content:
// the pager uses the full width; the list reserves 3 columns for the fixed
// cursor/selection markers.
func (m listModel) hViewW() int {
	if m.mode == modePager {
		return max(1, m.w)
	}
	return max(1, m.w-3)
}

// contentWidth is the widest line in the current view, used to clamp hoff so
// scrolling right can't run past the content into blank space.
func (m listModel) contentWidth() int {
	lines := m.pageLines
	if m.mode != modePager {
		header, body := m.listLines()
		lines = append([]string{header}, body...)
	}
	w := 0
	for _, l := range lines {
		w = max(w, ansi.StringWidth(l))
	}
	return w
}

// listLines renders the list table and splits it into the header row and the
// per-item body rows (body[i] aligns with items[i]).
func (m listModel) listLines() (header string, body []string) {
	var buf strings.Builder
	writeTable(&buf, listRows(m.items, m.allNS, m.sortBy))
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	return lines[0], lines[1:]
}

func (m *listModel) openGraph(items []aibom.AIBOM) {
	ctx := context.Background()
	runs, notes := buildGraphRuns(ctx, m.client, items)
	if len(runs) == 0 {
		m.status = "no usable telemetry — " + strings.Join(notes, "; ")
		return
	}
	keys, err := graphMetricKeys(runs, nil)
	if err != nil {
		m.status = err.Error()
		return
	}
	m.focus = items
	gm := newGraphModel(runs, keys, false)
	gm.embedded = true // show "d detail · q back" instead of "q quit"
	nm, _ := gm.Update(tea.WindowSizeMsg{Width: m.w, Height: m.h})
	m.gm = nm.(graphModel)
	m.status = strings.Join(notes, " · ")
	m.mode = modeGraph
}

// detailFromFocus returns to the pager for whatever the graph is showing: a
// single AIBOM's describe, or the compare table for several.
func (m *listModel) detailFromFocus() {
	switch {
	case len(m.focus) == 1:
		m.openDetail(m.focus[0])
	case len(m.focus) >= 2:
		m.openCompare(m.focus)
	}
}

// pagerFooterH counts the footer lines below the pager body: a blank separator
// and the key line, plus the compare --changed indicator and any status line,
// each on its own row.
func (m listModel) pagerFooterH() int {
	h := 2 // blank separator + key line
	if m.pageKind == pageCompare {
		h++
	}
	if m.status != "" {
		h++
	}
	return h
}

// pagerBodyH reserves a line for the nav tabs, the title, and the footer.
func (m listModel) pagerBodyH() int { return max(1, m.h-2-m.pagerFooterH()) }

func (m listModel) View() string {
	switch m.mode {
	case modeGraph:
		return m.gm.View()
	case modePager:
		return m.pagerView()
	default:
		return m.listView()
	}
}

func (m listModel) listView() string {
	faint := lipgloss.NewStyle().Faint(true)
	title := lipgloss.NewStyle().Bold(true).Render("oc aibom") + faint.Render(fmt.Sprintf("  %d AIBOMs", len(m.items)))
	help := faint.Render(" ↑↓ move · ←→ scroll · space select · esc clear · enter detail/compare · g graph · q quit")

	if len(m.items) == 0 {
		return title + "\n\n  No AIBOMs found.\n\n" + help
	}

	header, body := m.listLines() // body[i] aligns with m.items[i]

	avail := max(1, m.h-4) // title, header, help, status
	start := 0
	if m.cursor >= avail {
		start = m.cursor - avail + 1
	}
	end := min(len(body), start+avail)

	vw := m.hViewW()
	var b strings.Builder
	b.WriteString(title + "\n")
	b.WriteString("   " + hslice(header, m.hoff, vw) + "\n")
	for i := start; i < end; i++ {
		cur := "  "
		if i == m.cursor {
			cur = lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Bold(true).Render("▸ ")
		}
		sel := " "
		if m.selected[i] {
			sel = lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Render("✓")
		}
		row := cur + sel + hslice(body[i], m.hoff, vw)
		if i%2 == 1 { // zebra-stripe alternate rows for readability
			row = stripeRow(row, m.stripeBG, m.w)
		}
		b.WriteString(row + "\n")
	}
	b.WriteString(help)
	if m.status != "" {
		b.WriteString("\n" + faint.Render(" "+m.status))
	}
	return b.String()
}

func (m listModel) pagerView() string {
	bodyH := m.pagerBodyH()
	end := min(len(m.pageLines), m.pageOff+bodyH)
	title := lipgloss.NewStyle().Bold(true).Render(m.pageTitle)

	footer := lipgloss.NewStyle().Faint(true).Render(
		fmt.Sprintf(" ↑↓ ←→ scroll · q back · %d-%d/%d", m.pageOff+1, end, len(m.pageLines)))
	// The --changed toggle and its state get their own line so the hint is
	// clear and never pushed off the right edge of a narrow terminal.
	if m.pageKind == pageCompare {
		footer += "\n" + lipgloss.NewStyle().Faint(true).Render(" c · ") + changedIndicator(m.pageChanged)
	}
	if m.status != "" {
		footer += "\n" + lipgloss.NewStyle().Faint(true).Render(" "+m.status)
	}

	visible := make([]string, 0, end-m.pageOff)
	for _, l := range m.pageLines[m.pageOff:end] {
		visible = append(visible, hslice(l, m.hoff, m.w))
	}
	// Blank separator between the content and the footer tips.
	return navTabs("detail") + "\n" + title + "\n" + strings.Join(visible, "\n") + "\n\n" + footer
}

// changedIndicator is the bottom-of-screen toggle state for compare's
// --changed filter: a checkbox labelled "show only changed fields", green and
// ticked when on, faint and empty when off. The "c" key (shown in the footer)
// toggles it.
func changedIndicator(on bool) string {
	if on {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Bold(true).Render("[✓] show only changed fields")
	}
	return lipgloss.NewStyle().Faint(true).Render("[ ] show only changed fields")
}

// hslice returns the visible columns [off, off+width) of s, preserving the
// ANSI SGR (color/bold) styling active at the cut points. x/ansi.Truncate can
// drop columns from the right but not the left, and naively slicing runes
// would sever escape sequences, so this walks the string: it tracks the open
// SGR state (cleared by a reset), reopens it when output begins at off, and
// closes it with a reset at the end. Non-SGR escapes are passed through while
// emitting. Used for horizontal scrolling of colored list/describe lines.
func hslice(s string, off, width int) string {
	if width <= 0 {
		return ""
	}
	rs := []rune(s)
	var out, sgr strings.Builder
	col, emitting := 0, false
	for i := 0; i < len(rs); i++ {
		if rs[i] == 0x1b && i+1 < len(rs) && rs[i+1] == '[' { // CSI escape
			j := i + 2
			for j < len(rs) && (rs[j] < 0x40 || rs[j] > 0x7e) {
				j++
			}
			seq := string(rs[i:min(j+1, len(rs))])
			if j < len(rs) && rs[j] == 'm' { // SGR: track so it can be reopened
				if seq == "\x1b[0m" || seq == "\x1b[m" {
					sgr.Reset()
				} else {
					sgr.WriteString(seq)
				}
			}
			if emitting {
				out.WriteString(seq)
			}
			i = j
			continue
		}
		w := runewidth.RuneWidth(rs[i])
		if col >= off && col+w <= off+width {
			if !emitting {
				emitting = true
				out.WriteString(sgr.String())
			}
			out.WriteRune(rs[i])
		}
		if col += w; col >= off+width {
			break
		}
	}
	if emitting && sgr.Len() > 0 {
		out.WriteString(ansiReset)
	}
	return out.String()
}

// stripeRow paints a full-width background behind one list row. The row may
// contain colored cells whose reset (\x1b[0m) would otherwise clear the
// background mid-line, so each reset re-applies it; the row is padded to width
// so the stripe spans the whole line.
func stripeRow(row, bg string, width int) string {
	if bg == "" {
		return row
	}
	if pad := width - ansi.StringWidth(row); pad > 0 {
		row += strings.Repeat(" ", pad)
	}
	return bg + strings.ReplaceAll(row, ansiReset, ansiReset+bg) + ansiReset
}

// stripeBG is the background escape for zebra-striped rows: an off-white stripe
// on light terminals, an off-black one on dark, so alternate rows are easy to
// track across. Empty when color is disabled.
func stripeBG() string {
	if !colorEnabled {
		return ""
	}
	if lipgloss.HasDarkBackground() {
		return "\x1b[48;5;236m" // off-black
	}
	return "\x1b[48;5;254m" // off-white
}

func runListTUI(items []aibom.AIBOM, client dynamic.Interface, allNS bool, sortBy string) error {
	m := listModel{
		items:    items,
		client:   client,
		allNS:    allNS,
		sortBy:   sortBy,
		selected: map[int]bool{},
		stripeBG: stripeBG(), // resolved once; HasDarkBackground queries the terminal
	}
	_, err := tea.NewProgram(m, tea.WithAltScreen()).Run()
	return err
}
