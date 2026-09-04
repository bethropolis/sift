package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// DeltaInfo carries the git delta state the picker shows in its delta modal.
// It is nil (or DeltaInfo.Available is false) when the directory is not a git
// repository or no dump baseline has been recorded.
type DeltaInfo struct {
	RootDir    string
	FromHash   string
	FromMsg    string
	HeadHash   string
	HeadMsg    string
	Commits    []DeltaCommit // newest first; Checked selects how far back to go
	Files      []string      // changed files from FromHash..HEAD
	FilesToken int           // token estimate for the full-content strategy
	PatchToken int           // token estimate for the raw-patch strategy
	PatchLines int           // approximate line count of the raw patch
}

// DeltaCommit is one selectable commit shown in the delta modal.
type DeltaCommit struct {
	Short   string
	Subject string
	Checked bool
}

// DeltaStrategy selects how a delta dump is rendered.
type DeltaStrategy int

const (
	// DeltaFull dumps the full content of the changed files.
	DeltaFull DeltaStrategy = iota
	// DeltaPatch dumps the raw unified diff in a context_update block.
	DeltaPatch
)

// DeltaSelection describes a delta dump the user confirmed in the modal.
type DeltaSelection struct {
	Strategy  DeltaStrategy
	From      string
	To        string
	Clipboard bool
}

// newDeltaInfo returns a default DeltaInfo for the modal.
func newDeltaInfo() *DeltaInfo {
	return &DeltaInfo{}
}

// openDelta opens the delta modal, resetting the commit selection to include
// every commit since the last dump. Outside a git repo with a baseline it
// shows an explanatory notice instead.
func (m *model) openDelta() {
	if m.delta == nil {
		m.notice = "Delta mode needs a git repo with a recorded dump baseline"
		return
	}
	m.deltaOpen = true
	m.deltaCursor = 0
	m.deltaOffset = 0
	m.deltaStrategy = DeltaFull
	for i := range m.delta.Commits {
		m.delta.Commits[i].Checked = true
	}
}

// performDelta invokes the caller's delta handler with the current selection.
// copyMode routes output to the clipboard instead of the dump destination.
// The checked commits determine the range: the newest checked commit is the
// To boundary and the parent of the oldest checked commit the From boundary,
// so unchecking the top or bottom of the list narrows the delta.
func (m *model) performDelta(copyMode bool) {
	if m.delta == nil || m.onDelta == nil {
		m.notice = "Delta mode is unavailable"
		m.deltaOpen = false
		return
	}
	sel := DeltaSelection{
		Strategy:  m.deltaStrategy,
		From:      m.delta.FromHash,
		To:        m.delta.HeadHash,
		Clipboard: copyMode,
	}
	if from, to, ok := m.deltaRange(); ok {
		sel.From, sel.To = from, to
	}
	if err := m.onDelta(sel); err != nil {
		m.notice = "Delta failed: " + err.Error()
		m.deltaOpen = false
		return
	}
	m.deltaDone = true
	m.deltaOpen = false
	if !copyMode {
		// A delta dump replaces the picker's output, so finish the session.
		m.quit = true
	} else {
		m.notice = fmt.Sprintf("Delta copied to clipboard (%s)", strategyName(m.deltaStrategy))
	}
}

func strategyName(s DeltaStrategy) string {
	if s == DeltaPatch {
		return "patch"
	}
	return "full content"
}

// deltaRange computes the effective from/to from the checked commits. The
// commits slice is newest-first, so commit[i]'s parent is commit[i+1] (or the
// recorded baseline for the oldest entry).
func (m model) deltaRange() (from, to string, ok bool) {
	if m.delta == nil {
		return "", "", false
	}
	newest := -1
	oldest := -1
	for i, c := range m.delta.Commits {
		if c.Checked {
			if newest == -1 {
				newest = i
			}
			oldest = i
		}
	}
	if newest == -1 {
		return "", "", false
	}

	to = m.delta.Commits[newest].Short
	from = m.delta.FromHash
	if oldest+1 < len(m.delta.Commits) {
		from = m.delta.Commits[oldest+1].Short
	}
	if to == "" {
		to = m.delta.HeadHash
	}
	return from, to, true
}

