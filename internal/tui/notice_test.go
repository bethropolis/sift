package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestCopyNotice(t *testing.T) {
	root := BuildTree([]Item{{Path: "a.go", TokensFull: 10}})
	m := newModel(root, Options{OnCopy: func(sel []Selection) error { return nil }})
	m = updateKey(m, tea.KeyRunes, 'a')
	m = updateKey(m, tea.KeyRunes, 'y')
	if m.notice == "" {
		t.Fatal("notice empty after y")
	}
	if !strings.Contains(m.notice, "Copied 1 files") {
		t.Errorf("notice = %q", m.notice)
	}
}
