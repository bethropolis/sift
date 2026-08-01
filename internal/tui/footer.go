package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// footerHeight returns the number of lines the footer occupies. The join
// newline between body and footer is included, so the whole view is exactly
// height lines tall and the footer sits on the bottom row.
func (m model) footerHeight() int {
	h := footerLines
	if m.notice != "" {
		h++
	}
	return h
}

func (m model) renderFooter(width int) string {
	selected := m.root.SelectedCount()
	active := m.root.TotalActiveTokens()

	budget := ""
	if m.budget > 0 {
		pct := 0
		if active >= m.budget {
			pct = 100
		} else if active > 0 {
			pct = active * 100 / m.budget
		}
		barLen := max(1, width-60)
		filled := barLen * pct / 100
		bar := strings.Repeat("█", filled) + strings.Repeat("░", max(0, barLen-filled))

		var barColor lipgloss.Color
		switch {
		case active > m.budget:
			barColor = lipgloss.Color("9") // Red
		case pct >= 80:
			barColor = lipgloss.Color("11") // Yellow
		default:
			barColor = lipgloss.Color("10") // Green
		}
		styledBar := lipgloss.NewStyle().Foreground(barColor).Render(bar)
		budget = fmt.Sprintf("Budget: %d / %d %s %d%% | ", active, m.budget, styledBar, pct)
	}

	actions := "[?] Help  [g] Generate  [y] Copy  [q] Exit"
	if m.filtering {
		actions = fmt.Sprintf("/ Filter (%d matches): %s▌", len(m.rows), m.filter)
	}

	var b strings.Builder
	b.WriteString(strings.Repeat("─", width))
	if m.notice != "" {
		b.WriteString("\n")
		b.WriteString(noticeStyle.Render(m.notice))
	}
	b.WriteString("\n")
	b.WriteString(hintStyle.Render(fmt.Sprintf("%sStyle: %s | %d selected (%d tok) | %s",
		budget, m.style, selected, active, actions)))
	return b.String()
}
