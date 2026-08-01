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

func TestFooterHeightFixedWithNotice(t *testing.T) {
	root := BuildTree([]Item{{Path: "a.go", TokensFull: 10}})
	m := newModel(root, Options{})
	if got := m.footerHeight(); got != 2 {
		t.Errorf("footerHeight() = %d, want 2", got)
	}
	m.notice = "Generated output (1 files)"
	if got := m.footerHeight(); got != 2 {
		t.Errorf("footerHeight() with notice = %d, want fixed 2", got)
	}
}

func TestNoticeClearedByTimer(t *testing.T) {
	root := BuildTree([]Item{{Path: "a.go", TokensFull: 10}})
	m := newModel(root, Options{OnGenerate: func(sel []Selection) error { return nil }})
	m = updateKey(m, tea.KeyRunes, 'a')
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	m = updated.(model)
	if cmd == nil {
		t.Fatal("expected a tick command from g")
	}
	if m.notice == "" {
		t.Fatal("notice not set after g")
	}

	// Advance the timer: the clearNoticeMsg clears the notice.
	msgs := cmd()
	if msgs == nil {
		t.Fatal("expected a clearNoticeMsg from the tick")
	}
	updated, _ = m.Update(msgs)
	if updated.(model).notice != "" {
		t.Error("notice not cleared after the timer fired")
	}
}

func TestStaleNoticeTimerIgnored(t *testing.T) {
	m := newModel(BuildTree([]Item{{Path: "a.go"}}), Options{})
	// Two notices back to back; only the latest id may clear.
	m.setNotice("first")
	m.setNotice("second")
	if m.notice != "second" {
		t.Fatalf("notice = %q, want second", m.notice)
	}
	// Deliver a stale clear for the first id.
	updated, _ := m.Update(clearNoticeMsg{id: m.noticeID - 1})
	if updated.(model).notice != "second" {
		t.Error("stale timer cleared the newer notice")
	}
	// Deliver the current id.
	updated, _ = m.Update(clearNoticeMsg{id: m.noticeID})
	if updated.(model).notice != "" {
		t.Error("current timer did not clear the notice")
	}
}

func TestSmartSelectNoNotice(t *testing.T) {
	root := BuildTree([]Item{{Path: "a.go", TokensFull: 10}})
	m := newModel(root, Options{})
	m = updateKey(m, tea.KeyRunes, 's')
	if m.notice != "" {
		t.Errorf("smart select set a notice: %q", m.notice)
	}
	if m.root.SelectedCount() == 0 {
		t.Error("smart select selected nothing")
	}
}
