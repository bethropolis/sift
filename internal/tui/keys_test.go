package tui

import (
	"fmt"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func TestCursorClamping(t *testing.T) {
	m := newModel(BuildTree([]Item{
		{Path: "a.go"}, {Path: "b.go"}, {Path: "c.go"}, {Path: "d.go"}, {Path: "e.go"},
	}), Options{})
	m.height = 5

	for i := 0; i < 10; i++ {
		m = updateKey(m, tea.KeyDown)
	}
	if m.cursor != 4 {
		t.Errorf("cursor = %d, want 4", m.cursor)
	}
}

func TestViewRendersRows(t *testing.T) {
	m := newModel(BuildTree([]Item{{Path: "a.go", TokensFull: 10}}), Options{})
	if view := m.View(); view == "" {
		t.Fatal("empty view")
	}
}

func TestCardHeightsEqual(t *testing.T) {
	items := make([]Item, 30)
	for i := 0; i < 30; i++ {
		items[i] = Item{Path: fmt.Sprintf("file_%d.go", i), Content: []byte("package main\nfunc main() {}\n")}
	}
	m := newModel(BuildTree(items), Options{})
	m.height = 24
	m.width = 80
	bodyHeight := max(5, m.height-m.footerHeight())

	leftBox := m.renderTreeBox(40, bodyHeight)
	rightBox := m.renderPreviewBox(40, bodyHeight)

	if lipgloss.Height(leftBox) != lipgloss.Height(rightBox) {
		t.Errorf("height mismatch! leftBox=%d, rightBox=%d", lipgloss.Height(leftBox), lipgloss.Height(rightBox))
	}
}

func updateKey(m model, kt tea.KeyType, runes ...rune) model {
	msg := tea.KeyMsg{Type: kt, Runes: runes}
	updated, _ := m.Update(msg)
	return updated.(model)
}
