package tui

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

func helpContentLines() []string {
	return []string{
		"Navigation & Tree:",
		"  j / k, Up / Down     Move cursor",
		"  h / l, Left / Right  Collapse / Expand directory",
		"  Enter / Space        Expand folder or toggle file selection",
		"  E / C                Expand All / Collapse All",
		"  .                    Toggle hidden files",
		"  H                    Toggle gitignored files",
		"",
		"Selection & Context Density:",
		"  m                    Cycle Mode (FULL -> SIGS -> SKIP)",
		"  s                    Smart Select (FULL/SIGS by git history)",
		"  d                    Incremental Delta Dump Modal",
		"  a                    Select All / Deselect All",
		"  t                    Color theme selector",
		"  p                    Prompt / task directive builder",
		"",
		"Generation & Actions:",
		"  g                    Generate output document (stays in TUI)",
		"  y                    Copy selection to Clipboard",
		"  /                    Fuzzy path search",
		"  Tab                  Toggle focus between Tree and Preview pane",
		"  PgUp / PgDn, [ / ]   Scroll preview pane",
		"  J / K                Scroll preview down / up 1 line",
		"  q / Esc              Exit picker",
	}
}

// renderHelpModal overlays the keyboard-shortcut reference on the picker.
func (m *model) renderHelpModal(view string, width, height int) string {
	modalWidth := min(width, 58)
	if modalWidth < 30 {
		modalWidth = 30
	}

	lines := helpContentLines()
	modalHeight := min(max(8, height-2), len(lines)+5)
	bodyRows := max(1, modalHeight-5)
	maxOffset := max(0, len(lines)-bodyRows)
	if m.helpOffset > maxOffset {
		m.helpOffset = maxOffset
	}
	if m.helpOffset < 0 {
		m.helpOffset = 0
	}
	var b strings.Builder
	title := ansi.Truncate(" Sift Interactive Picker - Keyboard Shortcuts ", modalWidth-4, "…")
	b.WriteString(m.styles.title.Render(title))
	for _, line := range lines[m.helpOffset:min(len(lines), m.helpOffset+bodyRows)] {
		b.WriteString("\n")
		if strings.HasSuffix(line, ":") {
			b.WriteString(m.styles.title.Render(line))
		} else {
			b.WriteString(ansi.Truncate(line, modalWidth-4, "…"))
		}
	}
	b.WriteString("\n")
	b.WriteString(m.styles.hint.Render("↑/↓ scroll · Esc/?/q close"))
	return m.overlay(view, m.boxStyle(modalWidth).Height(modalHeight-2).Render(b.String()), width, height)
}

func (m *model) scrollHelp(delta int) {
	m.helpOffset += delta
	if m.helpOffset < 0 {
		m.helpOffset = 0
	}
	maxOffset := max(0, len(helpContentLines())-m.helpPageSize())
	if m.helpOffset > maxOffset {
		m.helpOffset = maxOffset
	}
}
