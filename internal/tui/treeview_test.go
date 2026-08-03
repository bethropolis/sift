package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// styleForegroundEscape returns the ANSI prefix a style wraps its content in,
// e.g. "\x1b[38;5;62m" for an 8-bit foreground, by rendering a marker rune.
func styleForegroundEscape(s lipgloss.Style) string {
	r := s.Render("X")
	if i := strings.Index(r, "X"); i >= 0 {
		return r[:i]
	}
	return r
}

func rowIndex(rows []*TreeNode, n *TreeNode) int {
	for i, r := range rows {
		if r == n {
			return i
		}
	}
	return 0
}

// TestHiddenAndGitIgnoredRowsRenderedMuted asserts that hidden and Git-ignored
// rows are displayed with the muted style (a visible color, never the
// near-invisible Faint), while ordinary visible rows are not muted.
func TestHiddenAndGitIgnoredRowsRenderedMuted(t *testing.T) {
	// lipgloss suppresses ANSI codes in non-TTY contexts; force a color
	// profile so the styles are observable.
	prev := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.ANSI256)
	defer lipgloss.SetColorProfile(prev)

	root := BuildTree([]Item{
		{Path: "a.go", TokensFull: 10},
		{Path: "readme.md", TokensFull: 3},
		{Path: ".env", Hidden: true, TokensFull: 5},
		{Path: "build.log", GitIgnored: true, TokensFull: 7},
	})
	m := newModel(root, Options{})
	m.showHidden = true
	m.showGitIgnored = true
	m.recomputeRows()

	env := root.findChild(".env")
	buildLog := root.findChild("build.log")

	// Park the cursor on a normal visible file so the hidden/gitignored rows
	// are rendered in their idle (non-hovered) state.
	m.cursor = rowIndex(m.rows, root.findChild("readme.md"))
	m.clampOffset()

	mutedEsc := styleForegroundEscape(m.styles.muted)
	if strings.Contains(mutedEsc, "\x1b[2m") {
		t.Errorf("muted style should not be Faint (too subtle): %q", mutedEsc)
	}

	for name, n := range map[string]*TreeNode{".env": env, "build.log": buildLog} {
		row := m.treeRow(n, 60)
		if !strings.Contains(row, mutedEsc) {
			t.Errorf("%s row not muted: missing %q in %q", name, mutedEsc, row)
		}
		if strings.Contains(row, "\x1b[2m") {
			t.Errorf("%s row should not carry the faint attribute", name)
		}
	}

	// Ordinary visible files must never be muted, even when another file is.
	for _, name := range []string{"a.go", "readme.md"} {
		n := root.findChild(name)
		if row := m.treeRow(n, 60); strings.Contains(row, mutedEsc) {
			t.Errorf("%s (visible) row should not be muted: %q", name, row)
		}
	}
}

// TestHiddenRowEmphasisOnCursorAndSelection asserts that hovering or selecting
// a hidden/Git-ignored row takes precedence over the muted styling, so the
// user's current focus remains obvious.
func TestHiddenRowEmphasisOnCursorAndSelection(t *testing.T) {
	prev := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.ANSI256)
	defer lipgloss.SetColorProfile(prev)

	root := BuildTree([]Item{
		{Path: "readme.md", TokensFull: 3},
		{Path: ".env", Hidden: true, TokensFull: 5},
	})
	m := newModel(root, Options{})
	m.showHidden = true
	m.recomputeRows()

	env := root.findChild(".env")
	mutedEsc := styleForegroundEscape(m.styles.muted)

	// Hovered: the cursor highlight wins over the muted color.
	m.cursor = rowIndex(m.rows, env)
	m.clampOffset()
	if row := m.treeRow(env, 60); strings.Contains(row, mutedEsc) {
		t.Errorf("hovered hidden row should use cursor emphasis, not muted: %q", row)
	}

	// Selected but not hovered: the selection highlight wins over muted.
	m.cursor = rowIndex(m.rows, root.findChild("readme.md"))
	m.clampOffset()
	env.SelectState = Selected
	if row := m.treeRow(env, 60); strings.Contains(row, mutedEsc) {
		t.Errorf("selected hidden row should use selection emphasis, not muted: %q", row)
	}
}
