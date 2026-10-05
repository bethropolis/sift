package tui

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// jobKey presses a single-rune key and returns the updated model and command.
func jobKey(t *testing.T, m model, r rune) (model, tea.Cmd) {
	t.Helper()
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	return updated.(model), cmd
}

// jobDoneMsg wraps a job result the way startGenJob's goroutine would.
func jobDone(text string) tea.Cmd {
	return func() tea.Msg { return genJobMsg{text: text} }
}

// TestGenerateErrorSurfacesAsNotice proves a failing callback reports its
// error as a notice instead of panicking or hanging the picker.
func TestGenerateErrorSurfacesAsNotice(t *testing.T) {
	root := BuildTree([]Item{{Path: "a.go", TokensFull: 10}})
	m := newModel(root, Options{
		OnGenerate: func(sel []Selection) error { return errors.New("disk on fire") },
	})
	m = updateKey(m, tea.KeyRunes, 'a')
	m, cmd := jobKey(t, m, 'g')
	m, _ = runGenJob(t, m, cmd)
	if !strings.Contains(m.notice, "Generate failed: disk on fire") {
		t.Errorf("notice = %q, want the callback error", m.notice)
	}
	if m.genBusy {
		t.Error("genBusy still set after the job reported back")
	}
}

// TestCopyErrorSurfacesAsNotice covers the copy half of the async path.
func TestCopyErrorSurfacesAsNotice(t *testing.T) {
	root := BuildTree([]Item{{Path: "a.go", TokensFull: 10}})
	m := newModel(root, Options{
		OnCopy: func(sel []Selection) error { return errors.New("no clipboard") },
	})
	m = updateKey(m, tea.KeyRunes, 'a')
	m, cmd := jobKey(t, m, 'y')
	m, _ = runGenJob(t, m, cmd)
	if !strings.Contains(m.notice, "Copy failed: no clipboard") {
		t.Errorf("notice = %q, want the callback error", m.notice)
	}
}

// TestGenerateAndCopyErrorSurfacesAsNotice covers uppercase Y's failure path.
func TestGenerateAndCopyErrorSurfacesAsNotice(t *testing.T) {
	root := BuildTree([]Item{{Path: "a.go", TokensFull: 10}})
	m := newModel(root, Options{
		OnGenerateCopy: func(_ []Selection, prompt string) error {
			return errors.New("render failed")
		},
	})
	m = updateKey(m, tea.KeyRunes, 'a')
	m, cmd := jobKey(t, m, 'Y')
	m, _ = runGenJob(t, m, cmd)
	if !strings.Contains(m.notice, "Generate and copy failed: render failed") {
		t.Errorf("notice = %q, want the callback error", m.notice)
	}
}

// TestGenerateAndCopyRendersOnce pins the single-render contract: one Y press
// must render and copy exactly once.
func TestGenerateAndCopyRendersOnce(t *testing.T) {
	calls := 0
	root := BuildTree([]Item{{Path: "a.go", TokensFull: 10}})
	m := newModel(root, Options{
		OnGenerateCopy: func(_ []Selection, prompt string) error {
			calls++
			return nil
		},
	})
	m = updateKey(m, tea.KeyRunes, 'a')
	m, cmd := jobKey(t, m, 'Y')
	m, _ = runGenJob(t, m, cmd)
	if calls != 1 {
		t.Errorf("generate-and-copy callback ran %d times, want 1", calls)
	}
	if !strings.Contains(m.notice, "Generated and copied codebase.md") {
		t.Errorf("notice = %q", m.notice)
	}
}

