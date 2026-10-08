package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func truncateString(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	return ansi.Truncate(s, maxLen, "…")
}

// truncateTail preserves the informative end of a path or label.
func truncateTail(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	if ansi.StringWidth(s) <= maxLen {
		return s
	}
	if maxLen == 1 {
		return "…"
	}
	runes := []rune(s)
	keep := maxLen - 1
	if keep > len(runes) {
		keep = len(runes)
	}
	return "…" + string(runes[len(runes)-keep:])
}

// compactTokenCount keeps exact values readable and scales large values to a
// compact developer-friendly form. Callers decide whether to add "~" for an
// estimate or a unit such as "tokens".
func compactTokenCount(value int) string {
	scale := 1
	suffix := ""
	switch {
	case value >= 1_000_000_000:
		scale, suffix = 1_000_000_000, "B"
	case value >= 1_000_000:
		scale, suffix = 1_000_000, "M"
	case value >= 1_000:
		scale, suffix = 1_000, "k"
	}
	if suffix == "" {
		return strconv.Itoa(value)
	}
	// Use integer half-up rounding instead of fmt's ties-to-even behavior,
	// which would display 1.25M as 1.2M.
	tenths := (value*10 + scale/2) / scale
	return fmt.Sprintf("%d.%d%s", tenths/10, tenths%10, suffix)
}

func formatTokenCount(tokens int) string {
	if tokens < 0 {
		return "0"
	}
	return compactTokenCount(tokens)
}

// displayProjectPath shortens the user's home directory to "~" while keeping
// repository identity readable in the application header.
func displayProjectPath(path string) string {
	if strings.TrimSpace(path) == "" {
		return ""
	}
	clean := filepath.Clean(path)
	if !filepath.IsAbs(clean) {
		if abs, err := filepath.Abs(clean); err == nil {
			clean = abs
		}
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		if rel, relErr := filepath.Rel(home, clean); relErr == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			if rel == "." {
				return "~"
			}
			return filepath.ToSlash(filepath.Join("~", rel))
		}
	}
	return filepath.ToSlash(clean)
}

// boxStyle returns a rounded bordered box of the given width using the
// model's theme border color.
func (m model) boxStyle(width int) lipgloss.Style {
	return lipgloss.NewStyle().
		Width(max(1, width-2)).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.styles.border)
}

// modalStyle is the shared presentation language for every overlay. Individual
// modals own content and sizing, while border treatment stays consistent.
func (m model) modalStyle(width int) lipgloss.Style {
	return m.boxStyle(width)
}

func (m model) modalTitle(icon, label string, width int) string {
	title := fmt.Sprintf(" %s%s ", icon, label)
	return m.styles.title.Render(ansi.Truncate(title, max(1, width-4), "…"))
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
