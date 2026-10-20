package main

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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

type listModel struct {
	items  []aibom.AIBOM
	client dynamic.Interface
	allNS  bool
	sortBy string

	cursor   int
	selected map[int]bool
	w, h     int
	mode     int

	// pager: captured describe/compare output, scrolled by line offset.
	pageTitle string
	pageLines []string
	pageOff   int

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
		return m, nil
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
	ctx := context.Background()
	vr := aibom.Verify(ctx, m.client, a)
	sr := aibom.VerifySeries(ctx, m.client, a, vr)
	var b strings.Builder
	printDescribe(&b, a, vr, sr, false)
	m.openPager("describe "+a.Name, b.String())
}

func (m *listModel) openCompare(items []aibom.AIBOM) {
	var b strings.Builder
	printCompare(&b, items)
	m.openPager(fmt.Sprintf("compare (%d AIBOMs)", len(items)), b.String())
}

func (m *listModel) openPager(title, body string) {
	m.pageTitle = title
	m.pageLines = strings.Split(strings.TrimRight(body, "\n"), "\n")
	m.pageOff = 0
	m.mode = modePager
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
	gm := newGraphModel(runs, keys, false)
	nm, _ := gm.Update(tea.WindowSizeMsg{Width: m.w, Height: m.h})
	m.gm = nm.(graphModel)
	m.status = strings.Join(notes, " · ")
	m.mode = modeGraph
}

func (m listModel) pagerBodyH() int { return max(1, m.h-2) }

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
	help := faint.Render(" ↑↓ move · space select · enter detail/compare · g graph · q quit")

	if len(m.items) == 0 {
		return title + "\n\n  No AIBOMs found.\n\n" + help
	}

	var buf strings.Builder
	writeTable(&buf, listRows(m.items, m.allNS, m.sortBy))
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	header, body := lines[0], lines[1:] // body[i] aligns with m.items[i]

	avail := max(1, m.h-4) // title, header, help, status
	start := 0
	if m.cursor >= avail {
		start = m.cursor - avail + 1
	}
	end := min(len(body), start+avail)

	var b strings.Builder
	b.WriteString(title + "\n")
	b.WriteString("   " + header + "\n")
	for i := start; i < end; i++ {
		cur := "  "
		if i == m.cursor {
			cur = lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Bold(true).Render("▸ ")
		}
		sel := " "
		if m.selected[i] {
			sel = lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Render("✓")
		}
		b.WriteString(cur + sel + body[i] + "\n")
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
	help := lipgloss.NewStyle().Faint(true).Render(
		fmt.Sprintf(" ↑↓ scroll · q back · %d-%d/%d", m.pageOff+1, end, len(m.pageLines)))
	return title + "\n" + strings.Join(m.pageLines[m.pageOff:end], "\n") + "\n" + help
}

func runListTUI(items []aibom.AIBOM, client dynamic.Interface, allNS bool, sortBy string) error {
	m := listModel{
		items:    items,
		client:   client,
		allNS:    allNS,
		sortBy:   sortBy,
		selected: map[int]bool{},
	}
	_, err := tea.NewProgram(m, tea.WithAltScreen()).Run()
	return err
}
