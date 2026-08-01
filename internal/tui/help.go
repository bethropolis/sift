package tui

import (
	"strings"
)

// renderHelpModal overlays the keyboard-shortcut reference on the picker.
func (m model) renderHelpModal(view string, width, height int) string {
	modalWidth := min(width, 58)
	if modalWidth < 30 {
		modalWidth = 30
	}

	var b strings.Builder
	b.WriteString(titleStyle.Render(" Sift Interactive Picker - Keyboard Shortcuts \n\n"))

	b.WriteString(titleStyle.Render("Navigation & Tree:\n"))
	b.WriteString("  j / k, Up / Down     Move cursor\n")
	b.WriteString("  h / l, Left / Right  Collapse / Expand directory\n")
	b.WriteString("  Enter / Space        Expand folder or toggle file selection\n")
	b.WriteString("  E / C                Expand All / Collapse All\n\n")

	b.WriteString(titleStyle.Render("Selection & Context Density:\n"))
	b.WriteString("  m                    Cycle Mode (FULL -> SIGS -> SKIP)\n")
	b.WriteString("  s                    Smart Auto-Select by Git Rank\n")
	b.WriteString("  d                    Incremental Delta Dump Modal\n")
	b.WriteString("  a                    Select All / Deselect All\n\n")

	b.WriteString(titleStyle.Render("Generation & Actions:\n"))
	b.WriteString("  g                    Generate output document (stays in TUI)\n")
	b.WriteString("  y                    Copy selection to Clipboard\n")
	b.WriteString("  /                    Fuzzy path search\n")
	b.WriteString("  PgUp / PgDn, [ / ]   Scroll preview pane\n")
	b.WriteString("  q / Esc              Exit picker\n\n")

	b.WriteString(hintStyle.Render("Press [Esc], [?], or [q] to close help"))

	return m.overlay(view, boxStyle(modalWidth).Render(b.String()), width, height)
}
