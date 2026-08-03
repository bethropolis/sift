package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// ThemePreset is an instance-local color palette. Applying a preset rebuilds
// the model's own styles; it never mutates package-level state.
type ThemePreset struct {
	Name     string
	Border   lipgloss.Color
	Title    lipgloss.Color
	CursorBg lipgloss.Color
	CursorFg lipgloss.Color
	Selected lipgloss.Color
	Notice   lipgloss.Color
	ModeFull lipgloss.Color
	ModeSig  lipgloss.Color
	ModeSkip lipgloss.Color
}

// ThemePresets lists every selectable theme in the interactive t menu. The
// Classic/Default entry reproduces the original ANSI palette.
var ThemePresets = []ThemePreset{
	{
		Name: "Catppuccin Mocha", Border: lipgloss.Color("#cba6f7"), Title: lipgloss.Color("#89b4fa"),
		CursorBg: lipgloss.Color("#313244"), CursorFg: lipgloss.Color("#cdd6f4"), Selected: lipgloss.Color("#89b4fa"),
		Notice: lipgloss.Color("#a6e3a1"), ModeFull: lipgloss.Color("#a6e3a1"), ModeSig: lipgloss.Color("#f9e2af"), ModeSkip: lipgloss.Color("#f38ba8"),
	},
	{
		Name: "Tokyo Night", Border: lipgloss.Color("#7aa2f7"), Title: lipgloss.Color("#7dcfff"),
		CursorBg: lipgloss.Color("#2ac3de"), CursorFg: lipgloss.Color("#1a1b26"), Selected: lipgloss.Color("#7aa2f7"),
		Notice: lipgloss.Color("#9ece6a"), ModeFull: lipgloss.Color("#9ece6a"), ModeSig: lipgloss.Color("#e0af68"), ModeSkip: lipgloss.Color("#f7768e"),
	},
	{
		Name: "Dracula", Border: lipgloss.Color("#bd93f9"), Title: lipgloss.Color("#8be9fd"),
		CursorBg: lipgloss.Color("#44475a"), CursorFg: lipgloss.Color("#f8f8f2"), Selected: lipgloss.Color("#ff79c6"),
		Notice: lipgloss.Color("#50fa7b"), ModeFull: lipgloss.Color("#50fa7b"), ModeSig: lipgloss.Color("#f1fa8c"), ModeSkip: lipgloss.Color("#ff5555"),
	},
	{
		Name: "Gruvbox Dark", Border: lipgloss.Color("#d3869b"), Title: lipgloss.Color("#83a598"),
		CursorBg: lipgloss.Color("#3c3836"), CursorFg: lipgloss.Color("#ebdbb2"), Selected: lipgloss.Color("#83a598"),
		Notice: lipgloss.Color("#b8bb26"), ModeFull: lipgloss.Color("#b8bb26"), ModeSig: lipgloss.Color("#fabd2f"), ModeSkip: lipgloss.Color("#fb4934"),
	},
	{
		Name: "Nord", Border: lipgloss.Color("#88c0d0"), Title: lipgloss.Color("#81a1c1"),
		CursorBg: lipgloss.Color("#3b4252"), CursorFg: lipgloss.Color("#eceff4"), Selected: lipgloss.Color("#88c0d0"),
		Notice: lipgloss.Color("#a3be8c"), ModeFull: lipgloss.Color("#a3be8c"), ModeSig: lipgloss.Color("#ebcb8b"), ModeSkip: lipgloss.Color("#bf616a"),
	},
	{
		Name: "Rose Pine", Border: lipgloss.Color("#c4a7e7"), Title: lipgloss.Color("#e0def4"),
		CursorBg: lipgloss.Color("#ebbcba"), CursorFg: lipgloss.Color("#191724"), Selected: lipgloss.Color("#31748f"),
		Notice: lipgloss.Color("#9ccfd8"), ModeFull: lipgloss.Color("#9ccfd8"), ModeSig: lipgloss.Color("#f6c177"), ModeSkip: lipgloss.Color("#eb6f92"),
	},
	{
		Name: "Classic (Default)", Border: lipgloss.Color("62"), Title: lipgloss.Color("12"),
		CursorBg: lipgloss.Color("236"), CursorFg: lipgloss.Color("15"), Selected: lipgloss.Color("12"),
		Notice: lipgloss.Color("10"), ModeFull: lipgloss.Color("10"), ModeSig: lipgloss.Color("11"), ModeSkip: lipgloss.Color("9"),
	},
}

