package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func TestPreviewScroll(t *testing.T) {
	content := make([]byte, 0, 200)
	for i := 1; i <= 50; i++ {
		content = append(content, []byte(fmt.Sprintf("line %02d\n", i))...)
	}
	m := newModel(BuildTree([]Item{{Path: "big.go", Content: content}}), Options{})
	m.height = 12
	m.width = 80

	page := m.previewPageSize()
	if page <= 0 {
		t.Fatalf("previewPageSize = %d, want > 0", page)
	}

	if m.previewOffset != 0 {
		t.Fatalf("initial previewOffset = %d, want 0", m.previewOffset)
	}

	m = updateKey(m, tea.KeyPgDown)
	if m.previewOffset != page {
		t.Errorf("after PgDn previewOffset = %d, want %d", m.previewOffset, page)
	}

	m = updateKey(m, tea.KeyPgUp)
	if m.previewOffset != 0 {
		t.Errorf("after PgUp previewOffset = %d, want 0", m.previewOffset)
	}

	m = updateKey(m, tea.KeyCtrlD)
	if m.previewOffset != m.previewHalfPage() {
		t.Errorf("after Ctrl+d previewOffset = %d, want %d", m.previewOffset, m.previewHalfPage())
	}

	m = updateKey(m, tea.KeyRunes, ']')
	if m.previewOffset != m.previewHalfPage()+page {
		t.Errorf("after ] previewOffset = %d, want %d", m.previewOffset, m.previewHalfPage()+page)
	}

	m = updateKey(m, tea.KeyRunes, '[')
	if m.previewOffset != m.previewHalfPage() {
		t.Errorf("after [ previewOffset = %d, want %d", m.previewOffset, m.previewHalfPage())
	}

	// Scrolling beyond the end clamps at the last possible offset.
	total := 50
	wantMax := max(0, total-m.previewPageSize())
	for i := 0; i < 10; i++ {
		m = updateKey(m, tea.KeyPgDown)
	}
	if m.previewOffset != wantMax {
		t.Errorf("clamped previewOffset = %d, want %d", m.previewOffset, wantMax)
	}

	// Scrolling above the start clamps at zero.
	for i := 0; i < 10; i++ {
		m = updateKey(m, tea.KeyPgUp)
	}
	if m.previewOffset != 0 {
		t.Errorf("clamped previewOffset = %d, want 0", m.previewOffset)
	}
}

func TestPreviewScrollResetOnCursorMove(t *testing.T) {
	content := make([]byte, 0, 200)
	for i := 1; i <= 50; i++ {
		content = append(content, []byte(fmt.Sprintf("line %02d\n", i))...)
	}
	m := newModel(BuildTree([]Item{
		{Path: "a.go", Content: content},
		{Path: "b.go", Content: []byte("short")},
	}), Options{})
	m.height = 12
	m.width = 80

	m = updateKey(m, tea.KeyPgDown)
	if m.previewOffset == 0 {
		t.Fatal("expected previewOffset to be scrolled before moving")
	}

	// Moving to the next file resets the scroll position.
	m = updateKey(m, tea.KeyDown)
	if m.previewOffset != 0 {
		t.Errorf("previewOffset after moving = %d, want 0 (reset)", m.previewOffset)
	}
	if m.previewNode == nil || m.previewNode.Path != "b.go" {
		t.Errorf("previewNode = %+v, want b.go", m.previewNode)
	}
}

func TestPreviewScrollNoopOnDirectory(t *testing.T) {
	m := newModel(BuildTree([]Item{{Path: "dir/a.go", Content: []byte("x")}}), Options{})
	m.height = 12
	m.width = 80

	m = updateKey(m, tea.KeyPgDown)
	if m.previewOffset != 0 {
		t.Errorf("previewOffset on dir = %d, want 0", m.previewOffset)
	}
}

func TestPreviewScrollbar(t *testing.T) {
	// Content that fits needs no scrollbar.
	if got := previewScrollbar(0, 10, 8); got != nil {
		t.Errorf("fitting content scrollbar = %v, want nil", got)
	}

	// Overflowing content has a track with exactly view rows.
	cols := previewScrollbar(0, 10, 100)
	if cols == nil || len(cols) != 10 {
		t.Fatalf("scrollbar len = %v, want 10", len(cols))
	}

	// The thumb is a contiguous block.
	inThumb := false
	seenGap := false
	for _, c := range cols {
		if c == "┃" {
			if seenGap {
				t.Fatalf("non-contiguous thumb: %v", cols)
			}
			inThumb = true
		} else if inThumb {
			seenGap = true
		}
	}
	if !inThumb {
		t.Fatalf("scrollbar has no thumb: %v", cols)
	}

	// A later offset moves the thumb down.
	cols2 := previewScrollbar(80, 10, 100)
	moved := false
	for i := range cols {
		if cols[i] == "┃" && cols2[i] != "┃" {
			moved = true
		}
	}
	if !moved {
		t.Errorf("thumb did not move with offset: %v -> %v", cols, cols2)
	}
}

func TestPreviewRendersFromOffset(t *testing.T) {
	content := make([]byte, 0, 200)
	for i := 1; i <= 30; i++ {
		content = append(content, []byte(fmt.Sprintf("line %d", i))...)
		if i < 30 {
			content = append(content, '\n')
		}
	}
	m := newModel(BuildTree([]Item{{Path: "big.go", Content: content}}), Options{})
	m.height = 12
	m.width = 80

	m = updateKey(m, tea.KeyPgDown)
	view := m.renderPreviewBox(40, m.height-m.footerHeight())
	if !strings.Contains(view, "│ ") {
		t.Fatal("preview missing line numbers")
	}
	// The viewport must not start at line 1 anymore (offset > 0 renders a
	// later line, which won't be the first content line).
	first := m.previewOffset + 1
	if !strings.Contains(view, fmt.Sprintf("%3d │", first)) {
		t.Errorf("preview view missing line %d after scrolling", first)
	}
}

func TestPreviewScrollbarStaysOnContentRow(t *testing.T) {
	content := []byte("short\n" + strings.Repeat("x", 180) + "\n\twide\t世界\n" + strings.Repeat("line\n", 40))
	for _, width := range []int{32, 40, 56, 80} {
		m := newModel(BuildTree([]Item{{Path: "wide.txt", Content: content}}), Options{})
		m.width, m.height = width, 16
		view := m.renderPreviewBox(width, m.bodyHeight())
		for lineNo, line := range strings.Split(ansi.Strip(view), "\n") {
			if got := ansi.StringWidth(line); got > width {
				t.Fatalf("width %d row %d is %d cells: %q", width, lineNo, got, line)
			}
		}
	}
}
