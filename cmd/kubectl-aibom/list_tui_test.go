package main

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/gsanders/oc-aibom/internal/aibom"
)

func testModel() listModel {
	return listModel{
		items:    []aibom.AIBOM{{Name: "a"}, {Name: "b"}, {Name: "c"}},
		selected: map[int]bool{},
	}
}

func key(s string) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)} }

func TestListCursorClamps(t *testing.T) {
	m := testModel()
	// up at top stays at 0
	nm, _ := m.updateList(key("k"))
	m = nm.(listModel)
	if m.cursor != 0 {
		t.Fatalf("cursor up at top = %d, want 0", m.cursor)
	}
	// down past the end clamps to last index
	for i := 0; i < 5; i++ {
		nm, _ = m.updateList(key("j"))
		m = nm.(listModel)
	}
	if m.cursor != 2 {
		t.Fatalf("cursor after many downs = %d, want 2", m.cursor)
	}
}

func TestListSelectOrder(t *testing.T) {
	m := testModel()
	// select c (index 2) first, then a (index 0); selectedItems must be in
	// display order, not click order, so colors stay stable.
	m.selected[2] = true
	m.selected[0] = true
	got := m.selectedItems()
	if len(got) != 2 || got[0].Name != "a" || got[1].Name != "c" {
		t.Fatalf("selectedItems = %+v, want [a c]", got)
	}
}

func TestGraphTargetsFallsBackToCursor(t *testing.T) {
	m := testModel()
	m.cursor = 1
	if g := m.graphTargets(); len(g) != 1 || g[0].Name != "b" {
		t.Fatalf("graphTargets with no selection = %+v, want [b]", g)
	}
	m.selected[0] = true
	if g := m.graphTargets(); len(g) != 1 || g[0].Name != "a" {
		t.Fatalf("graphTargets with selection = %+v, want [a]", g)
	}
}

func TestHslicePlain(t *testing.T) {
	if got := hslice("abcdefgh", 2, 3); got != "cde" {
		t.Fatalf("hslice plain = %q, want %q", got, "cde")
	}
	if got := hslice("abc", 0, 10); got != "abc" {
		t.Fatalf("hslice beyond end = %q, want %q", got, "abc")
	}
	if got := hslice("abc", 5, 3); got != "" {
		t.Fatalf("hslice past content = %q, want empty", got)
	}
}

func TestHslicePreservesColor(t *testing.T) {
	// A fully-colored word, sliced in the middle, must keep the color on the
	// visible run and occupy exactly the requested number of visible columns.
	colored := ansiRed + "abcd" + ansiReset
	got := hslice(colored, 1, 2)
	if ansi.StringWidth(got) != 2 {
		t.Fatalf("visible width = %d, want 2 (%q)", ansi.StringWidth(got), got)
	}
	if got != ansiRed+"bc"+ansiReset {
		t.Fatalf("hslice colored = %q, want red bc reset", got)
	}
}

func TestHscrollClamps(t *testing.T) {
	m := testModel()
	m.w = 20
	for i := 0; i < 50; i++ { // scroll hard right, far past the content
		(&m).hscroll("right")
	}
	if maxOff := m.contentWidth() - m.hViewW(); m.hoff > max(0, maxOff) {
		t.Fatalf("hoff %d exceeds clamp %d", m.hoff, maxOff)
	}
	(&m).hscroll("left")
	(&m).hscroll("left")
	if m.hoff < 0 {
		t.Fatalf("hoff went negative: %d", m.hoff)
	}
}

func TestClearSelection(t *testing.T) {
	m := testModel()
	m.selected[0] = true
	m.selected[2] = true
	nm, _ := m.updateList(key("c"))
	m = nm.(listModel)
	if len(m.selected) != 0 {
		t.Fatalf("clear left %d selected, want 0", len(m.selected))
	}
}

func TestComparePagerChangedToggle(t *testing.T) {
	m := testModel()
	m.focus = []aibom.AIBOM{m.items[0], m.items[1]}
	m.pageKind = pageCompare
	(&m).renderPager()
	full := len(m.pageLines)

	nm, _ := m.updatePager(key("c"))
	m = nm.(listModel)
	if !m.pageChanged {
		t.Fatal("c did not enable --changed in compare pager")
	}
	if len(m.pageLines) > full {
		t.Fatalf("changed view (%d lines) should not exceed full view (%d)", len(m.pageLines), full)
	}

	nm, _ = m.updatePager(key("c"))
	m = nm.(listModel)
	if m.pageChanged {
		t.Fatal("second c did not disable --changed")
	}
}

func TestChangedToggleIgnoredOnDetailPager(t *testing.T) {
	m := testModel()
	m.pageKind = pageDetail
	m.pageLines = []string{"Name: a"}
	nm, _ := m.updatePager(key("c"))
	m = nm.(listModel)
	if m.pageChanged {
		t.Fatal("c should not toggle --changed on a detail pager")
	}
}

func TestDetailFromFocusPicksCompareForMany(t *testing.T) {
	m := testModel()
	m.focus = []aibom.AIBOM{m.items[0], m.items[1]}
	(&m).detailFromFocus()
	if m.mode != modePager || m.pageKind != pageCompare {
		t.Fatalf("two-run focus → mode %d kind %d, want pager/compare", m.mode, m.pageKind)
	}
}

func TestResizeClampsPagerOffset(t *testing.T) {
	m := testModel()
	m.mode = modePager
	m.pageLines = []string{"1", "2", "3", "4", "5"}
	m.pageOff = 4
	nm, cmd := m.Update(tea.WindowSizeMsg{Width: 80, Height: 10})
	m = nm.(listModel)
	if maxOff := len(m.pageLines) - m.pagerBodyH(); m.pageOff > max(0, maxOff) {
		t.Fatalf("pageOff %d exceeds clamp %d after resize", m.pageOff, maxOff)
	}
	if cmd == nil {
		t.Fatal("resize should return a repaint command")
	}
}

func TestSpaceTogglesSelection(t *testing.T) {
	m := testModel()
	space := tea.KeyMsg{Type: tea.KeySpace}
	nm, _ := m.updateList(space)
	m = nm.(listModel)
	if !m.selected[0] {
		t.Fatalf("space did not select cursor row (key=%q)", space.String())
	}
	nm, _ = m.updateList(space)
	m = nm.(listModel)
	if m.selected[0] {
		t.Fatal("second space did not deselect cursor row")
	}
}
