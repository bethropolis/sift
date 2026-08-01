package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// footerHeight returns the number of lines the footer occupies. The join
// newline between body and footer is included, so the whole view is exactly
// height lines tall and the footer sits on the bottom row. It is always a
// fixed 2 lines (divider + status) so the panes never resize mid-session.
func (m model) footerHeight() int {
	return 2
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

	var statusLine string
	switch {
	case m.notice != "":
		statusLine = noticeStyle.Render(m.notice)
	case m.filtering:
		statusLine = fmt.Sprintf("/ Filter (%d matches): %s▌", len(m.rows), m.filter)
	default:
		actions := "[?] Help  [g] Generate  [y] Copy  [q] Exit"
		statusLine = fmt.Sprintf("%sStyle: %s | %d selected (%d tok) | %s", budget, m.style, selected, active, actions)
	}

	var b strings.Builder
	b.WriteString(strings.Repeat("─", width))
	b.WriteString("\n")
	b.WriteString(hintStyle.Render(statusLine))
	return b.String()
}
