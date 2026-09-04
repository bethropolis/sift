package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// scanSpinner returns one frame of the braille-dot scanning animation.
var scanSpinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

func (m model) spinnerChar() string {
	return scanSpinnerFrames[m.spinnerFrame%len(scanSpinnerFrames)]
}

// footerHeight returns the number of rendered footer lines. The body is
// normalized before the separator is added, so the separator itself does not
// consume a layout row.
func (m model) footerHeight() int {
	return 2
}

// keyBadge renders a single key hint as  key  label  using dim styling for
// the label and a slightly brighter style for the key itself.
func (m model) keyBadge(key, label string) string {
	k := lipgloss.NewStyle().Foreground(m.styles.accent).Bold(true).Render(key)
	l := m.styles.muted.Render(label)
	return k + " " + l
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
		barLen := 16
		filled := barLen * pct / 100
		bar := strings.Repeat("█", filled) + strings.Repeat("░", max(0, barLen-filled))

		var barStyle lipgloss.Style
		switch {
		case active > m.budget:
			barStyle = m.styles.modeSkip
		case pct >= 80:
			barStyle = m.styles.modeSig
		default:
			barStyle = m.styles.modeFull
		}
		styledBar := barStyle.Render(bar)
		budgetLabel := m.styles.muted.Render(fmt.Sprintf("%d/%d", active, m.budget))
		budget = budgetLabel + " " + styledBar + "  "
	}

	sep := m.styles.dim.Render(" · ")

	var statusLine string
	visibility := ""
	if m.showHidden {
		visibility += " +dot"
	}
	if m.showGitIgnored {
		visibility += " +git"
	}

	switch {
	case m.notice != "":
		statusLine = budget + m.styles.notice.Render("  "+m.notice)
	case m.filtering:
		matchStr := m.styles.hint.Render(fmt.Sprintf("%d match", len(m.rows)))
		if len(m.rows) != 1 {
			matchStr = m.styles.hint.Render(fmt.Sprintf("%d matches", len(m.rows)))
		}
		statusLine = m.styles.title.Render("/") + " " +
			m.styles.hint.Render(m.filter) +
			m.styles.title.Render("▌") + "  " +
			matchStr
	case m.stream.active() && !m.scanDone:
		spinner := lipgloss.NewStyle().Foreground(m.styles.accent).Bold(true).Render(m.spinnerChar())
		scanInfo := m.styles.muted.Render(fmt.Sprintf("%d files · %d dirs", m.scanFiles, m.scanDirs))
		hints := strings.Join([]string{
			m.keyBadge("p", "Prompt"),
			m.keyBadge("m", "Mode"),
			m.keyBadge("g", "Gen"),
			m.keyBadge("y", "Copy"),
			m.keyBadge("q", "Quit"),
			m.keyBadge("?", "Help"),
		}, sep)
		statusLine = budget + spinner + " " + scanInfo + "  " + hints
	default:
		selStr := m.styles.hint.Render(fmt.Sprintf("%d sel", selected))
		tokStr := m.styles.hint.Render(fmt.Sprintf("%d tok", active))
		styleStr := m.styles.muted.Render("Style: " + m.style + visibility)

		var hintList []string
		if width < 80 {
			hintList = []string{
				m.keyBadge("m", "Mode"),
				m.keyBadge("y", "Copy"),
				m.keyBadge("?", "Help"),
				m.keyBadge("q", "Quit"),
			}
		} else {
			hintList = []string{
				m.keyBadge("p", "Prompt"),
				m.keyBadge("m", "Mode"),
				m.keyBadge("g", "Gen"),
				m.keyBadge("y", "Copy"),
				m.keyBadge("?", "Help"),
				m.keyBadge("q", "Quit"),
			}
		}
		hints := strings.Join(hintList, sep)
		statusLine = budget + selStr + sep + tokStr + sep + styleStr + "  " + hints
	}

	var b strings.Builder
	b.WriteString(m.styles.dim.Render(strings.Repeat("─", width)))
	b.WriteString("\n")
	statusLine = ansi.Truncate(statusLine, max(1, width), "…")
	b.WriteString(statusLine)
	return b.String()
}
