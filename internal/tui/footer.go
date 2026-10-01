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

func (m model) budgetMeter(active int) string {
	if m.budget <= 0 || m.width < 100 {
		return ""
	}
	pct := 100
	if active < m.budget {
		pct = active * 100 / m.budget
	}
	barLen := 10
	filled := barLen * pct / 100
	if filled > barLen {
		filled = barLen
	}
	bar := strings.Repeat("━", filled) + strings.Repeat("─", barLen-filled)
	style := m.styles.success
	switch {
	case active > m.budget:
		style = m.styles.danger
	case pct >= 80:
		style = m.styles.modeSig
	}
	return fmt.Sprintf("  %s %s %d%%", style.Render(bar), m.styles.muted.Render(fmt.Sprintf("%s/%s", formatTokenCount(active), formatTokenCount(m.budget))), pct)
}

func (m model) contextualActions(width int) string {
	sep := m.styles.subtle.Render(" · ")
	var hints []string
	switch {
	case m.filtering:
		hints = []string{m.keyBadge("↵", "Apply"), m.keyBadge("Esc", "Cancel")}
	case m.helpOpen:
		hints = []string{m.keyBadge("↑↓", "Scroll"), m.keyBadge("Esc", "Close")}
	case m.themeOpen:
		hints = []string{m.keyBadge("↵", "Apply"), m.keyBadge("Esc", "Close")}
	case m.promptOpen:
		hints = []string{m.keyBadge("↵", "Apply"), m.keyBadge("c", "Custom"), m.keyBadge("Esc", "Close")}
	case m.deltaOpen:
		hints = []string{m.keyBadge("Space", "Toggle"), m.keyBadge("m", "Mode"), m.keyBadge("↵", "Dump"), m.keyBadge("Esc", "Cancel")}
	case m.focus == FocusPreview:
		hints = []string{m.keyBadge("↑↓", "Scroll"), m.keyBadge("Tab", "Explorer"), m.keyBadge("y", "Copy"), m.keyBadge("?", "Help"), m.keyBadge("q", "Quit")}
	default:
		hints = []string{m.keyBadge("Space", "Select"), m.keyBadge("/", "Filter"), m.keyBadge("s", "Smart"), m.keyBadge("g", "Generate"), m.keyBadge("y", "Copy"), m.keyBadge("?", "Help")}
	}
	if width < 60 {
		if m.filtering || m.helpOpen || m.themeOpen || m.promptOpen || m.deltaOpen {
			return strings.Join(hints[:min(1, len(hints))], sep)
		}
		return lipgloss.NewStyle().Foreground(m.styles.accent).Bold(true).Render("?")
	}
	if width < 90 {
		if len(hints) > 4 {
			hints = hints[:4]
		}
	}
	return strings.Join(hints, sep)
}

func footerLine(left, right string, width int) string {
	if right == "" {
		return ansi.Truncate(left, width, "…")
	}
	gap := width - ansi.StringWidth(left) - ansi.StringWidth(right)
	if gap >= 2 {
		return left + strings.Repeat(" ", gap) + right
	}
	rightWidth := max(8, width/3)
	right = ansi.Truncate(right, rightWidth, "…")
	leftWidth := max(1, width-ansi.StringWidth(right)-1)
	left = ansi.Truncate(left, leftWidth, "…")
	return left + " " + right
}

func (m model) renderFooter(width int) string {
	selected := m.root.SelectedCount()
	active := m.root.TotalActiveTokens()
	sep := m.styles.subtle.Render(" · ")

	var left string
	switch {
	case m.notice != "":
		notice := m.notice
		lower := strings.ToLower(notice)
		style := m.styles.hint
		icon := m.glyphs.Warning
		switch {
		case strings.Contains(lower, "failed"), strings.Contains(lower, "error"), strings.Contains(lower, "exceeded"):
			style = m.styles.warning
		case strings.Contains(lower, "copied"), strings.Contains(lower, "generated"):
			style = m.styles.notice
			icon = m.glyphs.Success
		case strings.Contains(lower, "rescan"):
			icon = m.glyphs.Refresh
		}
		left = style.Render(strings.TrimSpace(icon) + " " + notice)
	case m.filtering:
		match := fmt.Sprintf("%d matches", len(m.rows))
		if len(m.rows) == 1 {
			match = "1 match"
		}
		left = m.styles.title.Render("/") + " " + m.styles.hint.Render(m.filter) + m.styles.title.Render("▌") + sep + m.styles.subtle.Render(match)
	case m.stream.active() && !m.scanDone:
		left = lipgloss.NewStyle().Foreground(m.styles.accent).Bold(true).Render(m.spinnerChar()) +
			" " + m.styles.hint.Render(fmt.Sprintf("Scanning · %d files · %d dirs", m.scanFiles, m.scanDirs))
	default:
		visibility := ""
		if m.showHidden {
			visibility += " +dot"
		}
		if m.showGitIgnored {
			visibility += " +git"
		}
		left = m.styles.hint.Render(fmt.Sprintf("%d selected · %s tokens", selected, formatTokenCount(active))) +
			sep + m.styles.subtle.Render("Style: "+m.style+visibility) + m.budgetMeter(active)
	}

	separator := m.styles.subtle.Render(strings.Repeat("─", width))
	return separator + "\n" + footerLine(left, m.contextualActions(width), width)
}
