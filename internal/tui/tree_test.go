package tui

import (
	"reflect"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
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

// TestMixedModeFolderTokens guards against the recompute override that used
// to overwrite a selected folder's child-summed ActiveTokens with the uniform
// TokensFull/TokensSig, which inflated the total when children had mixed modes.
func TestMixedModeFolderTokens(t *testing.T) {
	root := BuildTree([]Item{
		{Path: "internal/a.go", TokensFull: 20, TokensSig: 8},
		{Path: "internal/b.go", TokensFull: 30, TokensSig: 12},
	})
	internal := root.findChild("internal")
	internal.setSelected(true)

	// Switch only one file to signatures so the folder holds mixed modes.
	a := internal.findChild("a.go")
	a.applyMode(ModeSignatures)
	root.recompute()

	if got := internal.TotalActiveTokens(); got != 38 {
		t.Errorf("TotalActiveTokens = %d, want 38 (8 sig + 30 full)", got)
	}
	if got := root.TotalActiveTokens(); got != 38 {
		t.Errorf("root TotalActiveTokens = %d, want 38", got)
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

func TestBuildTreeAppliesPreferredMode(t *testing.T) {
	root := BuildTree([]Item{
		{Path: "a.go", PreferredMode: ModeSignatures, TokensFull: 10},
		{Path: "b.go", TokensFull: 20},
	})
	if got := root.findChild("a.go").Mode; got != ModeSignatures {
		t.Errorf("a.go Mode = %q, want signatures", got)
	}
	if got := root.findChild("b.go").Mode; got != ModeFull {
		t.Errorf("b.go Mode = %q, want full", got)
	}
}

func TestSmartSelectModeAssignment(t *testing.T) {
	root := BuildTree([]Item{
		{Path: "pref.go", TokensFull: 100, TokensSig: 5, RankScore: 1.0, PreferredMode: ModeSignatures},
		{Path: "sig.go", TokensFull: 50, TokensSig: 5, RankScore: 0.5, SigContent: []byte("sig")},
		{Path: "plain.go", TokensFull: 30, RankScore: 0.1},
	})
	root.SelectByRank(1000)
	got := map[string]CompressMode{}
	for _, s := range root.Selections() {
		got[s.Path] = s.Mode
	}
	if got["pref.go"] != ModeSignatures {
		t.Errorf("pref.go mode = %q, want signatures", got["pref.go"])
	}
	if got["sig.go"] != ModeSignatures {
		t.Errorf("sig.go mode = %q, want signatures (sig-content fallback)", got["sig.go"])
	}
	if got["plain.go"] != ModeFull {
		t.Errorf("plain.go mode = %q, want full", got["plain.go"])
	}
}

func TestSmartSelectBudgetUsesSigTokens(t *testing.T) {
	root := BuildTree([]Item{
		{Path: "big.go", TokensFull: 100, TokensSig: 5, RankScore: 1.0, PreferredMode: ModeSignatures},
		{Path: "small.go", TokensFull: 30, RankScore: 0.5},
	})
	if count := root.SelectByRank(35); count != 2 {
		t.Fatalf("SelectByRank = %d, want 2 (big.go charged at sig tokens)", count)
	}
	if got := len(root.Selections()); got != 2 {
		t.Fatalf("selected %d files, want 2", got)
	}
}
