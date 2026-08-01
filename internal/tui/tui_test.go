package tui

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func TestSelectAndResult(t *testing.T) {
	items := []Item{
		{Path: "a.go", TokensFull: 10, TokensSig: 5},
		{Path: "b.go", TokensFull: 20, TokensSig: 8},
		{Path: "c.go", TokensFull: 30, TokensSig: 12},
	}
	m := newModel(BuildTree(items), Options{})

	// Select the second item (row 1 after "a.go").
	m = updateKey(m, tea.KeyDown)
	m = updateKey(m, tea.KeySpace)

	// Select everything via 'a'.
	m = updateKey(m, tea.KeyRunes, 'a')

	got := m.root.Selections()
	want := []Selection{
		{Path: "a.go", Mode: ModeFull},
		{Path: "b.go", Mode: ModeFull},
		{Path: "c.go", Mode: ModeFull},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Selections = %v, want %v", got, want)
	}

	// 'a' again deselects all.
	m = updateKey(m, tea.KeyRunes, 'a')
	if got := m.root.Selections(); len(got) != 0 {
		t.Errorf("after toggle-all-off Selections = %v, want empty", got)
	}
}

func TestTreeToggleDirectory(t *testing.T) {
	root := BuildTree([]Item{
		{Path: "cmd/a.go", TokensFull: 10},
		{Path: "internal/b.go", TokensFull: 20},
		{Path: "internal/c.go", TokensFull: 30},
	})
	cmd := root.findChild("cmd")
	internal := root.findChild("internal")

	internal.Toggle()
	if internal.SelectState != Selected {
		t.Errorf("internal SelectState = %v, want Selected", internal.SelectState)
	}
	if root.SelectState != Partial {
		t.Errorf("root SelectState = %v, want Partial", root.SelectState)
	}
	if got := len(root.Selections()); got != 2 {
		t.Errorf("selections = %d, want 2", got)
	}

	cmd.Toggle()
	if root.SelectState != Selected {
		t.Errorf("root SelectState = %v, want Selected", root.SelectState)
	}
}

func TestTreePrefix(t *testing.T) {
	root := BuildTree([]Item{
		{Path: "cmd/main.go"},
		{Path: "internal/app/app.go"},
	})
	glyphs := NewNerdFontGlyphs()

	cmd := root.findChild("cmd")
	if cmd == nil {
		t.Fatal("missing cmd node")
	}
	pCmd := cmd.TreePrefix(glyphs)
	if pCmd != glyphs.TreeMiddle {
		t.Errorf("cmd TreePrefix = %q, want %q", pCmd, glyphs.TreeMiddle)
	}

	mainGo := cmd.findChild("main.go")
	if mainGo == nil {
		t.Fatal("missing main.go node")
	}
	pMain := mainGo.TreePrefix(glyphs)
	wantMain := glyphs.TreePipe + glyphs.TreeLast
	if pMain != wantMain {
		t.Errorf("main.go TreePrefix = %q, want %q", pMain, wantMain)
	}
}

func TestModePropagation(t *testing.T) {
	root := BuildTree([]Item{
		{Path: "internal/b.go", TokensFull: 20, TokensSig: 8},
		{Path: "internal/c.go", TokensFull: 30, TokensSig: 12},
	})
	internal := root.findChild("internal")
	internal.setSelected(true)

	internal.CycleMode() // full -> signatures
	if internal.Mode != ModeSignatures {
		t.Errorf("dir Mode = %v, want signatures", internal.Mode)
	}
	for _, c := range internal.Children {
		if c.Mode != ModeSignatures {
			t.Errorf("child %s Mode = %v, want signatures", c.Path, c.Mode)
		}
	}
	if got := internal.TotalActiveTokens(); got != 20 {
		t.Errorf("TotalActiveTokens = %d, want 20 (8+12)", got)
	}

	internal.CycleMode() // signatures -> skip
	if got := root.Selections(); len(got) != 0 {
		t.Errorf("skipped dir still selected: %v", got)
	}
}

