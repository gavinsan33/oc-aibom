package main

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

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
