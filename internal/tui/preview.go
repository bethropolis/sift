package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// syncPreview resets the preview scroll position whenever the selected node
// changes, so each file starts at its top.
func (m *model) syncPreview() {
	if n := m.node(); n != m.previewNode {
		m.previewNode = n
		m.previewOffset = 0
	}
}

// previewLines splits preview content into lines, dropping the phantom empty
// line left by a trailing newline so the scroll range matches visible text.
func previewLines(content []byte) []string {
	lines := strings.Split(string(content), "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// previewLineCount returns the number of preview lines for the selected node.
func (m model) previewLineCount() int {
	n := m.node()
	if n == nil || n.Kind != KindFile {
		return 0
	}
	return len(previewLines(n.Preview()))
}

// previewPageSize returns how many content lines fit in the preview pane.
func (m model) previewPageSize() int {
	bodyHeight := max(5, m.height-m.footerHeight())
	return max(1, bodyHeight-3)
}

// previewHalfPage returns the number of lines for a half-page scroll step.
func (m model) previewHalfPage() int {
	return max(1, m.previewPageSize()/2)
}

// scrollPreview scrolls the preview pane by delta lines, clamped to the
// available content. It is a no-op when the selected node is a directory.
func (m *model) scrollPreview(delta int) {
	n := m.node()
	if n == nil || n.Kind != KindFile {
		return
	}
	total := m.previewLineCount()
	maxOffset := max(0, total-m.previewPageSize())
	m.previewOffset += delta
	if m.previewOffset < 0 {
		m.previewOffset = 0
	}
	if m.previewOffset > maxOffset {
		m.previewOffset = maxOffset
	}
}

func (m model) renderPreviewBox(width, height int) string {
	boxStyle := lipgloss.NewStyle().
		Width(max(1, width-2)).
		Height(max(1, height-2)).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62")).
		Padding(0, 1)

	var b strings.Builder
	n := m.node()

	title := " Preview "
	if n != nil && n.Kind == KindFile {
		title = fmt.Sprintf(" %sPreview: %s ", m.glyphs.File, n.Path)
	} else if n != nil && n.Kind == KindDir {
		title = fmt.Sprintf(" %sFolder: %s/ ", m.glyphs.FolderOpen, n.Path)
	}
	b.WriteString(titleStyle.Render(title))

	if n == nil {
		b.WriteString("\n")
		b.WriteString(hintStyle.Render("No file selected"))
		return boxStyle.Render(b.String())
	}

	innerWidth := max(10, width-4)

	if n.Kind == KindDir {
		innerRows := max(1, height-3)
		lines := []string{
			fmt.Sprintf("%d files, %d tokens", n.FileCount(), n.TotalActiveTokens()),
			"",
			"Press [Space] to toggle directory selection.",
			"Press [Enter] or [l] to expand/collapse.",
		}
		for i := 0; i < len(lines) && i < innerRows; i++ {
			b.WriteString("\n")
			b.WriteString(hintStyle.Render(lines[i]))
		}
	} else {
		content := n.Preview()
		lines := previewLines(content)

		innerRows := max(1, height-3)
		if n.SecretCount > 0 {
			innerRows--
		}

		// Clamp the scroll position in case the pane was resized since the
		// last scroll.
		if maxOffset := max(0, len(lines)-innerRows); m.previewOffset > maxOffset {
			m.previewOffset = maxOffset
		}

		barCols := previewScrollbar(m.previewOffset, innerRows, len(lines))

		end := min(len(lines), m.previewOffset+innerRows)
		for i := m.previewOffset; i < end; i++ {
			lineNo := i + 1
			lineText := lines[i]

			prefix := fmt.Sprintf("%3d │ ", lineNo)
			prefixWidth := lipgloss.Width(prefix)

			barWidth := 0
			var bar string
			if barCols != nil {
				barWidth = 1
				bar = barCols[i-m.previewOffset]
			}

			maxLen := max(1, innerWidth-prefixWidth-barWidth)

			lineText = strings.ReplaceAll(lineText, "\t", "    ")
			if lipgloss.Width(lineText) > maxLen {
				lineText = truncateString(lineText, maxLen)
			}

			b.WriteString("\n")
			b.WriteString(dimStyle.Render(prefix))
			b.WriteString(lineText)
			if barWidth > 0 {
				b.WriteString(strings.Repeat(" ", max(0, maxLen-lipgloss.Width(lineText))))
				b.WriteString(scrollbarStyle.Render(bar))
			}
		}

		if n.SecretCount > 0 {
			b.WriteString("\n")
			b.WriteString(warningStyle.Render(fmt.Sprintf("%sWarning: %d secret(s) detected in this file",
				m.glyphs.Warning, n.SecretCount)))
		}
	}

	return boxStyle.Render(b.String())
}

// previewScrollbar returns a one-character-per-row scrollbar column for a
// viewport showing view rows of total lines starting at offset, or nil when
// the content fits without scrolling.
func previewScrollbar(offset, view, total int) []string {
	if total <= view {
		return nil
	}
	track := view
	thumb := max(1, track*view/total)
	maxOffset := total - view
	thumbTop := 0
	if maxOffset > 0 {
		thumbTop = offset * (track - thumb) / maxOffset
	}

	cols := make([]string, track)
	for i := 0; i < track; i++ {
		if i >= thumbTop && i < thumbTop+thumb {
			cols[i] = "█"
		} else {
			cols[i] = "░"
		}
	}
	return cols
}

// Preview returns the preview text for a file: its signature summary when the
// mode is signatures and one is available, otherwise the full content.
func (n *TreeNode) Preview() []byte {
	if n.Mode == ModeSignatures && len(n.SigContent) > 0 {
		return n.SigContent
	}
	return n.Content
}
