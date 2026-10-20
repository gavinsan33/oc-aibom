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