// renderDeltaModal overlays the delta selection modal centered on the view.
func (m model) renderDeltaModal(view string, width, height int) string {
	modalWidth := min(width, 68)
	if modalWidth < 30 {
		modalWidth = 30
	}

	var b strings.Builder
	title := fmt.Sprintf(" %sIncremental Delta ", m.glyphs.Delta)
	b.WriteString(m.styles.title.Render(ansi.Truncate(title, modalWidth-4, "…")))
	b.WriteString("\n")

	if m.delta == nil {
		b.WriteString(m.styles.hint.Render("Not available outside a git repository."))
		return m.overlay(view, m.boxStyle(modalWidth).Render(b.String()), width, height)
	}

	b.WriteString(m.styles.muted.Render(fmt.Sprintf("Project: %s", m.delta.RootDir)))
	b.WriteString("\n")
	b.WriteString(m.styles.dim.Render(fmt.Sprintf("Baseline: %s  %s",
		m.delta.FromHash, truncateString(m.delta.FromMsg, modalWidth-22))))
	b.WriteString("\n")
	b.WriteString(m.styles.dim.Render(fmt.Sprintf("HEAD:     %s  %s",
		m.delta.HeadHash, truncateString(m.delta.HeadMsg, modalWidth-22))))
	b.WriteString("\n\n")
	b.WriteString(m.styles.title.Render("Commits since baseline:"))
	b.WriteString("\n")

	// Scrollable commit list, newest first.
	innerRows := max(1, height-16)
	end := min(len(m.delta.Commits), m.deltaOffset+innerRows)
	for i := m.deltaOffset; i < end; i++ {
		c := m.delta.Commits[i]
		var checkMark string
		if c.Checked {
			checkMark = m.styles.modeFull.Render("[✓]")
		} else {
			checkMark = m.styles.dim.Render("[ ]")
		}
		line := fmt.Sprintf("%s %s  %s", checkMark, c.Short, c.Subject)
		line = truncateString(line, modalWidth-4)
		if i == m.deltaCursor {
			b.WriteString(m.styles.cursor.Render(line))
		} else {
			b.WriteString(line)
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(m.styles.title.Render("Output mode:"))
	b.WriteString("\n")

	if m.deltaStrategy == DeltaFull {
		b.WriteString(m.styles.modeFull.Render("◉ Full content"))
		b.WriteString(m.styles.dim.Render(fmt.Sprintf("  (%d files · %d tok)", len(m.delta.Files), m.delta.FilesToken)))
		b.WriteString("\n")
		b.WriteString(m.styles.dim.Render("○ Git patch diff"))
		b.WriteString(m.styles.dim.Render(fmt.Sprintf("  (%d lines · %d tok)", m.delta.PatchLines, m.delta.PatchToken)))
	} else {
		b.WriteString(m.styles.dim.Render("○ Full content"))
		b.WriteString(m.styles.dim.Render(fmt.Sprintf("  (%d files · %d tok)", len(m.delta.Files), m.delta.FilesToken)))
		b.WriteString("\n")
		b.WriteString(m.styles.modeFull.Render("◉ Git patch diff"))
		b.WriteString(m.styles.dim.Render(fmt.Sprintf("  (%d lines · %d tok)", m.delta.PatchLines, m.delta.PatchToken)))
	}
	b.WriteString("\n\n")
	b.WriteString(m.styles.hint.Render("Space toggle · m mode · Enter dump · c copy · Esc cancel"))

	modal := m.boxStyle(modalWidth).Render(b.String())
	return m.overlay(view, modal, width, height)
}
