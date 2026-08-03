package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
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
	m := newModel(BuildTree(items), Options{UseNerd: true})
	m.height = 24
	m.width = 80
	bodyHeight := max(5, m.height-m.footerHeight())

	leftBox := m.renderTreeBox(40, bodyHeight)
	rightBox := m.renderPreviewBox(40, bodyHeight)

	if lipgloss.Height(leftBox) != lipgloss.Height(rightBox) {
		t.Errorf("height mismatch! leftBox=%d, rightBox=%d", lipgloss.Height(leftBox), lipgloss.Height(rightBox))
	}
	for _, line := range strings.Split(ansi.Strip(leftBox), "\n") {
		if !strings.HasSuffix(line, "│") && !strings.HasSuffix(line, "╮") && !strings.HasSuffix(line, "╯") {
			t.Fatalf("left card lost its right border: %q", line)
		}
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

	for i := 0; i < 100; i++ {
		m = updateKey(m, tea.KeyDown)
		if got := lipgloss.Height(m.View()); got != m.height {
			t.Fatalf("after scroll %d, view height = %d, want %d", i, got, m.height)
		}
	}
	lines := strings.Split(m.View(), "\n")
	if !strings.Contains(lines[len(lines)-1], "Style:") {
		t.Fatalf("footer status is not on the final row: %q", lines[len(lines)-1])
	}
}

func TestViewLinesFitTerminalWidthWhileScrolling(t *testing.T) {
	items := []Item{
		{Path: "short.go", Content: []byte("package main\n")},
		{Path: "long.go", Content: []byte(strings.Repeat("x", 400) + "\n")},
		{Path: "secret.go", Content: []byte("package main\n"), SecretCount: 3},
	}
	for _, width := range []int{40, 60, 80, 120} {
		m := newModel(BuildTree(items), Options{UseNerd: true})
		m.width = width
		m.height = 20
		for step := 0; step < len(m.rows)*2; step++ {
			view := m.View()
			for lineNo, line := range strings.Split(view, "\n") {
				if got := ansi.StringWidth(line); got > width {
					t.Fatalf("width %d step %d line %d is %d cells wide: %q", width, step, lineNo, got, ansi.Strip(line))
				}
			}
			m = updateKey(m, tea.KeyDown)
		}
	}
}

func updateKey(m model, kt tea.KeyType, runes ...rune) model {
	msg := tea.KeyMsg{Type: kt, Runes: runes}
	updated, _ := m.Update(msg)
	return updated.(model)
}
