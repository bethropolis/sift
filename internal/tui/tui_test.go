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

func TestPreviewScroll(t *testing.T) {
	content := make([]byte, 0, 200)
	for i := 1; i <= 50; i++ {
		content = append(content, []byte(fmt.Sprintf("line %02d\n", i))...)
	}
	m := newModel(BuildTree([]Item{{Path: "big.go", Content: content}}), Options{})
	m.height = 12
	m.width = 80

	page := m.previewPageSize()
	if page <= 0 {
		t.Fatalf("previewPageSize = %d, want > 0", page)
	}

	// Initially at the top.
	if m.previewOffset != 0 {
		t.Fatalf("initial previewOffset = %d, want 0", m.previewOffset)
	}

	// PgDn advances by a page.
	m = updateKey(m, tea.KeyPgDown)
	if m.previewOffset != page {
		t.Errorf("after PgDn previewOffset = %d, want %d", m.previewOffset, page)
	}

	// PgUp returns to the top.
	m = updateKey(m, tea.KeyPgUp)
	if m.previewOffset != 0 {
		t.Errorf("after PgUp previewOffset = %d, want 0", m.previewOffset)
	}

	// Ctrl+d advances by a half page.
	m = updateKey(m, tea.KeyCtrlD)
	if m.previewOffset != m.previewHalfPage() {
		t.Errorf("after Ctrl+d previewOffset = %d, want %d", m.previewOffset, m.previewHalfPage())
	}

	// ']' scrolls down a full page from the half-page offset.
	m = updateKey(m, tea.KeyRunes, ']')
	if m.previewOffset != m.previewHalfPage()+page {
		t.Errorf("after ] previewOffset = %d, want %d", m.previewOffset, m.previewHalfPage()+page)
	}

	// '[' scrolls back up a full page.
	m = updateKey(m, tea.KeyRunes, '[')
	if m.previewOffset != m.previewHalfPage() {
		t.Errorf("after [ previewOffset = %d, want %d", m.previewOffset, m.previewHalfPage())
	}

	// Scrolling beyond the end clamps at the last possible offset.
	total := 50
	wantMax := max(0, total-m.previewPageSize())
	for i := 0; i < 10; i++ {
		m = updateKey(m, tea.KeyPgDown)
	}
	if m.previewOffset != wantMax {
		t.Errorf("clamped previewOffset = %d, want %d", m.previewOffset, wantMax)
	}

	// Scrolling above the start clamps at zero.
	for i := 0; i < 10; i++ {
		m = updateKey(m, tea.KeyPgUp)
	}
	if m.previewOffset != 0 {
		t.Errorf("clamped previewOffset = %d, want 0", m.previewOffset)
	}
}

func TestPreviewScrollResetOnCursorMove(t *testing.T) {
	content := make([]byte, 0, 200)
	for i := 1; i <= 50; i++ {
		content = append(content, []byte(fmt.Sprintf("line %02d\n", i))...)
	}
	m := newModel(BuildTree([]Item{
		{Path: "a.go", Content: content},
		{Path: "b.go", Content: []byte("short")},
	}), Options{})
	m.height = 12
	m.width = 80

	m = updateKey(m, tea.KeyPgDown)
	if m.previewOffset == 0 {
		t.Fatal("expected previewOffset to be scrolled before moving")
	}

	// Moving to the next file resets the scroll position.
	m = updateKey(m, tea.KeyDown)
	if m.previewOffset != 0 {
		t.Errorf("previewOffset after moving = %d, want 0 (reset)", m.previewOffset)
	}
	if m.previewNode == nil || m.previewNode.Path != "b.go" {
		t.Errorf("previewNode = %+v, want b.go", m.previewNode)
	}
}

func TestPreviewScrollNoopOnDirectory(t *testing.T) {
	m := newModel(BuildTree([]Item{{Path: "dir/a.go", Content: []byte("x")}}), Options{})
	m.height = 12
	m.width = 80

	m = updateKey(m, tea.KeyPgDown)
	if m.previewOffset != 0 {
		t.Errorf("previewOffset on dir = %d, want 0", m.previewOffset)
	}
}

func TestPreviewScrollbar(t *testing.T) {
	// Content that fits needs no scrollbar.
	if got := previewScrollbar(0, 10, 8); got != nil {
		t.Errorf("fitting content scrollbar = %v, want nil", got)
	}

	// Overflowing content has a track with exactly view rows.
	cols := previewScrollbar(0, 10, 100)
	if cols == nil || len(cols) != 10 {
		t.Fatalf("scrollbar len = %v, want 10", len(cols))
	}

	// The thumb is a contiguous block.
	inThumb := false
	seenGap := false
	for _, c := range cols {
		if c == "█" {
			if seenGap {
				t.Fatalf("non-contiguous thumb: %v", cols)
			}
			inThumb = true
		} else if inThumb {
			seenGap = true
		}
	}
	if !inThumb {
		t.Fatalf("scrollbar has no thumb: %v", cols)
	}

	// A later offset moves the thumb down.
	cols2 := previewScrollbar(80, 10, 100)
	moved := false
	for i := range cols {
		if cols[i] == "█" && cols2[i] != "█" {
			moved = true
		}
	}
	if !moved {
		t.Errorf("thumb did not move with offset: %v -> %v", cols, cols2)
	}
}

func TestPreviewRendersFromOffset(t *testing.T) {
	content := make([]byte, 0, 200)
	for i := 1; i <= 30; i++ {
		content = append(content, []byte(fmt.Sprintf("line %d", i))...)
		if i < 30 {
			content = append(content, '\n')
		}
	}
	m := newModel(BuildTree([]Item{{Path: "big.go", Content: content}}), Options{})
	m.height = 12
	m.width = 80

	m = updateKey(m, tea.KeyPgDown)
	view := m.renderPreviewBox(40, m.height-m.footerHeight())
	if !strings.Contains(view, "│ ") {
		t.Fatal("preview missing line numbers")
	}
	// The viewport must not start at line 1 anymore (offset > 0 renders a
	// later line, which won't be the first content line).
	first := m.previewOffset + 1
	if !strings.Contains(view, fmt.Sprintf("%3d │", first)) {
		t.Errorf("preview view missing line %d after scrolling", first)
	}
}

func updateKey(m model, kt tea.KeyType, runes ...rune) model {
	msg := tea.KeyMsg{Type: kt, Runes: runes}
	updated, _ := m.Update(msg)
	return updated.(model)
}
