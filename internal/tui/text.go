package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const headerLines = 2
const footerLines = 2

func truncateString(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	if maxLen == 1 {
		return "…"
	}
	return string(runes[:maxLen-1]) + "…"
}

// boxStyle returns a rounded bordered box of the given width.
func boxStyle(width int) lipgloss.Style {
	return lipgloss.NewStyle().
		Width(max(1, width-2)).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62"))
}

// overlay centers sub over view, blanking the area behind it.
func (m model) overlay(view, sub string, width, height int) string {
	subHeight := lipgloss.Height(sub)
	subWidth := lipgloss.Width(sub)
	top := max(0, (height-subHeight)/2)
	if top > height-1 {
		top = height - 1
	}
	left := max(0, (width-subWidth)/2)
	right := max(0, width-subWidth-left)

	viewLines := strings.Split(view, "\n")
	subLines := strings.Split(sub, "\n")

	for i := 0; i < len(subLines) && top+i < len(viewLines); i++ {
		viewLines[top+i] = strings.Repeat(" ", left) + subLines[i] + strings.Repeat(" ", right)
	}
	return strings.Join(viewLines, "\n")
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
