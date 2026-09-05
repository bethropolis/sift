package tui

import (
	"fmt"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
)

// rescan.go implements the "r" refresh: re-running the background scan
// (fresh metadata walk, git-history analysis, content walk, rank patch) and
// reconciling the live tree against it. Selections, per-file modes, and
// expanded state are preserved by path; only paths the fresh walk never
// reported are dropped once every scan channel is exhausted.

// rescan starts a fresh background scan, swapping in the new stream and
// resetting progress state. It snapshots the current tree so paths the new
// walk never reports can be pruned when the scan fully drains.
func (m *model) rescan() tea.Cmd {
	if m.onRescan == nil {
		return m.setNotice("Rescan is not available in this session")
	}
	if !m.scanDone {
		return m.setNotice("Scan in progress — try again when it finishes")
	}
	stream, err := m.onRescan()
	if err != nil {
		return m.setNotice("Rescan failed: " + err.Error())
	}
	m.rescanActive = true
	m.rescanBaseline = make(map[string]bool, len(m.nodeIndex))
	for p := range m.nodeIndex {
		m.rescanBaseline[p] = true
	}
	m.rescanSeen = make(map[string]bool)
	m.scanFiles, m.scanDirs, m.scanProcessed = 0, 0, 0
	m.scanDone = false
	m.streamNodesClosed, m.streamProgressClosed, m.streamErrClosed = false, false, false
	m.stream = stream
	return tea.Batch(
		m.listenStream(),
		m.setNotice("Rescanning files and git history…"),
	)
}

// trackRescanSeen records every path the fresh walk reports. Only paths
// missing from this set at drain time count as deleted.
func (m *model) trackRescanSeen(items []Item) {
	if !m.rescanActive {
		return
	}
	for i := range items {
		m.rescanSeen[filepath.ToSlash(items[i].Path)] = true
	}
}

// abortRescan drops rescan bookkeeping without pruning, used when the fresh
// scan errors out partway: the tree may be partial, so existing nodes stay.
func (m *model) abortRescan() {
	m.rescanActive = false
	m.rescanBaseline = nil
	m.rescanSeen = nil
}

// finishRescan prunes baseline paths the fresh walk never reported and
// reports completion. It runs only once every scan channel is exhausted, so
// no in-flight batch can resurrect a pruned node afterwards.
func (m *model) finishRescan() tea.Cmd {
	if !m.rescanActive {
		return nil
	}
	m.rescanActive = false
	keepPath := ""
	if n := m.node(); n != nil {
		keepPath = n.Path
	}
	var files []string
	var dirs []string
	for p := range m.rescanBaseline {
		if m.rescanSeen[p] {
			continue
		}
		n, ok := m.nodeIndex[p]
		if !ok {
			continue
		}
		if n.Kind == KindFile {
			files = append(files, p)
		} else {
			dirs = append(dirs, p)
		}
	}
	m.removeNodes(files)
	// Drop directories left childless by the prune, repeatedly: a renamed
	// directory surfaces as an unseen empty shell once its files are gone.
	for {
		var empty []string
		for _, p := range dirs {
			if n, ok := m.nodeIndex[p]; ok && len(n.Children) == 0 {
				empty = append(empty, p)
			}
		}
		if len(empty) == 0 {
			break
		}
		m.removeNodes(empty)
	}
	m.rescanBaseline = nil
	m.rescanSeen = nil
	m.recomputeRows()
	if keepPath != "" {
		if idx := m.findRow(keepPath); idx >= 0 {
			m.cursor = idx
			m.clampOffset()
		}
	}
	return m.setNotice(fmt.Sprintf("Rescan complete: %d files", m.scanFiles))
}
