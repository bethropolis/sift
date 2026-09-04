package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/bethropolis/sift/internal/highlight"
)

// ThemePreset is an instance-local color palette. Applying a preset rebuilds
// the model's own styles; it never mutates package-level state.
type ThemePreset struct {
	Name     string
	Border   lipgloss.Color
	Title    lipgloss.Color
	Muted    lipgloss.Color
	CursorBg lipgloss.Color
	CursorFg lipgloss.Color
	Selected lipgloss.Color
	Notice   lipgloss.Color
	ModeFull lipgloss.Color
	ModeSig  lipgloss.Color
	ModeSkip lipgloss.Color
	// Highlight recolors preview syntax highlighting to match the preset.
	// It is applied to the model's highlight options on every theme change.
	Highlight highlight.Palette
}

// hlPalette builds a preview-highlight palette from theme hex colors:
// keyword (bold), string, number, comment (faint), type name.
func hlPalette(keyword, str, number, comment, typeName string) highlight.Palette {
	return highlight.Palette{
		Keyword:  highlight.ANSI(keyword, "1"),
		String:   highlight.ANSI(str),
		Number:   highlight.ANSI(number),
		Comment:  highlight.ANSI(comment, "2"),
		TypeName: highlight.ANSI(typeName),
	}
}

// ThemePresets lists every selectable theme in the interactive t menu. The
// Classic/Default entry reproduces the original ANSI palette.
var ThemePresets = []ThemePreset{
	{
		Name: "Catppuccin Mocha", Border: lipgloss.Color("#cba6f7"), Title: lipgloss.Color("#89b4fa"), Muted: lipgloss.Color("#6c7086"),
		CursorBg: lipgloss.Color("#313244"), CursorFg: lipgloss.Color("#cdd6f4"), Selected: lipgloss.Color("#89b4fa"),
		Notice: lipgloss.Color("#a6e3a1"), ModeFull: lipgloss.Color("#a6e3a1"), ModeSig: lipgloss.Color("#f9e2af"), ModeSkip: lipgloss.Color("#f38ba8"),
		Highlight: hlPalette("#cba6f7", "#a6e3a1", "#fab387", "#6c7086", "#f9e2af"),
	},
	{
		Name: "Tokyo Night", Border: lipgloss.Color("#7aa2f7"), Title: lipgloss.Color("#7dcfff"), Muted: lipgloss.Color("#565f89"),
		CursorBg: lipgloss.Color("#2ac3de"), CursorFg: lipgloss.Color("#1a1b26"), Selected: lipgloss.Color("#7aa2f7"),
		Notice: lipgloss.Color("#9ece6a"), ModeFull: lipgloss.Color("#9ece6a"), ModeSig: lipgloss.Color("#e0af68"), ModeSkip: lipgloss.Color("#f7768e"),
		Highlight: hlPalette("#bb9af7", "#9ece6a", "#ff9e64", "#565f89", "#7dcfff"),
	},
	{
		Name: "Dracula", Border: lipgloss.Color("#bd93f9"), Title: lipgloss.Color("#8be9fd"), Muted: lipgloss.Color("#6272a4"),
		CursorBg: lipgloss.Color("#44475a"), CursorFg: lipgloss.Color("#f8f8f2"), Selected: lipgloss.Color("#ff79c6"),
		Notice: lipgloss.Color("#50fa7b"), ModeFull: lipgloss.Color("#50fa7b"), ModeSig: lipgloss.Color("#f1fa8c"), ModeSkip: lipgloss.Color("#ff5555"),
		Highlight: hlPalette("#ff79c6", "#f1fa8c", "#bd93f9", "#6272a4", "#8be9fd"),
	},
	{
		Name: "Gruvbox Dark", Border: lipgloss.Color("#d3869b"), Title: lipgloss.Color("#83a598"), Muted: lipgloss.Color("#928374"),
		CursorBg: lipgloss.Color("#3c3836"), CursorFg: lipgloss.Color("#ebdbb2"), Selected: lipgloss.Color("#83a598"),
		Notice: lipgloss.Color("#b8bb26"), ModeFull: lipgloss.Color("#b8bb26"), ModeSig: lipgloss.Color("#fabd2f"), ModeSkip: lipgloss.Color("#fb4934"),
		Highlight: hlPalette("#fb4934", "#b8bb26", "#d3869b", "#928374", "#fabd2f"),
	},
	{
		Name: "Nord", Border: lipgloss.Color("#88c0d0"), Title: lipgloss.Color("#81a1c1"), Muted: lipgloss.Color("#4c566a"),
		CursorBg: lipgloss.Color("#3b4252"), CursorFg: lipgloss.Color("#eceff4"), Selected: lipgloss.Color("#88c0d0"),
		Notice: lipgloss.Color("#a3be8c"), ModeFull: lipgloss.Color("#a3be8c"), ModeSig: lipgloss.Color("#ebcb8b"), ModeSkip: lipgloss.Color("#bf616a"),
		Highlight: hlPalette("#81a1c1", "#a3be8c", "#b48ead", "#4c566a", "#88c0d0"),
	},
	{
		Name: "Rose Pine", Border: lipgloss.Color("#c4a7e7"), Title: lipgloss.Color("#e0def4"), Muted: lipgloss.Color("#6e6a86"),
		CursorBg: lipgloss.Color("#ebbcba"), CursorFg: lipgloss.Color("#191724"), Selected: lipgloss.Color("#31748f"),
		Notice: lipgloss.Color("#9ccfd8"), ModeFull: lipgloss.Color("#9ccfd8"), ModeSig: lipgloss.Color("#f6c177"), ModeSkip: lipgloss.Color("#eb6f92"),
		Highlight: hlPalette("#c4a7e7", "#9ccfd8", "#f6c177", "#6e6a86", "#ebbcba"),
	},
	{
		Name: "Classic (Default)", Border: lipgloss.Color("62"), Title: lipgloss.Color("12"), Muted: lipgloss.Color("245"),
		CursorBg: lipgloss.Color("236"), CursorFg: lipgloss.Color("15"), Selected: lipgloss.Color("12"),
		Notice: lipgloss.Color("10"), ModeFull: lipgloss.Color("10"), ModeSig: lipgloss.Color("11"), ModeSkip: lipgloss.Color("9"),
		// Classic keeps the legacy ANSI palette byte-for-byte so the default
		// preview rendering is unchanged.
		Highlight: highlight.Palette{Keyword: "\033[1;34m", String: "\033[32m", Number: "\033[36m", Comment: "\033[2m", TypeName: "\033[35m"},
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

// themeIndex resolves a persisted theme name while tolerating a missing or
// stale preference after themes are renamed or removed.
func themeIndex(name string) int {
	if name != "" {
		for i, p := range ThemePresets {
			if p.Name == name {
				return i
			}
		}
	}
	return defaultThemeIndex()
}

// applyTheme rebuilds the model's own styles from the preset. Because the
// styles live on the model, two pickers never share theme state and applying
// a theme cannot race another render. The preset's highlight palette is
// installed alongside the chrome styles so preview syntax highlighting
// follows the theme; highlighting stays off entirely when disabled.
func (m *model) applyTheme(p ThemePreset) {
	m.styles = uiStyles{
		title:     lipgloss.NewStyle().Bold(true).Foreground(p.Title),
		hint:      lipgloss.NewStyle().Foreground(p.Title),
		dim:       lipgloss.NewStyle().Foreground(p.Title).Faint(true),
		muted:     lipgloss.NewStyle().Foreground(p.Muted),
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
	m.highlight.Palette = &p.Highlight
}

// renderThemeModal overlays the theme selector on the view. It is bounded by
// the terminal dimensions and truncates every line to the modal width, so it
// can never wrap or destabilize the layout.
func (m *model) renderThemeModal(view string, width, height int) string {
	modalWidth := min(width, 52)
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
	title := fmt.Sprintf(" %sColor Theme ", m.glyphs.Theme)
	b.WriteString(m.styles.title.Render(ansi.Truncate(title, modalWidth-4, "…")))
	for i := m.themeOffset; i < m.themeOffset+bodyRows && i < len(ThemePresets); i++ {
		b.WriteString("\n")
		p := ThemePresets[i]
		mark := "  "
		if i == m.themeIndex {
			mark = m.styles.notice.Render("●")
		}
		cur := "  "
		if i == m.themeCursor {
			cur = m.styles.title.Render("❯")
		}
		// Three color swatches: border, title, selected
		swatch := lipgloss.NewStyle().Foreground(p.Border).Render("■") +
			lipgloss.NewStyle().Foreground(p.Title).Render("■") +
			lipgloss.NewStyle().Foreground(p.Selected).Render("■")

		name := ansi.Truncate(p.Name, modalWidth-14, "…")
		line := fmt.Sprintf("%s %s %s %s", cur, mark, swatch, name)
		if i == m.themeCursor {
			b.WriteString(m.styles.cursor.Render(ansi.Truncate(line, modalWidth-4, "…")))
		} else {
			b.WriteString(ansi.Truncate(line, modalWidth-4, "…"))
		}
	}
	b.WriteString("\n")
	b.WriteString(m.styles.hint.Render("↑/↓ move · Enter/Space apply · Esc/q close"))
	return m.overlay(view, m.boxStyle(modalWidth).Height(max(1, height-4)).Render(b.String()), width, height)
}
