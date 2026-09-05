package tui

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestRescanKeyStartsRescan(t *testing.T) {
	called := false
	m := newModel(BuildTree([]Item{{Path: "a.go"}}), Options{
		OnRescan: func() (Stream, error) {
			called = true
			return Stream{}, nil
		},
	})
	m.scanDone = true
	m = updateKey(m, tea.KeyRunes, 'r')
	if !called {
		t.Fatal("r did not invoke OnRescan")
	}
	if !m.rescanActive {
		t.Error("rescanActive = false after r")
	}
	if m.scanDone {
		t.Error("scanDone = true during rescan")
	}
	if len(m.rescanBaseline) == 0 {
		t.Error("rescanBaseline is empty")
	}
	if m.notice == "" {
		t.Error("no notice set when rescan starts")
	}
}

func TestRescanKeyBlockedWhileScanning(t *testing.T) {
	called := false
	m := newModel(BuildTree([]Item{{Path: "a.go"}}), Options{
		OnRescan: func() (Stream, error) {
			called = true
			return Stream{}, nil
		},
	})
	m.scanDone = false
	m = updateKey(m, tea.KeyRunes, 'r')
	if called {
		t.Error("OnRescan invoked while a scan is in flight")
	}
	if m.rescanActive {
		t.Error("rescanActive = true while a scan is in flight")
	}
	if !strings.Contains(m.notice, "in progress") {
		t.Errorf("notice = %q, want scan-in-progress message", m.notice)
	}
}

func TestRescanUnavailableWithoutHook(t *testing.T) {
	m := newModel(BuildTree([]Item{{Path: "a.go"}}), Options{})
	m.scanDone = true
	m = updateKey(m, tea.KeyRunes, 'r')
	if m.rescanActive {
		t.Error("rescanActive = true without an OnRescan hook")
	}
	if !strings.Contains(m.notice, "not available") {
		t.Errorf("notice = %q, want unavailable message", m.notice)
	}
}

func TestRescanHookErrorKeepsTree(t *testing.T) {
	m := newModel(BuildTree([]Item{{Path: "a.go"}}), Options{
		OnRescan: func() (Stream, error) {
			return Stream{}, errors.New("walk exploded")
		},
	})
	m.scanDone = true
	m = updateKey(m, tea.KeyRunes, 'r')
	if m.rescanActive {
		t.Error("rescanActive = true after hook error")
	}
	if !m.scanDone {
		t.Error("scanDone = false after hook error")
	}
	if !strings.Contains(m.notice, "Rescan failed") {
		t.Errorf("notice = %q, want failure message", m.notice)
	}
	if _, ok := m.nodeIndex["a.go"]; !ok {
		t.Error("a.go dropped after failed rescan")
	}
}

func TestFinishRescanPrunesStalePreservesState(t *testing.T) {
	m := newModel(BuildTree([]Item{
		{Path: "keep.go", Content: []byte("package main\n")},
		{Path: "gone.go", Content: []byte("package main\n")},
		{Path: "sub/old.go", Content: []byte("package main\n")},
		{Path: "sub/stays.go", Content: []byte("package main\n")},
	}), Options{})
	m.scanDone = true
	keep := m.nodeIndex["keep.go"]
	keep.SelectState = Selected
	keep.Mode = ModeSignatures

	// Enter the reconcile window as rescan() would, then report only the
	// surviving paths from the fresh walk.
	m.rescanActive = true
	m.rescanBaseline = map[string]bool{}
	for p := range m.nodeIndex {
		m.rescanBaseline[p] = true
	}
	m.rescanSeen = map[string]bool{"keep.go": true, "sub": true, "sub/stays.go": true}
	m.cursor = m.findRow("keep.go")

	updated, _ := m.Update(streamClosedMsg{channel: streamNodesClosed})
	updated, _ = updated.(model).Update(streamClosedMsg{channel: streamProgressClosed})
	final, _ := updated.(model).Update(streamClosedMsg{channel: streamErrClosed})
	m = final.(model)

	if m.rescanActive {
		t.Error("rescanActive still true after all channels closed")
	}
	if _, ok := m.nodeIndex["gone.go"]; ok {
		t.Error("gone.go survived the rescan prune")
	}
	if _, ok := m.nodeIndex["sub/old.go"]; ok {
		t.Error("sub/old.go survived the rescan prune")
	}
	if _, ok := m.nodeIndex["keep.go"]; !ok {
		t.Fatal("keep.go pruned though the fresh walk reported it")
	}
	if got := m.nodeIndex["keep.go"].SelectState; got != Selected {
		t.Errorf("keep.go SelectState = %v, want Selected", got)
	}
	if got := m.nodeIndex["keep.go"].Mode; got != ModeSignatures {
		t.Errorf("keep.go Mode = %v, want ModeSignatures", got)
	}
	if _, ok := m.nodeIndex["sub/stays.go"]; !ok {
		t.Error("sub/stays.go pruned though the fresh walk reported it")
	}
	if !strings.Contains(m.notice, "Rescan complete") {
		t.Errorf("notice = %q, want completion message", m.notice)
	}
	if n := m.node(); n == nil || n.Path != "keep.go" {
		t.Errorf("cursor not pinned to keep.go, on %v", n)
	}
}

func TestFinishRescanDropsEmptiedDirs(t *testing.T) {
	m := newModel(BuildTree([]Item{
		{Path: "top.go"},
		{Path: "old/only.go"},
	}), Options{})
	m.scanDone = true
	m.rescanActive = true
	m.rescanBaseline = map[string]bool{}
	for p := range m.nodeIndex {
		m.rescanBaseline[p] = true
	}
	m.rescanSeen = map[string]bool{"top.go": true}

	updated, _ := m.Update(streamClosedMsg{channel: streamNodesClosed})
	updated, _ = updated.(model).Update(streamClosedMsg{channel: streamProgressClosed})
	final, _ := updated.(model).Update(streamClosedMsg{channel: streamErrClosed})
	m = final.(model)

	if _, ok := m.nodeIndex["old/only.go"]; ok {
		t.Error("old/only.go survived the rescan prune")
	}
	if _, ok := m.nodeIndex["old"]; ok {
		t.Error("emptied dir old survived the rescan prune")
	}
}

func TestFinishRescanInactiveIsNoop(t *testing.T) {
	m := newModel(BuildTree([]Item{{Path: "a.go"}}), Options{})
	if cmd := m.finishRescan(); cmd != nil {
		t.Error("finishRescan returned a cmd without an active rescan")
	}
}

func TestHelpListsRescan(t *testing.T) {
	found := false
	for _, line := range helpContentLines() {
		if strings.Contains(line, "r") && strings.Contains(line, "Rescan") {
			found = true
			break
		}
	}
	if !found {
		t.Error("help content has no rescan row")
	}
}
