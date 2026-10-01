package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func (m *model) clampOffset() {
	innerRows := m.treeViewportRows()

	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+innerRows {
		m.offset = m.cursor - innerRows + 1
	}
	m.clampTreeOffset()
}

func (m model) renderTreeBox(width, height int) string {
	borderColor := m.styles.border
	if m.focus == FocusTree {
		borderColor = m.styles.accent // Active border follows the theme accent
	}
	boxStyle := lipgloss.NewStyle().
		Width(max(1, width-2)).
		// Width is the content width; the border adds two columns. The max
		// bounds therefore apply to the complete rendered card width.
		MaxWidth(max(1, width)).
		Height(max(1, height-2)).
		MaxHeight(max(1, height)).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor)

	var b strings.Builder

	titleStyle := m.styles.hint
	if m.focus == FocusTree {
		titleStyle = m.styles.title
	}
	fileCount := m.root.FileCount()
	selCount := m.root.SelectedCount()
	titleText := fmt.Sprintf(" Explorer  %d files", fileCount)
	if selCount > 0 {
		titleText += fmt.Sprintf("  %d selected", selCount)
	}
	titleText += " "
	titleText = ansi.Truncate(titleText, max(1, width-4), "…")
	b.WriteString(titleStyle.Render(titleText))

	if m.filtering && m.filter != "" {
		filterLine := ansi.Truncate(fmt.Sprintf(" / %s", m.filter), max(1, width-4), "…")
		b.WriteString("\n")
		b.WriteString(m.styles.notice.Render(filterLine))
	}

	innerRows := max(1, height-3)
	if m.filtering && m.filter != "" {
		innerRows = max(1, height-4)
	}
	end := min(len(m.rows), m.offset+innerRows)
	innerWidth := max(10, width-4)

	for i := m.offset; i < end; i++ {
		b.WriteString("\n")
		b.WriteString(m.treeRow(m.rows[i], innerWidth))
	}

	return boxStyle.Render(b.String())
}

func (m model) treeRow(n *TreeNode, width int) string {
	prefix := n.TreePrefix(m.glyphs)

	var mark string
	switch n.SelectState {
	case Selected:
		mark = m.glyphs.CheckFull
	case Partial:
		mark = m.glyphs.CheckPartial
	default:
		mark = m.glyphs.CheckNone
	}

	cursorMark := "  "
	if n == m.node() {
		cursorMark = m.glyphs.Cursor
	}

	icon := m.glyphs.File
	name := n.Name
	relevant := ""
	tokens := n.TokensSig
	if n.Kind == KindDir {
		icon = m.glyphs.FolderClosed
		if n.Expanded {
			icon = m.glyphs.FolderOpen
		}
		name += "/"
		tokens = n.TotalActiveTokens()
	} else if n.Mode == ModeFull {
		tokens = n.TokensFull
	}
	if n.Kind == KindFile && n.RankScore >= 0.55 {
		relevant = " " + lipgloss.NewStyle().Foreground(m.styles.accentSoft).Render(m.glyphs.Relevant)
	}

	secret := ""
	if n.SecretCount > 0 {
		secret = " " + m.styles.warning.Render(fmt.Sprintf("%s%d", strings.TrimSpace(m.glyphs.Warning), n.SecretCount))
	}

	modeStr := ""
	if n.Kind == KindFile {
		switch n.Mode {
		case ModeFull:
			modeStr = " " + m.styles.modeFull.Render(m.glyphs.ModeFull)
		case ModeSignatures:
			modeStr = " " + m.styles.modeSig.Render(m.glyphs.ModeSigns)
		case ModeSkip:
			modeStr = " " + m.styles.modeSkip.Render(m.glyphs.ModeSkip)
		}
	}

	treeGuide := m.styles.treeGuide.Render(prefix)
	left := fmt.Sprintf("%s%s %s %s%s%s%s%s", cursorMark, treeGuide, mark, icon, name, relevant, modeStr, secret)

	// Style the token count based on budget usage and zero-state.
	var right string
	if n.Kind == KindFile && n.TokensFull == 0 && n.ApproxTokens > 0 {
		right = m.styles.muted.Render(fmt.Sprintf("%6s", "~"+formatTokenCount(n.ApproxTokens)))
	} else if tokens == 0 {
		right = m.styles.dim.Render(fmt.Sprintf("%6s", formatTokenCount(0)))
	} else {
		right = lipgloss.NewStyle().Foreground(m.tokenColor(tokens)).Render(fmt.Sprintf("%6s", formatTokenCount(tokens)))
	}

	leftWidth := ansi.StringWidth(left)
	rightWidth := ansi.StringWidth(right)
	compose := func(name string) string {
		return fmt.Sprintf("%s%s %s %s%s%s%s%s", cursorMark, treeGuide, mark, icon, name, relevant, modeStr, secret)
	}

	if leftWidth+rightWidth > width {
		fixed := ansi.StringWidth(cursorMark) + ansi.StringWidth(treeGuide) + ansi.StringWidth(mark) +
			ansi.StringWidth(icon) + ansi.StringWidth(relevant) + ansi.StringWidth(modeStr) +
			ansi.StringWidth(secret) + 2
		nameSpace := width - rightWidth - fixed
		if nameSpace >= 2 && ansi.StringWidth(name) > nameSpace {
			name = ansi.Truncate(name, nameSpace, "…")
			left = compose(name)
			leftWidth = ansi.StringWidth(left)
		}
	}

	pad := max(0, width-leftWidth-rightWidth)
	row := left + strings.Repeat(" ", pad) + right
	if ansi.StringWidth(row) > width {
		row = ansi.Truncate(row, width, "…")
	}

	// Cursor and selection intentionally use different grammar: the cursor owns
	// a narrow marker plus surface, while selection owns the state glyph/color.
	if n == m.node() {
		return m.styles.cursor.Render(row)
	}
	if n.SelectState != Unselected {
		return m.styles.selected.Render(row)
	}

	// Hidden and Git-ignored rows recede without losing their state glyphs.
	if n.Hidden || n.GitIgnored {
		name = m.styles.muted.Render(name)
		right = m.styles.muted.Render(fmt.Sprintf("%6s", formatTokenCount(tokens)))
		left = compose(name)
		leftWidth = ansi.StringWidth(left)
		pad = max(0, width-leftWidth-ansi.StringWidth(right))
		row = ansi.Truncate(left+strings.Repeat(" ", pad)+right, width, "…")
	}
	return row
}

// tokenColor returns a theme-aware color for a token count relative to the
// budget. When no budget is set every non-zero count uses the dim (faint) style
// so it doesn't interfere with the muted style that marks hidden/git-ignored rows.
func (m model) tokenColor(tokens int) lipgloss.TerminalColor {
	if m.budget <= 0 || tokens <= 0 {
		return m.styles.dim.GetForeground()
	}
	pct := tokens * 100 / m.budget
	switch {
	case pct >= 30:
		return m.styles.modeSkip.GetForeground()
	case pct >= 10:
		return m.styles.modeSig.GetForeground()
	default:
		return m.styles.dim.GetForeground()
	}
}
