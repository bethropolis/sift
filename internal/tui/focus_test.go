package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func bigContent() []byte {
	content := make([]byte, 0, 200)
	for i := 1; i <= 50; i++ {
		content = append(content, []byte(fmt.Sprintf("line %02d\n", i))...)
	}
	return content
}

func TestTabTogglesFocus(t *testing.T) {
	m := newModel(BuildTree([]Item{{Path: "a.go"}}), Options{})
	if m.focus != FocusTree {
		t.Fatalf("initial focus = %v, want FocusTree", m.focus)
	}

	m = updateKey(m, tea.KeyTab)
	if m.focus != FocusPreview {
		t.Errorf("after Tab focus = %v, want FocusPreview", m.focus)
	}

	m = updateKey(m, tea.KeyTab)
	if m.focus != FocusTree {
		t.Errorf("after second Tab focus = %v, want FocusTree", m.focus)
	}
}

func TestFocusPreviewScrollKeys(t *testing.T) {
	m := newModel(BuildTree([]Item{{Path: "big.go", Content: bigContent()}}), Options{})
	m.height = 12
	m.width = 80

	m = updateKey(m, tea.KeyRunes, 'j')
	if m.cursor != 0 {
		t.Fatalf("tree-focused j moved cursor to %d, want 0 (single row)", m.cursor)
	}
	if m.previewOffset != 0 {
		t.Fatalf("tree-focused j changed previewOffset to %d, want 0", m.previewOffset)
	}

	m = updateKey(m, tea.KeyTab) // FocusPreview
	m = updateKey(m, tea.KeyRunes, 'j')
	if m.previewOffset != 1 {
		t.Errorf("preview-focused j previewOffset = %d, want 1", m.previewOffset)
	}
	m = updateKey(m, tea.KeyRunes, 'k')
	if m.previewOffset != 0 {
		t.Errorf("preview-focused k previewOffset = %d, want 0", m.previewOffset)
	}

	m = updateKey(m, tea.KeyDown)
	if m.previewOffset != 1 {
		t.Errorf("preview-focused Down previewOffset = %d, want 1", m.previewOffset)
	}
	m = updateKey(m, tea.KeyUp)
	if m.previewOffset != 0 {
		t.Errorf("preview-focused Up previewOffset = %d, want 0", m.previewOffset)
	}
}

func TestFocusSwitchBackKeys(t *testing.T) {
	m := newModel(BuildTree([]Item{{Path: "big.go", Content: bigContent()}}), Options{})
	m.height = 12
	m.width = 80
	m = updateKey(m, tea.KeyTab) // FocusPreview
	m = updateKey(m, tea.KeyRunes, 'j')

	m = updateKey(m, tea.KeyRunes, 'h')
	if m.focus != FocusTree {
		t.Errorf("h focus = %v, want FocusTree", m.focus)
	}
	if m.previewOffset != 1 {
		t.Errorf("h reset previewOffset to %d, want 1", m.previewOffset)
	}

	m = updateKey(m, tea.KeyTab)
	m = updateKey(m, tea.KeyLeft)
	if m.focus != FocusTree {
		t.Errorf("Left focus = %v, want FocusTree", m.focus)
	}
}

func TestEscSwitchesBeforeQuitting(t *testing.T) {
	m := newModel(BuildTree([]Item{{Path: "a.go"}}), Options{})

	m = updateKey(m, tea.KeyEsc)
	if !m.quit {
		t.Error("Esc with tree focus should quit")
	}

	// Preview focus: Esc returns to the tree without quitting.
	m = newModel(BuildTree([]Item{{Path: "a.go"}}), Options{})
	m = updateKey(m, tea.KeyTab)
	m = updateKey(m, tea.KeyEsc)
	if m.quit {
		t.Error("Esc with preview focus should not quit")
	}
	if m.focus != FocusTree {
		t.Errorf("Esc focus = %v, want FocusTree", m.focus)
	}
}