// TestBusyRejectsDuplicateJobs verifies the genBusy guard: while a job is in
// flight, further generate/copy presses are acknowledged instead of stacking a
// second unbounded render on the same selection.
func TestBusyRejectsDuplicateJobs(t *testing.T) {
	calls := 0
	block := make(chan struct{})
	root := BuildTree([]Item{{Path: "a.go", TokensFull: 10}})
	m := newModel(root, Options{
		OnGenerate: func(sel []Selection) error {
			calls++
			<-block
			return nil
		},
		OnCopy: func(sel []Selection) error {
			calls++
			<-block
			return nil
		},
	})
	m = updateKey(m, tea.KeyRunes, 'a')
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	m = updated.(model)
	if !m.genBusy {
		t.Fatal("genBusy not set while the job runs")
	}
	if m.genDone == nil {
		t.Fatal("genDone channel not published for the exit wait")
	}

	// Every other generate/copy action must be refused while busy.
	for _, r := range []rune{'g', 'y', 'Y'} {
		m, cmd2 := jobKey(t, m, r)
		if !strings.Contains(m.notice, "already running") {
			t.Errorf("%q while busy set notice = %q, want the busy refusal", r, m.notice)
		}
		if cmd2 == nil {
			t.Errorf("%q while busy returned no command; want a clear-notice timer", r)
		}
	}
	if calls != 0 {
		t.Errorf("callbacks ran %d times before the job command ran, want 0", calls)
	}
	if m.genDone == nil {
		t.Error("genDone replaced during the busy refusal")
	}

	// Let the in-flight job finish and confirm the model recovers.
	close(block)
	m, _ = runGenJob(t, m, cmd)
	if m.genBusy {
		t.Error("genBusy still set after the job finished")
	}
	if m.genDone != nil {
		t.Error("genDone not cleared after the job finished")
	}
	if calls != 1 {
		t.Errorf("callbacks ran %d times, want 1", calls)
	}

	// A new job is accepted once the previous one has reported back.
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	if !updated.(model).genBusy {
		t.Error("generate refused after the previous job completed")
	}
}

// TestTickerUpdatesNoticeWhileBusy proves the progress ticker refreshes the
// footer line while a job runs, and that a stale tick cannot resurrect it once
// the job has reported back.
func TestTickerUpdatesNoticeWhileBusy(t *testing.T) {
	block := make(chan struct{})
	defer close(block)
	root := BuildTree([]Item{{Path: "a.go", TokensFull: 10}})
	m := newModel(root, Options{
		OnGenerate: func(sel []Selection) error {
			<-block
			return nil
		},
	})
	m = updateKey(m, tea.KeyRunes, 'a')
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	m = updated.(model)

	before := m.notice
	if !strings.Contains(before, "Generating 1 file") {
		t.Fatalf("in-flight notice = %q", before)
	}
	updated, next := m.Update(busyTickMsg{
		id:      m.noticeID,
		label:   "Generating 1 file…",
		started: time.Now().Add(-2 * time.Second),
	})
	m = updated.(model)
	if m.notice == before {
		t.Errorf("tick did not refresh the notice (still %q)", m.notice)
	}
	if !strings.Contains(m.notice, "2s") {
		t.Errorf("notice = %q, want it to include elapsed time", m.notice)
	}
	if next == nil {
		t.Error("tick did not re-arm itself")
	}

	// Once the job lands, a stale tick must not overwrite the settled notice.
	updated, _ = m.Update(jobDone("Generated output · 1 file · 10 tokens")())
	m = updated.(model)
	if m.genBusy {
		t.Fatal("genBusy still set after the job reported back")
	}
	settled := m.notice
	updated, next = m.Update(busyTickMsg{
		id:      m.noticeID - 1,
		label:   "Generating 1 file…",
		started: time.Now().Add(-9 * time.Second),
	})
	m = updated.(model)
	if m.notice != settled {
		t.Errorf("stale tick rewrote the settled notice: %q", m.notice)
	}
	if next != nil {
		t.Error("stale tick kept re-arming after the job finished")
	}
}

// TestQuitDuringJobKeepsWaitHandle proves quitting mid-job leaves genDone
// published so runProgram can wait for the in-flight write, and that the
// channel closes once the job finishes.
func TestQuitDuringJobKeepsWaitHandle(t *testing.T) {
	block := make(chan struct{})
	root := BuildTree([]Item{{Path: "a.go", TokensFull: 10}})
	m := newModel(root, Options{
		OnGenerate: func(sel []Selection) error {
			<-block
			return nil
		},
	})
	m = updateKey(m, tea.KeyRunes, 'a')
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	m = updated.(model)
	if m.genDone == nil {
		t.Fatal("genDone not published while busy")
	}
	done := m.genDone

	m = updateKey(m, tea.KeyRunes, 'q')
	if !m.quit {
		t.Fatal("q did not quit while a job was running")
	}
	if m.genDone == nil {
		t.Fatal("quitting dropped the genDone wait handle")
	}
	// runProgram's bounded wait must still be pending at this point.
	select {
	case <-done:
		t.Fatal("genDone closed before the job finished")
	case <-time.After(10 * time.Millisecond):
	}

	// Finishing the job releases the wait handle captured before quitting.
	close(block)
	m, _ = runGenJob(t, m, cmd)
	if m.genBusy {
		t.Error("genBusy still set after the job finished")
	}
	select {
	case <-done:
	default:
		t.Error("genDone not closed after the job finished")
	}
}
