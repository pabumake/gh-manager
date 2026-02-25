package tui

import "testing"

func TestAllocateColumnWidths(t *testing.T) {
	cols := []columnSpec{
		{title: "a", min: 3, max: 3, weight: 0},
		{title: "b", min: 8, max: 20, weight: 2},
		{title: "c", min: 8, max: 20, weight: 2},
	}

	narrow := allocateColumnWidths(24, cols)
	if len(narrow) != 3 || narrow[0] != 3 {
		t.Fatalf("bad narrow widths: %#v", narrow)
	}
	wide := allocateColumnWidths(80, cols)
	if wide[1] <= narrow[1] || wide[2] <= narrow[2] {
		t.Fatalf("expected wider columns in wide layout: narrow=%#v wide=%#v", narrow, wide)
	}
}

func TestEnsureVisible(t *testing.T) {
	m := model{
		filtered: make([]int, 200),
		height:   20,
	}
	m.cursor = 120
	m.ensureVisible()
	if m.scroll == 0 {
		t.Fatalf("expected scroll to move for deep cursor")
	}
	m.cursor = 0
	m.ensureVisible()
	if m.scroll != 0 {
		t.Fatalf("expected scroll reset near top, got %d", m.scroll)
	}
}