// defaultThemeIndex returns the index of the Classic (Default) preset, which
// is the initial theme.
func defaultThemeIndex() int {
	for i, p := range ThemePresets {
		if p.Name == "Classic (Default)" {
			return i
		}
	}
	return 0
}

// applyTheme rebuilds the model's own styles from the preset. Because the
// styles live on the model, two pickers never share theme state and applying
// a theme cannot race another render.
func (m *model) applyTheme(p ThemePreset) {
	m.styles = uiStyles{
		title:     lipgloss.NewStyle().Bold(true).Foreground(p.Title),
		hint:      lipgloss.NewStyle().Foreground(p.Title),
		dim:       lipgloss.NewStyle().Foreground(p.Title).Faint(true),
		treeGuide: lipgloss.NewStyle().Foreground(p.Border),
		cursor:    lipgloss.NewStyle().Bold(true).Background(p.CursorBg).Foreground(p.CursorFg),
		scrollbar: lipgloss.NewStyle().Foreground(p.Border),
		selected:  lipgloss.NewStyle().Foreground(p.Selected),
		warning:   lipgloss.NewStyle().Bold(true).Foreground(p.ModeSkip),
		notice:    lipgloss.NewStyle().Foreground(p.Notice),
		modeFull:  lipgloss.NewStyle().Foreground(p.ModeFull),
		modeSig:   lipgloss.NewStyle().Foreground(p.ModeSig),
		modeSkip:  lipgloss.NewStyle().Foreground(p.ModeSkip),
		border:    p.Border,
		accent:    p.Title,
	}
}

// renderThemeModal overlays the theme selector on the view. It is bounded by
// the terminal dimensions and truncates every line to the modal width, so it
// can never wrap or destabilize the layout.
func (m *model) renderThemeModal(view string, width, height int) string {
	modalWidth := min(width, 48)
	if modalWidth < 30 {
		modalWidth = 30
	}
	bodyRows := max(1, min(len(ThemePresets), height-6))
	maxOffset := max(0, len(ThemePresets)-bodyRows)
	if m.themeOffset > maxOffset {
		m.themeOffset = maxOffset
	}
	if m.themeOffset < 0 {
		m.themeOffset = 0
	}
	if m.themeCursor < m.themeOffset {
		m.themeOffset = m.themeCursor
	}
	if m.themeCursor >= m.themeOffset+bodyRows {
		m.themeOffset = m.themeCursor - bodyRows + 1
	}

	var b strings.Builder
	b.WriteString(m.styles.title.Render(ansi.Truncate(" Select Color Theme ", modalWidth-4, "…")))
	for i := m.themeOffset; i < m.themeOffset+bodyRows && i < len(ThemePresets); i++ {
		b.WriteString("\n")
		mark := "  "
		if i == m.themeIndex {
			mark = "●"
		}
		cur := "  "
		if i == m.themeCursor {
			cur = "❯"
		}
		line := fmt.Sprintf("%s %s %s", cur, mark, ThemePresets[i].Name)
		line = ansi.Truncate(line, modalWidth-4, "…")
		if i == m.themeCursor {
			b.WriteString(m.styles.cursor.Render(line))
		} else {
			b.WriteString(line)
		}
	}
	b.WriteString("\n")
	b.WriteString(m.styles.hint.Render("↑/↓ move · Enter/Space apply · Esc/q close"))
	return m.overlay(view, m.boxStyle(modalWidth).Height(max(1, height-4)).Render(b.String()), width, height)
}
