package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestHelpToggle(t *testing.T) {
	m := newModel(BuildTree([]Item{{Path: "a.go"}}), Options{})

	m = updateKey(m, tea.KeyRunes, '?')
	if !m.helpOpen {
		t.Fatal("? did not open help")
	}
	if view := m.View(); !strings.Contains(view, "Keyboard Shortcuts") {
		t.Errorf("help view missing title: %q", view)
	}

	m = updateKey(m, tea.KeyRunes, '?')
	if m.helpOpen {
		t.Error("second ? did not close help")
	}
}

func TestHelpCloseKeys(t *testing.T) {
	m := newModel(BuildTree([]Item{{Path: "a.go"}}), Options{})
	m = updateKey(m, tea.KeyRunes, '?')
	if !m.helpOpen {
		t.Fatal("help did not open")
	}

	// Esc closes help without quitting.
	m = updateKey(m, tea.KeyEsc)
	if m.helpOpen {
		t.Error("Esc did not close help")
	}
	if m.quit {
		t.Error("Esc in help should not quit the picker")
	}
}

func TestHelpScrollsAndClamps(t *testing.T) {
	m := newModel(BuildTree([]Item{{Path: "a.go"}}), Options{})
	m.height = 10
	m = updateKey(m, tea.KeyRunes, '?')
	if !m.helpOpen {
		t.Fatal("help did not open")
	}
	for i := 0; i < 20; i++ {
		m = updateKey(m, tea.KeyDown)
	}
	if m.helpOffset == 0 {
		t.Fatal("help did not scroll down")
	}
	maxOffset := max(0, len(helpContentLines())-m.helpPageSize())
	if m.helpOffset > maxOffset {
		t.Fatalf("helpOffset = %d, want <= %d", m.helpOffset, maxOffset)
	}
	for i := 0; i < 20; i++ {
		m = updateKey(m, tea.KeyUp)
	}
	if m.helpOffset != 0 {
		t.Fatalf("helpOffset = %d after scrolling up, want 0", m.helpOffset)
	}
}

func TestWindowTitleSanitizesControlCharacters(t *testing.T) {
	got := sanitizeWindowTitle(" sift\x1b]0;bad\a\n ")
	if strings.ContainsAny(got, "\x1b\n\a") {
		t.Fatalf("title contains control characters: %q", got)
	}
}

func TestEscQuitsInBaseState(t *testing.T) {
	m := newModel(BuildTree([]Item{{Path: "a.go"}}), Options{})
	m = updateKey(m, tea.KeyEsc)
	if !m.quit {
		t.Error("Esc should quit the picker in the base state")
	}
}

func TestEnterTogglesFile(t *testing.T) {
	m := newModel(BuildTree([]Item{
		{Path: "a.go"},
		{Path: "b.go"},
	}), Options{})

	m = updateKey(m, tea.KeyEnter)
	if m.quit {
		t.Fatal("Enter on a file should not quit")
	}
	if got := m.root.Selections(); len(got) != 1 || got[0].Path != "a.go" {
		t.Errorf("after Enter selections = %v, want a.go", got)
	}
}

func TestEnterTogglesDirExpand(t *testing.T) {
	m := newModel(BuildTree([]Item{
		{Path: "internal/app.go"},
		{Path: "internal/b.go"},
	}), Options{})
	if len(m.rows) != 1 {
		t.Fatalf("initial rows = %d, want 1 (collapsed dir)", len(m.rows))
	}

	m = updateKey(m, tea.KeyEnter)
	if len(m.rows) != 3 {
		t.Errorf("after Enter rows = %d, want 3 (expanded dir)", len(m.rows))
	}
	if m.quit {
		t.Error("Enter on a dir should not quit")
	}
}

func TestGenerateNotice(t *testing.T) {
	root := BuildTree([]Item{{Path: "a.go", TokensFull: 10}})
	m := newModel(root, Options{OnGenerate: func(sel []Selection) error { return nil }})
	m = updateKey(m, tea.KeyRunes, 'a')
	m = updateKey(m, tea.KeyRunes, 'g')

	if m.quit {
		t.Fatal("g should not quit the picker")
	}
	if !strings.Contains(m.notice, "Generated output (1 files") {
		t.Errorf("notice = %q", m.notice)
	}
}

func TestGenerateNothingSelected(t *testing.T) {
	m := newModel(BuildTree([]Item{{Path: "a.go"}}), Options{OnGenerate: func(sel []Selection) error { return nil }})
	m = updateKey(m, tea.KeyRunes, 'g')
	if m.notice != "Nothing selected to generate" {
		t.Errorf("notice = %q", m.notice)
	}
}

func TestExpandCollapseAllKeys(t *testing.T) {
	m := newModel(BuildTree([]Item{
		{Path: "a/x.go"},
		{Path: "b/y/z.go"},
	}), Options{})
	if len(m.rows) != 2 {
		t.Fatalf("initial rows = %d, want 2", len(m.rows))
	}

	// E expands every directory.
	m = updateKey(m, tea.KeyRunes, 'E')
	wantRows := 5 // a, x.go, b, y, z.go
	if len(m.rows) != wantRows {
		t.Errorf("after E rows = %d, want %d", len(m.rows), wantRows)
	}

	// C collapses every directory.
	m = updateKey(m, tea.KeyRunes, 'C')
	if len(m.rows) != 2 {
		t.Errorf("after C rows = %d, want 2", len(m.rows))
	}
}
