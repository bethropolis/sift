package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/bethropolis/sift/internal/highlight"
	"github.com/bethropolis/sift/internal/theme"
)

func (m *model) applyTheme(p theme.ThemePreset) {
	p = theme.Normalize(p)
	m.styles = uiStyles{
		appTitle:   lipgloss.NewStyle().Bold(true).Foreground(p.Title),
		title:      lipgloss.NewStyle().Bold(true).Foreground(p.Title),
		hint:       lipgloss.NewStyle().Foreground(p.Muted),
		dim:        lipgloss.NewStyle().Foreground(p.Border).Faint(true),
		subtle:     lipgloss.NewStyle().Foreground(p.Border).Faint(true),
		muted:      lipgloss.NewStyle().Foreground(p.Muted),
		treeGuide:  lipgloss.NewStyle().Foreground(p.Border),
		cursor:     lipgloss.NewStyle().Bold(true).Background(p.CursorBg).Foreground(p.CursorFg),
		scrollbar:  lipgloss.NewStyle().Foreground(p.Muted),
		selected:   lipgloss.NewStyle().Foreground(p.Selected),
		warning:    lipgloss.NewStyle().Bold(true).Foreground(p.Status.Warning),
		notice:     lipgloss.NewStyle().Foreground(p.Status.Info),
		modeFull:   lipgloss.NewStyle().Foreground(p.UI.ModeFull),
		modeSig:    lipgloss.NewStyle().Foreground(p.UI.ModeSig),
		modeSkip:   lipgloss.NewStyle().Foreground(p.UI.ModeSkip),
		danger:     lipgloss.NewStyle().Foreground(p.Status.Error),
		success:    lipgloss.NewStyle().Foreground(p.Status.Success),
		border:     p.UI.Border,
		accent:     p.UI.Accent,
		accentSoft: p.UI.AccentSoft,
	}
	m.highlight.Palette = &p.Highlight
	m.highlight.Syntax = &p.Syntax
}

func themePreviewStyles(p theme.ThemePreset) uiStyles {
	p = theme.Normalize(p)
	return uiStyles{
		appTitle:   lipgloss.NewStyle().Bold(true).Foreground(p.UI.Accent),
		title:      lipgloss.NewStyle().Bold(true).Foreground(p.UI.Accent),
		hint:       lipgloss.NewStyle().Foreground(p.UI.TextMuted),
		muted:      lipgloss.NewStyle().Foreground(p.UI.TextMuted),
		subtle:     lipgloss.NewStyle().Foreground(p.UI.TextSubtle),
		dim:        lipgloss.NewStyle().Foreground(p.UI.TextSubtle).Faint(true),
		treeGuide:  lipgloss.NewStyle().Foreground(p.UI.TreeGuide),
		cursor:     lipgloss.NewStyle().Bold(true).Background(p.UI.CursorBg).Foreground(p.UI.CursorFg),
		scrollbar:  lipgloss.NewStyle().Foreground(p.UI.Scrollbar),
		selected:   lipgloss.NewStyle().Foreground(p.UI.AccentSoft),
		warning:    lipgloss.NewStyle().Bold(true).Foreground(p.Status.Warning),
		notice:     lipgloss.NewStyle().Foreground(p.Status.Info),
		modeFull:   lipgloss.NewStyle().Foreground(p.UI.ModeFull),
		modeSig:    lipgloss.NewStyle().Foreground(p.UI.ModeSig),
		modeSkip:   lipgloss.NewStyle().Foreground(p.UI.ModeSkip),
		danger:     lipgloss.NewStyle().Foreground(p.Status.Error),
		success:    lipgloss.NewStyle().Foreground(p.Status.Success),
		border:     p.UI.Border,
		accent:     p.UI.Accent,
		accentSoft: p.UI.AccentSoft,
	}
}

// themePreviewLines renders a miniature Sift surface for a candidate theme
// without mutating the active model. It deliberately uses the same semantic
// palette and ANSI renderer as the live preview.
func themePreviewLines(p theme.ThemePreset, width int) []string {
	p = theme.Normalize(p)
	styles := themePreviewStyles(p)
	inner := max(10, width-4)
	lines := []string{
		styles.title.Render("Theme Preview"),
		styles.hint.Render(p.Family + " · " + p.Variant + " · " + p.ID),
		styles.treeGuide.Render("  " + string(p.UI.BorderActive) + " internal/"),
		styles.cursor.Render("▸ app.go") + "  " + styles.modeFull.Render("[FULL]") + "  " + styles.hint.Render("1.8k"),
		styles.selected.Render("  config.go") + "  " + styles.modeSig.Render("[SIG]") + "  " + styles.hint.Render("0.7k"),
	}
	doc := highlight.Parse("preview.go", []byte("func greet() string {\n\treturn \"ready\"\n}"), highlight.Options{Enabled: true, Theme: highlight.ThemeAuto, Syntax: &p.Syntax})
	code := []string{
		highlight.RenderDocumentLine(doc, 0, highlight.Options{Enabled: true, Theme: highlight.ThemeAuto, Syntax: &p.Syntax}),
		highlight.RenderDocumentLine(doc, 1, highlight.Options{Enabled: true, Theme: highlight.ThemeAuto, Syntax: &p.Syntax}),
	}
	lines = append(lines, code...)
	lines = append(lines, styles.success.Render("✓ Ready")+"  "+styles.warning.Render("⚠ Warning"))
	for i := range lines {
		lines[i] = ansi.Truncate(lines[i], inner, "…")
	}
	return lines
}
func (m *model) renderThemeModal(view string, width, height int) string {
	modalWidth := min(width, 68)
	if modalWidth < 30 {
		modalWidth = 30
	}
	showPreview := modalWidth >= 52 && height >= 18
	previewRows := 0
	if showPreview {
		previewRows = min(9, max(0, height-9))
	}
	bodyRows := max(1, min(len(m.themes), height-6-previewRows))
	maxOffset := max(0, len(m.themes)-bodyRows)
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
	b.WriteString(m.modalTitle(m.glyphs.Theme, "Color Theme", modalWidth))
	for i := m.themeOffset; i < m.themeOffset+bodyRows && i < len(m.themes); i++ {
		b.WriteString("\n")
		p := m.themes[i]
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
	if showPreview {
		b.WriteString("\n")
		preview := themePreviewLines(m.themes[m.themeCursor], modalWidth)
		for i := 0; i < previewRows && i < len(preview); i++ {
			b.WriteString("\n")
			b.WriteString(preview[i])
		}
	}
	b.WriteString("\n")
	b.WriteString(m.styles.hint.Render("↑/↓ move · Enter/Space apply · Esc/q close"))
	modalHeight := max(1, height-4)
	if showPreview {
		modalHeight = min(modalHeight, lipgloss.Height(b.String())+2)
	}
	return m.overlay(view, m.modalStyle(modalWidth).Height(modalHeight).Render(b.String()), width, height)
}