func TestSmartSelectByRank(t *testing.T) {
	root := BuildTree([]Item{
		{Path: "low.go", TokensFull: 100, RankScore: 0.1},
		{Path: "mid.go", TokensFull: 50, RankScore: 0.5},
		{Path: "hot.go", TokensFull: 30, RankScore: 1.0},
	})
	count := root.SelectByRank(100)
	if count != 2 {
		t.Errorf("SelectByRank = %d, want 2", count)
	}
	got := root.Selections()
	if len(got) != 2 || got[0].Path != "hot.go" || got[1].Path != "mid.go" {
		t.Errorf("rank selection = %v, want hot.go, mid.go", got)
	}
}

func TestFilterVisibleRows(t *testing.T) {
	root := BuildTree([]Item{
		{Path: "internal/app/app.go", TokensFull: 10},
		{Path: "internal/compress/compress.go", TokensFull: 10},
		{Path: "cmd/main.go", TokensFull: 10},
	})
	rows := root.VisibleRows("compress")
	if len(rows) != 3 {
		t.Fatalf("filtered rows = %d, want 3 (ancestors kept)", len(rows))
	}
	last := rows[len(rows)-1]
	if last.Path != "internal/compress/compress.go" {
		t.Errorf("filtered leaf = %s", last.Path)
	}
	for _, r := range rows {
		if r.Path == "internal/app/app.go" || r.Path == "cmd/main.go" {
			t.Errorf("filter leaked non-matching row %s", r.Path)
		}
	}
}

func TestCursorClamping(t *testing.T) {
	m := newModel(BuildTree([]Item{
		{Path: "a.go"}, {Path: "b.go"}, {Path: "c.go"}, {Path: "d.go"}, {Path: "e.go"},
	}), Options{})
	m.height = 5

	for i := 0; i < 10; i++ {
		m = updateKey(m, tea.KeyDown)
	}
	if m.cursor != 4 {
		t.Errorf("cursor = %d, want 4", m.cursor)
	}
}

func TestDirsPreCollapsed(t *testing.T) {
	root := BuildTree([]Item{
		{Path: "cmd/main.go"},
		{Path: "internal/app/app.go"},
	})
	if root.Expanded != true {
		t.Errorf("root Expanded = %v, want true", root.Expanded)
	}
	for _, child := range root.Children {
		if child.Kind == KindDir && child.Expanded {
			t.Errorf("dir %s Expanded = true, want pre-collapsed", child.Path)
		}
	}
	// Only top-level rows visible initially.
	if rows := root.VisibleRows(""); len(rows) != 2 {
		t.Errorf("initial visible rows = %d, want 2", len(rows))
	}
}

func TestViewRendersRows(t *testing.T) {
	m := newModel(BuildTree([]Item{{Path: "a.go", TokensFull: 10}}), Options{})
	if view := m.View(); view == "" {
		t.Fatal("empty view")
	}
}

func TestCardHeightsEqual(t *testing.T) {
	items := make([]Item, 30)
	for i := 0; i < 30; i++ {
		items[i] = Item{Path: fmt.Sprintf("file_%d.go", i), Content: []byte("package main\nfunc main() {}\n")}
	}
	m := newModel(BuildTree(items), Options{})
	m.height = 24
	m.width = 80
	bodyHeight := max(5, m.height-m.footerHeight())

	leftBox := m.renderTreeBox(40, bodyHeight)
	rightBox := m.renderPreviewBox(40, bodyHeight)

	if lipgloss.Height(leftBox) != lipgloss.Height(rightBox) {
		t.Errorf("height mismatch! leftBox=%d, rightBox=%d", lipgloss.Height(leftBox), lipgloss.Height(rightBox))
	}
}

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
	if view := m.View(); !strings.Contains(view, "Delta Mode") {
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

func updateKey(m model, kt tea.KeyType, runes ...rune) model {
	msg := tea.KeyMsg{Type: kt, Runes: runes}
	updated, _ := m.Update(msg)
	return updated.(model)
}