func TestDirectPreviewHotkeys(t *testing.T) {
	m := newModel(BuildTree([]Item{{Path: "big.go", Content: bigContent()}}), Options{})
	m.height = 12
	m.width = 80

	// J/K scroll from anywhere without moving the cursor.
	m = updateKey(m, tea.KeyRunes, 'J')
	if m.previewOffset != 1 {
		t.Errorf("J previewOffset = %d, want 1", m.previewOffset)
	}
	m = updateKey(m, tea.KeyRunes, 'K')
	if m.previewOffset != 0 {
		t.Errorf("K previewOffset = %d, want 0", m.previewOffset)
	}

	// The cursor must not have moved.
	if m.cursor != 0 {
		t.Errorf("J/K moved cursor to %d, want 0", m.cursor)
	}
}

func TestMouseWheelScopesToPane(t *testing.T) {
	m := newModel(BuildTree([]Item{
		{Path: "a.go"},
		{Path: "big.go", Content: bigContent()},
	}), Options{})
	m.height = 12
	m.width = 80
	leftWidth := m.leftPaneWidth()

	// Wheel over the tree moves the cursor.
	m = updateMouse(m, tea.MouseMsg{Type: tea.MouseWheelDown, X: leftWidth - 1})
	if m.cursor != 1 {
		t.Errorf("wheel over tree cursor = %d, want 1", m.cursor)
	}

	// Wheel over the preview scrolls it, leaving the cursor alone.
	m = updateMouse(m, tea.MouseMsg{Type: tea.MouseWheelDown, X: leftWidth + 1})
	if m.previewOffset != 3 {
		t.Errorf("wheel over preview previewOffset = %d, want 3", m.previewOffset)
	}
	if m.cursor != 1 {
		t.Errorf("wheel over preview moved cursor to %d, want 1", m.cursor)
	}

	// Clicking a pane switches focus to it.
	m = updateMouse(m, tea.MouseMsg{Type: tea.MouseRelease, X: leftWidth + 1})
	if m.focus != FocusPreview {
		t.Errorf("click preview focus = %v, want FocusPreview", m.focus)
	}
	m = updateMouse(m, tea.MouseMsg{Type: tea.MouseRelease, X: leftWidth - 1})
	if m.focus != FocusTree {
		t.Errorf("click tree focus = %v, want FocusTree", m.focus)
	}
}

// TestMouseIgnoredInModal ensures pointer events are inert while a modal or
// the filter prompt is open.
func TestMouseIgnoredInModal(t *testing.T) {
	m := newModel(BuildTree([]Item{{Path: "a.go"}}), Options{})
	m.filtering = true
	before := m.cursor
	m = updateMouse(m, tea.MouseMsg{Type: tea.MouseWheelDown, X: 10})
	if m.cursor != before {
		t.Error("mouse wheel should be ignored while filtering")
	}
}

func updateMouse(m model, mm tea.MouseMsg) model {
	updated, _ := m.Update(mm)
	return updated.(model)
}

// TestPaneBorderHighlights asserts the focused pane renders with the bright
// blue border and the inactive one stays muted.
func TestPaneBorderHighlights(t *testing.T) {
	// lipgloss suppresses ANSI codes in non-TTY contexts; force a color
	// profile so the border colors are observable.
	prev := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.ANSI256)
	defer lipgloss.SetColorProfile(prev)

	m := newModel(BuildTree([]Item{{Path: "a.go"}}), Options{})
	bodyHeight := max(5, m.height-m.footerHeight())

	treeFocused := m.renderTreeBox(40, bodyHeight)
	m.focus = FocusPreview
	treeBlurred := m.renderTreeBox(40, bodyHeight)
	previewFocused := m.renderPreviewBox(40, bodyHeight)
	m.focus = FocusTree
	previewBlurred := m.renderPreviewBox(40, bodyHeight)

	// The top-left border corner carries the border color: bright blue (94)
	// when focused, muted 8-bit 62 when not.
	assertBorder := func(t *testing.T, name, got, focused string) {
		t.Helper()
		if !strings.Contains(got, focused) {
			t.Errorf("%s border not highlighted: missing %q", name, focused)
		}
	}
	assertBorder(t, "tree focused", treeFocused, "\x1b[94m╭")
	assertBorder(t, "preview focused", previewFocused, "\x1b[94m╭")
	if !strings.Contains(treeBlurred, "\x1b[38;5;62m╭") {
		t.Errorf("tree blurred border should stay muted color 62")
	}
	if !strings.Contains(previewBlurred, "\x1b[38;5;62m╭") {
		t.Errorf("preview blurred border should stay muted color 62")
	}
}
