package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func helpContentLines() []string {
	return helpLines(helpSections(), 56)
}

// helpSection groups related shortcuts.
type helpSection struct {
	Header string
	Rows   []helpRow
}

type helpRow struct {
	Keys string
	Desc string
}

func helpSections() []helpSection {
	return []helpSection{
		{
			Header: "Navigation",
			Rows: []helpRow{
				{"j / k · ↑ / ↓", "Move cursor"},
				{"h · ←", "Collapse dir / go to parent"},
				{"l · →", "Expand directory"},
				{"Enter", "Toggle dir expand / file selection"},
				{"Space", "Toggle selection"},
				{"E / C", "Expand All / Collapse All"},
			},
		},
		{
			Header: "Visibility",
			Rows: []helpRow{
				{".", "Toggle hidden (dot) files"},
				{"H", "Toggle git-ignored files"},
				{"/", "Fuzzy path search"},
			},
		},
		{
			Header: "Selection & Mode",
			Rows: []helpRow{
				{"m", "Cycle Mode  FULL → SIGS → SKIP"},
				{"a", "Select All / Deselect All"},
				{"s", "Smart Select by git history"},
				{"f / F", "Follow imports from file (dependents / dependencies)"},
			},
		},
		{
			Header: "Output & Actions",
			Rows: []helpRow{
				{"g", "Generate output (stays in TUI)"},
				{"Y", "Generate and copy codebase.md"},
				{"p", "Prompt / task directive"},
				{"d", "Incremental delta dump"},
				{"t", "Color theme picker"},
			},
		},
		{
			Header: "Preview",
			Rows: []helpRow{
				{"Tab", "Toggle focus Tree ↔ Preview"},
				{"PgUp / PgDn · [ / ]", "Scroll preview page"},
				{"J / K", "Scroll preview 1 line"},
				{"Ctrl-U / Ctrl-D", "Scroll half page"},
			},
		},
		{
			Header: "General",
			Rows: []helpRow{
				{"q / Esc", "Exit picker"},
				{"?", "Toggle this help"},
				{"r", "Rescan files and git history"},
			},
		},
	}
}

func helpLines(sections []helpSection, contentWidth int) []string {
	var out []string
	for si, sec := range sections {
		if si > 0 {
			out = append(out, "")
		}
		out = append(out, sec.Header+":")
		keyW := 22
		for _, r := range sec.Rows {
			key := r.Keys
			if len(key) > keyW {
				keyW = len(key)
			}
		}
		if keyW > contentWidth/2 {
			keyW = contentWidth / 2
		}
		for _, r := range sec.Rows {
			pad := keyW - len(r.Keys)
			if pad < 1 {
				pad = 1
			}
			out = append(out, "  "+r.Keys+strings.Repeat(" ", pad)+" "+r.Desc)
		}
	}
	return out
}

// helpDisplayLines styles the structural hierarchy: section names and keys are
// anchors, while descriptions recede. helpLines remains the plain-text source
// used by scroll sizing and tests.
func (m model) helpDisplayLines(sections []helpSection, contentWidth int) []string {
	var out []string
	for si, sec := range sections {
		if si > 0 {
			out = append(out, "")
		}
		out = append(out, m.styles.title.Render(sec.Header))
		keyWidth := 0
		for _, row := range sec.Rows {
			keyWidth = max(keyWidth, ansi.StringWidth(row.Keys))
		}
		keyWidth = min(keyWidth, contentWidth/2)
		for _, row := range sec.Rows {
			key := ansi.Truncate(row.Keys, keyWidth, "…")
			pad := strings.Repeat(" ", max(1, keyWidth-ansi.StringWidth(key)+2))
			description := ansi.Truncate(row.Desc, max(1, contentWidth-keyWidth-4), "…")
			out = append(out, "  "+lipgloss.NewStyle().Foreground(m.styles.accentSoft).Render(key)+pad+m.styles.muted.Render(description))
		}
	}
	return out
}

func (m *model) renderHelpModal(view string, width, height int) string {
	modalWidth := min(width, 62)
	if modalWidth < 30 {
		modalWidth = 30
	}

	sections := helpSections()
	lines := m.helpDisplayLines(sections, modalWidth-6)

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
	b.WriteString(m.modalTitle(m.glyphs.Help, "Keyboard Shortcuts", modalWidth))
	for _, line := range lines[m.helpOffset:min(len(lines), m.helpOffset+bodyRows)] {
		b.WriteString("\n")
		b.WriteString(ansi.Truncate(line, modalWidth-4, "…"))
	}
	b.WriteString("\n")
	b.WriteString(m.styles.hint.Render(m.keyBadge("↑↓", "Scroll") + "  " + m.keyBadge("Esc", "Close")))
	return m.overlay(view, m.modalStyle(modalWidth).Height(modalHeight-2).Render(b.String()), width, height)
}

func (m *model) scrollHelp(delta int) {
	m.helpOffset += delta
	if m.helpOffset < 0 {
		m.helpOffset = 0
	}
	sections := helpSections()
	lines := helpLines(sections, 56)
	maxOffset := max(0, len(lines)-m.helpPageSize())
	if m.helpOffset > maxOffset {
		m.helpOffset = maxOffset
	}
}
