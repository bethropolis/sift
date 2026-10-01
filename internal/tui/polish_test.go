package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestFormatTokenCount(t *testing.T) {
	tests := map[int]string{
		0: "0", 812: "812", 1_700: "1.7k", 12_900: "12.9k",
		1_250_000: "1.3M", 2_000_000_000: "2.0B",
	}
	for input, want := range tests {
		if got := formatTokenCount(input); got != want {
			t.Errorf("formatTokenCount(%d) = %q, want %q", input, got, want)
		}
	}
}

func TestHeaderShowsProjectAndSelectionWithinWidth(t *testing.T) {
	root := BuildTree([]Item{
		{Path: "docs/a.md", TokensFull: 1_200},
		{Path: "docs/b.md", TokensFull: 800},
	})
	root.setSelected(true)
	m := newModel(root, Options{ProjectPath: "/tmp/a-very-long-project-name/with/more/path"})
	for _, width := range []int{36, 80, 120} {
		header := m.renderHeader(width)
		if got := ansi.StringWidth(header); got != width {
			t.Errorf("header width at %d = %d", width, got)
		}
		plain := ansi.Strip(header)
		if !strings.Contains(plain, "sift") || !strings.Contains(plain, "2/2") {
			t.Errorf("header missing identity/state at %d: %q", width, plain)
		}
	}
}

func TestTreeCursorAndSelectionUseDistinctGrammar(t *testing.T) {
	root := BuildTree([]Item{{Path: "a.go"}, {Path: "b.go"}})
	m := newModel(root, Options{UseNerd: true})
	a := root.findChild("a.go")
	b := root.findChild("b.go")
	a.Toggle()
	m.cursor = rowIndex(m.rows, b)

	selected := ansi.Strip(m.treeRow(a, 60))
	cursor := ansi.Strip(m.treeRow(b, 60))
	if strings.Contains(selected, "▸") || !strings.Contains(selected, "●") {
		t.Errorf("selected row conflates cursor/selection: %q", selected)
	}
	if !strings.Contains(cursor, "▸") || strings.Contains(cursor, "●") {
		t.Errorf("cursor row missing cursor marker: %q", cursor)
	}
}

func TestFilePreviewShowsCompactMetadata(t *testing.T) {
	root := BuildTree([]Item{{
		Path: "README.md", Content: []byte("# Sift\n"), TokensFull: 1_200, SecretCount: 2,
	}})
	m := newModel(root, Options{})
	m.width, m.height = 80, 20
	view := ansi.Strip(m.renderPreviewBox(40, m.bodyHeight()))
	for _, want := range []string{"Markdown", "FULL", "1.2k tokens", "2", "# Sift"} {
		if !strings.Contains(view, want) {
			t.Errorf("file preview missing %q: %q", want, view)
		}
	}
}

func TestDirectoryInspectorUsesCachedAggregates(t *testing.T) {
	root := BuildTree([]Item{
		{Path: "docs/a.md", TokensFull: 1_000},
		{Path: "docs/b.md", TokensFull: 2_000},
	})
	root.setSelected(true)
	m := newModel(root, Options{})
	m.width, m.height = 80, 20
	view := ansi.Strip(m.renderPreviewBox(40, m.bodyHeight()))
	for _, want := range []string{"2 files", "2 selected", "Full context", "Signatures", "3.0k"} {
		if !strings.Contains(view, want) {
			t.Errorf("directory inspector missing %q: %q", want, view)
		}
	}
}

func TestEmptyPreviewHasIntentionalState(t *testing.T) {
	m := newModel(BuildTree(nil), Options{})
	m.rows = nil
	m.width, m.height = 80, 20
	view := ansi.Strip(m.renderPreviewBox(40, m.bodyHeight()))
	for _, want := range []string{"sift", "Select files to build context", "0 / 0 selected"} {
		if !strings.Contains(view, want) {
			t.Errorf("empty state missing %q: %q", want, view)
		}
	}
}

func TestContextualFooterStaysBounded(t *testing.T) {
	m := newModel(BuildTree([]Item{{Path: "scheduler.go"}}), Options{})
	m.width = 40
	m.filtering = true
	m.filter = "sched"
	lines := strings.Split(ansi.Strip(m.renderFooter(m.width)), "\n")
	if len(lines) != m.footerHeight() {
		t.Fatalf("footer lines = %d, want %d", len(lines), m.footerHeight())
	}
	for i, line := range lines {
		if got := ansi.StringWidth(line); got > m.width {
			t.Errorf("footer line %d width = %d: %q", i, got, line)
		}
	}
	if !strings.Contains(lines[1], "Apply") {
		t.Errorf("filter footer missing contextual action: %q", lines[1])
	}
}

func TestASCIIGlyphFallbackRemainsSemantic(t *testing.T) {
	g := NewASCIIGlyphs()
	if g.Cursor == "" || g.CheckFull == "" || g.CheckNone == "" || g.Relevant == "" {
		t.Fatalf("ASCII glyph vocabulary incomplete: %+v", g)
	}
	if g.Cursor == g.CheckFull {
		t.Error("ASCII cursor and selected glyphs must remain distinct")
	}
}
