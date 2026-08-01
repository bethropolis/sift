package tui

import (
	"fmt"
	"strings"
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
	modalWidth := min(width, 64)
	if modalWidth < 30 {
		modalWidth = 30
	}

	var b strings.Builder
	b.WriteString(" Incremental Context / Delta Mode ")
	b.WriteString("\n")

	if m.delta == nil {
		b.WriteString(hintStyle.Render("Not available outside a git repository."))
		return m.overlay(view, boxStyle(modalWidth).Render(b.String()), width, height)
	}

	b.WriteString(fmt.Sprintf("Project: %s", m.delta.RootDir))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("Last Dumped: %s %s", m.delta.FromHash, truncateString(m.delta.FromMsg, modalWidth-30)))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("HEAD:        %s %s", m.delta.HeadHash, truncateString(m.delta.HeadMsg, modalWidth-30)))
	b.WriteString("\n\n")
	b.WriteString(dimStyle.Render("Commits since last dump:"))
	b.WriteString("\n")

	// Scrollable commit list, newest first.
	innerRows := max(1, height-14)
	end := min(len(m.delta.Commits), m.deltaOffset+innerRows)
	for i := m.deltaOffset; i < end; i++ {
		c := m.delta.Commits[i]
		mark := " [ ] "
		if c.Checked {
			mark = " [x] "
		}
		line := fmt.Sprintf("%s %s %s", mark, c.Short, c.Subject)
		line = truncateString(line, modalWidth-4)
		if i == m.deltaCursor {
			b.WriteString(cursorStyle.Render(line))
		} else {
			b.WriteString(line)
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(dimStyle.Render("Delta output mode:"))
	b.WriteString("\n")

	modeFull := "( ) "
	if m.deltaStrategy == DeltaFull {
		modeFull = "(*) "
	}
	modePatch := "( ) "
	if m.deltaStrategy == DeltaPatch {
		modePatch = "(*) "
	}
	b.WriteString(fmt.Sprintf("%sFull content of modified files (%d files, %d tokens)", modeFull, len(m.delta.Files), m.delta.FilesToken))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("%sGit unified patch diff (%d lines, %d tokens)", modePatch, m.delta.PatchLines, m.delta.PatchToken))
	b.WriteString("\n\n")
	b.WriteString(hintStyle.Render("[Enter] perform delta dump | [c] copy | [Esc] cancel"))

	modal := boxStyle(modalWidth).Render(b.String())
	return m.overlay(view, modal, width, height)
}
