package tui

import (
	"reflect"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestSelectAndResult(t *testing.T) {
	items := []Item{
		{Path: "a.go", Tokens: 10},
		{Path: "b.go", Tokens: 20},
		{Path: "c.go", Tokens: 30},
	}
	m := newModel(items)

	// Select the second item.
	m = updateKey(m, tea.KeyDown)
	m = updateKey(m, tea.KeySpace)

	// Select everything via 'a'.
	m = updateKey(m, tea.KeyRunes, 'a')

	got := m.selectedPaths()
	want := []string{"a.go", "b.go", "c.go"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("selectedPaths = %v, want %v", got, want)
	}

	// 'a' again deselects all.
	m = updateKey(m, tea.KeyRunes, 'a')
	if got := m.selectedPaths(); len(got) != 0 {
		t.Errorf("after toggle-all-off selectedPaths = %v, want empty", got)
	}
}

func TestCursorClamping(t *testing.T) {
	m := newModel([]Item{{Path: "a"}, {Path: "b"}, {Path: "c"}, {Path: "d"}, {Path: "e"}})
	m.height = 5 // header 2 + footer 2 => 1 visible row

	// Move past the end; cursor must stop at the last item.
	for i := 0; i < 10; i++ {
		m = updateKey(m, tea.KeyDown)
	}
	if m.cursor != 4 {
		t.Errorf("cursor = %d, want 4", m.cursor)
	}
	// Window must follow the cursor.
	if m.offset != 4 {
		t.Errorf("offset = %d, want 4", m.offset)
	}
	if got := m.row(4, 60); got == "" {
		t.Error("row(4) rendered empty")
	}
}

func TestViewRendersRows(t *testing.T) {
	m := newModel([]Item{{Path: "a.go", Tokens: 10}})
	view := m.View()
	if view == "" {
		t.Fatal("empty view")
	}
}

func updateKey(m model, kt tea.KeyType, runes ...rune) model {
	msg := tea.KeyMsg{Type: kt, Runes: runes}
	updated, _ := m.Update(msg)
	return updated.(model)
}
