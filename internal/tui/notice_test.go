package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

// runGenJob executes the command returned by a generate/copy action the way
// Bubble Tea would: unwrap the batch, run the job and its progress tick,
// deliver both messages, and return the model plus the follow-up command
// (the notice-clear timer) without running it. Job actions are asynchronous,
// so tests must drive the returned command to observe their outcome.
func runGenJob(t *testing.T, m model, cmd tea.Cmd) (model, tea.Cmd) {
	t.Helper()
	if cmd == nil {
		t.Fatal("expected a command from the generate/copy action")
	}
	msg := cmd()
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		t.Fatalf("expected BatchMsg from job command, got %T", msg)
	}
	var follow tea.Cmd
	for _, sub := range batch {
		subMsg := sub()
		if subMsg == nil {
			continue
		}
		updated, next := m.Update(subMsg)
		m = updated.(model)
		if next != nil {
			follow = next
		}
	}
	return m, follow
}

func TestCopyNotice(t *testing.T) {
	root := BuildTree([]Item{{Path: "a.go", TokensFull: 10}})
	m := newModel(root, Options{OnCopy: func(sel []Selection) error { return nil }})
	m = updateKey(m, tea.KeyRunes, 'a')
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	m = updated.(model)
	if m.notice == "" {
		t.Fatal("notice empty after y")
	}
	if !strings.Contains(m.notice, "Copying 1 file") {
		t.Errorf("in-flight notice = %q, want it to name the running copy", m.notice)
	}
	m, _ = runGenJob(t, m, cmd)
	if !strings.Contains(m.notice, "Copied 1 file") {
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

func TestNoticeVisualSemantics(t *testing.T) {
	m := newModel(BuildTree([]Item{{Path: "a.go"}}), Options{UseNerd: true})
	m.width = 120
	tests := []struct {
		notice string
		glyph  string
	}{
		{"Copied 1 file", "✓"},
		{"Generate failed", m.glyphs.Warning},
		{"Rescan complete", "↻"},
		{"Nothing selected", m.glyphs.Warning},
	}
	for _, tt := range tests {
		m.notice = tt.notice
		line := ansi.Strip(strings.Split(ansi.Strip(m.renderFooter(m.width)), "\n")[1])
		want := strings.TrimSpace(tt.glyph) + " " + tt.notice
		if !strings.Contains(line, want) {
			t.Errorf("notice %q rendered as %q, want %q", tt.notice, line, want)
		}
	}
}

func TestNoticeClearedByTimer(t *testing.T) {
	root := BuildTree([]Item{{Path: "a.go", TokensFull: 10}})
	m := newModel(root, Options{OnGenerate: func(sel []Selection) error { return nil }})
	m = updateKey(m, tea.KeyRunes, 'a')
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	m = updated.(model)
	if cmd == nil {
		t.Fatal("expected a job command from g")
	}
	if m.notice == "" {
		t.Fatal("notice not set after g")
	}

	// Run the job to completion; it schedules the clear-notice timer.
	m, clearTimer := runGenJob(t, m, cmd)
	if clearTimer == nil {
		t.Fatal("expected a clear-notice timer from job completion")
	}
	if m.notice == "" {
		t.Fatal("notice missing after job completion")
	}

	// Advance the timer: the clearNoticeMsg clears the notice.
	msgs := clearTimer()
	if msgs == nil {
		t.Fatal("expected a clearNoticeMsg from the tick")
	}
	if _, ok := msgs.(clearNoticeMsg); !ok {
		t.Fatalf("timer produced %T, want clearNoticeMsg", msgs)
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

func TestSmartSelectNotice(t *testing.T) {
	root := BuildTree([]Item{{Path: "a.go", TokensFull: 10}})
	m := newModel(root, Options{})
	m = updateKey(m, tea.KeyRunes, 's')
	if !strings.Contains(m.notice, "Smart selection") {
		t.Errorf("smart select notice = %q", m.notice)
	}
	if !strings.Contains(m.notice, "1 file") {
		t.Errorf("smart select notice = %q", m.notice)
	}
	if m.root.SelectedCount() == 0 {
		t.Error("smart select selected nothing")
	}
}
