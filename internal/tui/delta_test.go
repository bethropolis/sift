package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestDeltaModalOpenAndClose(t *testing.T) {
	opts := Options{Delta: &DeltaInfo{
		FromHash: "aaa", FromMsg: "baseline",
		HeadHash: "bbb", HeadMsg: "head",
		Commits: []DeltaCommit{{Short: "bbb", Subject: "head"}, {Short: "aaa", Subject: "baseline"}},
	}}
	m := newModel(BuildTree([]Item{{Path: "a.go"}}), opts)

	m = updateKey(m, tea.KeyRunes, 'd')
	if !m.deltaOpen {
		t.Fatal("d did not open the delta modal")
	}
	if view := m.View(); !strings.Contains(view, "Incremental Delta") {
		t.Errorf("modal view missing title: %q", view)
	}

	m = updateKey(m, tea.KeyEsc)
	if m.deltaOpen {
		t.Error("esc did not close the delta modal")
	}
}

func TestDeltaModalUnavailable(t *testing.T) {
	m := newModel(BuildTree([]Item{{Path: "a.go"}}), Options{})
	m = updateKey(m, tea.KeyRunes, 'd')
	if m.deltaOpen {
		t.Error("delta modal opened without Delta info")
	}
	if m.notice == "" {
		t.Error("expected a notice when delta is unavailable")
	}
}

func TestDeltaRangeFromCheckedCommits(t *testing.T) {
	// Newest-first: HEAD, second, baseline(oldest).
	m := newModel(BuildTree([]Item{{Path: "a.go"}}), Options{Delta: &DeltaInfo{
		FromHash: "base",
		HeadHash: "head",
		Commits: []DeltaCommit{
			{Short: "head", Subject: "head", Checked: true},
			{Short: "mid", Subject: "second", Checked: true},
			{Short: "base", Subject: "baseline", Checked: true},
		},
	}})

	from, to, ok := m.deltaRange()
	if !ok || from != "base" || to != "head" {
		t.Errorf("all checked: from=%q to=%q ok=%v", from, to, ok)
	}

	// Uncheck HEAD: newest checked is mid, so to = mid.
	m.delta.Commits[0].Checked = false
	from, to, ok = m.deltaRange()
	if !ok || from != "base" || to != "mid" {
		t.Errorf("unchecked head: from=%q to=%q", from, to)
	}

	// Uncheck the oldest two: oldest checked is head, from = mid.
	m.delta.Commits[0].Checked = true
	m.delta.Commits[2].Checked = false
	m.delta.Commits[1].Checked = false
	from, to, ok = m.deltaRange()
	if !ok || from != "mid" || to != "head" {
		t.Errorf("unchecked oldest: from=%q to=%q", from, to)
	}

	// Nothing checked: not ok.
	m.delta.Commits[0].Checked = false
	if _, _, ok := m.deltaRange(); ok {
		t.Error("empty selection should not be ok")
	}
}

func TestDeltaPerformInvokesHandler(t *testing.T) {
	var got DeltaSelection
	handled := false
	opts := Options{
		Delta: &DeltaInfo{
			FromHash: "base", HeadHash: "head",
			Commits: []DeltaCommit{{Short: "head", Subject: "head", Checked: true}, {Short: "base", Subject: "base", Checked: true}},
		},
		OnDelta: func(sel DeltaSelection) error {
			got = sel
			handled = true
			return nil
		},
	}
	m := newModel(BuildTree([]Item{{Path: "a.go"}}), opts)
	m = updateKey(m, tea.KeyRunes, 'd')
	m = updateKey(m, tea.KeyEnter)

	if !handled {
		t.Fatal("OnDelta was not called")
	}
	if got.Strategy != DeltaFull || got.From != "base" || got.To != "head" {
		t.Errorf("selection = %+v", got)
	}
	if !m.quit {
		t.Error("delta dump should quit the picker")
	}

	// 'c' copy keeps the picker open.
	m = newModel(BuildTree([]Item{{Path: "a.go"}}), opts)
	m = updateKey(m, tea.KeyRunes, 'd')
	m = updateKey(m, tea.KeyRunes, 'c')
	if !handled || m.quit {
		t.Errorf("copy delta quit=%v", m.quit)
	}
	if got.Clipboard != true {
		t.Errorf("copy selection Clipboard = %v", got.Clipboard)
	}
}

func TestDeltaPerformErrorSetsNotice(t *testing.T) {
	opts := Options{
		Delta:   &DeltaInfo{FromHash: "base", HeadHash: "head"},
		OnDelta: func(sel DeltaSelection) error { return fmt.Errorf("boom") },
	}
	m := newModel(BuildTree([]Item{{Path: "a.go"}}), opts)
	m = updateKey(m, tea.KeyRunes, 'd')
	m = updateKey(m, tea.KeyEnter)

	if !strings.Contains(m.notice, "boom") {
		t.Errorf("notice = %q, want delta failed message", m.notice)
	}
	if m.deltaOpen {
		t.Error("modal should close on error")
	}
}

func TestDeltaStrategyToggle(t *testing.T) {
	opts := Options{
		Delta: &DeltaInfo{FromHash: "base", HeadHash: "head", Commits: []DeltaCommit{{Short: "head", Checked: true}}},
	}
	m := newModel(BuildTree([]Item{{Path: "a.go"}}), opts)
	m = updateKey(m, tea.KeyRunes, 'd')
	m = updateKey(m, tea.KeyRunes, 'm')
	if m.deltaStrategy != DeltaPatch {
		t.Errorf("strategy = %v, want DeltaPatch", m.deltaStrategy)
	}
}
