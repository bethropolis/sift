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

func TestVisibilityToggles(t *testing.T) {
	m := newModel(BuildTree([]Item{
		{Path: "visible.go"},
		{Path: ".env", Hidden: true},
		{Path: "build.log", GitIgnored: true},
	}), Options{})
	if len(m.rows) != 1 {
		t.Fatalf("default visible rows = %d, want 1", len(m.rows))
	}
	m = updateKey(m, tea.KeyRunes, '.')
	if len(m.rows) != 2 {
		t.Fatalf("after dot toggle rows = %d, want 2", len(m.rows))
	}
	m = updateKey(m, tea.KeyRunes, 'H')
	if len(m.rows) != 3 {
		t.Fatalf("after H toggle rows = %d, want 3", len(m.rows))
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

func TestViewKeepsStableHeightWithNarrowWidth(t *testing.T) {
	items := make([]Item, 4)
	for i := range items {
		items[i] = Item{Path: fmt.Sprintf("a/very-long-file-name-%d.go", i), TokensFull: 100}
	}
	m := newModel(BuildTree(items), Options{})
	m.width = 36
	m.height = 12
	m.showHidden = true
	m.showGitIgnored = true

	want := m.height
	if got := lipgloss.Height(m.View()); got != want {
		t.Fatalf("view height = %d, want %d", got, want)
	}
}

func updateKey(m model, kt tea.KeyType, runes ...rune) model {
	msg := tea.KeyMsg{Type: kt, Runes: runes}
	updated, _ := m.Update(msg)
	return updated.(model)
}
