package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m *model) clampOffset() {
	bodyHeight := max(5, m.height-m.footerHeight())
	innerRows := max(1, bodyHeight-3)

	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+innerRows {
		m.offset = m.cursor - innerRows + 1
	}
}

func (m model) renderTreeBox(width, height int) string {
	borderColor := lipgloss.Color("62")
	if m.focus == FocusTree {
		borderColor = lipgloss.Color("12") // Active bright blue border
	}
	boxStyle := lipgloss.NewStyle().
		Width(max(1, width-2)).
		Height(max(1, height-2)).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor)

	var b strings.Builder
	title := fmt.Sprintf(" Explorer (%d files, %d tok) ",
		m.root.FileCount(), m.root.TotalActiveTokens())
	if m.glyphs.FolderOpen != "" {
		title = fmt.Sprintf(" %sExplorer (%d files, %d tok) ",
			m.glyphs.FolderOpen, m.root.FileCount(), m.root.TotalActiveTokens())
	}
	b.WriteString(titleStyle.Render(title))

	innerRows := max(1, height-3)
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

	icon := m.glyphs.File
	name := n.Name
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

	secret := ""
	if n.SecretCount > 0 {
		secret = " " + warningStyle.Render(m.glyphs.Warning)
	}

	modeStr := ""
	if n.Kind == KindFile {
		switch n.Mode {
		case ModeFull:
			modeStr = " " + modeFullStyle.Render(m.glyphs.ModeFull)
		case ModeSignatures:
			modeStr = " " + modeSigStyle.Render(m.glyphs.ModeSigns)
		case ModeSkip:
			modeStr = " " + modeSkipStyle.Render(m.glyphs.ModeSkip)
		}
	}

	treeGuide := treeGuideStyle.Render(prefix)
	left := fmt.Sprintf("%s %s %s%s%s%s", treeGuide, mark, icon, name, modeStr, secret)
	right := fmt.Sprintf("%6d tok", tokens)

	leftWidth := lipgloss.Width(left)
	rightWidth := lipgloss.Width(right)

	if leftWidth+rightWidth > width {
		fixed := lipgloss.Width(treeGuide) + lipgloss.Width(mark) + lipgloss.Width(icon) +
			lipgloss.Width(modeStr) + lipgloss.Width(secret) + 2
		nameSpace := width - rightWidth - fixed
		if nameSpace >= 2 && len(name) > nameSpace {
			name = truncateString(name, nameSpace)
			left = fmt.Sprintf("%s %s %s%s%s%s", treeGuide, mark, icon, name, modeStr, secret)
			leftWidth = lipgloss.Width(left)
		}
	}

	pad := max(0, width-leftWidth-rightWidth)
	row := left + strings.Repeat(" ", pad) + right

	if n == m.node() {
		return cursorStyle.Render(row)
	}
	if n.SelectState != Unselected {
		return selectedStyle.Render(row)
	}
	return row
}
