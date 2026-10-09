package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"

	"github.com/bethropolis/sift/internal/highlight"
	"github.com/bethropolis/sift/internal/theme"
)

func TestThemeToggle(t *testing.T) {
	m := newModel(BuildTree([]Item{{Path: "a.go"}}), Options{})

	m = updateKey(m, tea.KeyRunes, 't')
	if !m.themeOpen {
		t.Fatal("t did not open theme modal")
	}
	if view := m.View(); !strings.Contains(view, "Color Theme") {
		t.Errorf("theme view missing title: %q", view)
	}

	m = updateKey(m, tea.KeyRunes, 't')
	if m.themeOpen {
		t.Error("second t did not close theme modal")
	}
}

func TestThemeCloseKeys(t *testing.T) {
	m := newModel(BuildTree([]Item{{Path: "a.go"}}), Options{})
	m = updateKey(m, tea.KeyRunes, 't')
	if !m.themeOpen {
		t.Fatal("theme did not open")
	}

	m = updateKey(m, tea.KeyEsc)
	if m.themeOpen {
		t.Error("Esc did not close theme modal")
	}
	if m.quit {
		t.Error("Esc in theme modal should not quit the picker")
	}
}

func TestThemeSelectAppliesPreset(t *testing.T) {
	m := newModel(BuildTree([]Item{{Path: "a.go"}}), Options{})
	idx := theme.IndexOf(theme.ThemePresets, "tokyo-night")
	m = updateKey(m, tea.KeyRunes, 't')
	if !m.themeOpen {
		t.Fatal("theme did not open")
	}
	for i := m.themeCursor; i > idx; i-- {
		m = updateKey(m, tea.KeyUp)
	}
	for i := m.themeCursor; i < idx; i++ {
		m = updateKey(m, tea.KeyDown)
	}
	m = updateKey(m, tea.KeyEnter)

	if m.themeOpen {
		t.Error("Enter did not close theme modal")
	}
	p := theme.ThemePresets[idx]
	if m.styles.border != p.UI.Border {
		t.Errorf("border color = %q, want %q", m.styles.border, p.UI.Border)
	}
	if m.styles.accent != p.Title {
		t.Errorf("accent color = %q, want %q", m.styles.accent, p.Title)
	}
	// The title style must render with an ANSI color sequence for the preset.
	// Force a color profile so the assertion does not depend on the test
	// terminal's TTY state.
	lipgloss.SetColorProfile(termenv.TrueColor)
	lipgloss.SetHasDarkBackground(true)
	rendered := m.styles.title.Render("x")
	if !strings.Contains(rendered, "\x1b[") {
		t.Errorf("title render %q carries no color sequence", rendered)
	}
	// A distinct preset must produce a visibly different title render.
	want := lipgloss.NewStyle().Bold(true).Foreground(p.Title).Render("x")
	if rendered != want {
		t.Errorf("title render = %q, want %q", rendered, want)
	}
}

func TestThemeSwitchReusesCachedDocument(t *testing.T) {
	content := []byte("func greet() { return true }")
	m := newModel(BuildTree([]Item{{Path: "main.go", Content: content}}), Options{Highlight: true, UITheme: "classic"})
	first := m.syntaxCache.Get("main.go", content, m.highlight)
	m.applyTheme(theme.ThemePresets[theme.IndexOf(theme.ThemePresets, "tokyo-night")])
	second := m.syntaxCache.Get("main.go", content, m.highlight)
	if first != second {
		t.Fatal("theme switch reparsed cached document")
	}
	oldLine := highlight.RenderDocumentLine(first, 0, m.highlight)
	m.applyTheme(theme.ThemePresets[2])
	newLine := highlight.RenderDocumentLine(second, 0, m.highlight)
	if oldLine == newLine {
		t.Fatal("theme switch did not change semantic rendering")
	}
}

func TestPersistedThemeInitializesModel(t *testing.T) {
	idx := theme.IndexOf(theme.ThemePresets, "tokyo-night")
	m := newModel(BuildTree([]Item{{Path: "a.go"}}), Options{UITheme: theme.ThemePresets[idx].Name})
	if m.themeIndex != idx || m.themeCursor != idx {
		t.Fatalf("theme indexes = (%d, %d), want (%d, %d)", m.themeIndex, m.themeCursor, idx, idx)
	}
	if m.styles.border != theme.ThemePresets[idx].UI.Border {
		t.Errorf("initial border = %q, want %q", m.styles.border, theme.ThemePresets[idx].UI.Border)
	}
}

func TestThemeSelectionNotifiesPersistenceCallback(t *testing.T) {
	var got string
	m := newModel(BuildTree([]Item{{Path: "a.go"}}), Options{
		OnThemeChange: func(name string) error {
			got = name
			return nil
		},
	})
	m = updateKey(m, tea.KeyRunes, 't')
	initial := m.themeCursor
	m = updateKey(m, tea.KeyUp)
	m = updateKey(m, tea.KeyEnter)
	want := m.themes[initial-1].ID
	if got != want {
		t.Fatalf("persisted theme = %q, want %q", got, want)
	}
}

func TestThemeApplyIsInstanceLocal(t *testing.T) {
	// Two models start from the same preset list; applying a theme to one must
	// not leak into the other's styles.
	m1 := newModel(BuildTree([]Item{{Path: "a.go"}}), Options{})
	m2 := newModel(BuildTree([]Item{{Path: "b.go"}}), Options{})

	before := m2.styles.title.Render("x")
	m1.applyTheme(theme.ThemePresets[theme.IndexOf(theme.ThemePresets, "tokyo-night")])
	if after := m2.styles.title.Render("x"); after != before {
		t.Error("theme change on one model leaked into another model's title style")
	}
}

func TestThemeSelectionPersistsStableID(t *testing.T) {
	var got string
	m := newModel(BuildTree([]Item{{Path: "a.go"}}), Options{OnThemeChange: func(id string) error { got = id; return nil }})
	m = updateKey(m, tea.KeyRunes, 't')
	initial := m.themeCursor
	m = updateKey(m, tea.KeyUp)
	m = updateKey(m, tea.KeyEnter)
	if want := theme.IDAt(theme.ThemePresets, initial-1); got != want {
		t.Fatalf("persisted theme = %q, want %q", got, want)
	}
}

func TestThemeMoveClamps(t *testing.T) {
	m := newModel(BuildTree([]Item{{Path: "a.go"}}), Options{})
	m.themeOpen = true

	// Scrolling far below the list clamps to the last preset.
	for i := 0; i < len(theme.ThemePresets)+5; i++ {
		m = updateKey(m, tea.KeyDown)
	}
	if m.themeCursor != len(theme.ThemePresets)-1 {
		t.Fatalf("themeCursor = %d, want %d", m.themeCursor, len(theme.ThemePresets)-1)
	}

	// Scrolling far above clamps back to the first preset.
	for i := 0; i < len(theme.ThemePresets)+5; i++ {
		m = updateKey(m, tea.KeyUp)
	}
	if m.themeCursor != 0 {
		t.Fatalf("themeCursor = %d after scroll up, want 0", m.themeCursor)
	}
}

func TestThemeModalShowsCandidatePreview(t *testing.T) {
	m := newModel(BuildTree([]Item{{Path: "main.go", Content: []byte("func main() {}")}}), Options{Highlight: true})
	m.width, m.height = 100, 28
	m = updateKey(m, tea.KeyRunes, 't')
	view := ansi.Strip(m.View())
	for _, want := range []string{"Theme Preview", "Catppuccin", "app.go", "func", "Ready", "Warning"} {
		if !strings.Contains(view, want) {
			t.Errorf("theme preview missing %q: %q", want, view)
		}
	}
	if m.themeOpen != true {
		t.Fatal("theme modal closed while previewing")
	}
}

func TestThemePreviewDoesNotMutateActiveTheme(t *testing.T) {
	m := newModel(BuildTree([]Item{{Path: "main.go"}}), Options{})
	active := m.themeIndex
	m = updateKey(m, tea.KeyRunes, 't')
	m = updateKey(m, tea.KeyDown)
	if m.themeIndex != active {
		t.Fatalf("moving preview changed active theme index to %d", m.themeIndex)
	}
	if m.styles.accent != theme.ThemePresets[active].Title {
		t.Fatal("moving preview mutated active styles")
	}
}

func TestThemeModalFallsBackAtNarrowSize(t *testing.T) {
	m := newModel(BuildTree([]Item{{Path: "a.go"}}), Options{})
	m.width, m.height = 40, 12
	m.themeOpen = true
	view := ansi.Strip(m.View())
	if strings.Contains(view, "Theme Preview") {
		t.Errorf("narrow theme modal unexpectedly showed preview: %q", view)
	}
	if lines := strings.Count(view, "\n") + 1; lines > m.height {
		t.Errorf("narrow theme modal exceeds height: %d", lines)
	}
}

func TestThemeRendersNoPanicOnSmallViewport(t *testing.T) {
	m := newModel(BuildTree([]Item{{Path: "a.go"}}), Options{})
	m.themeOpen = true
	m.width, m.height = 20, 3
	view := m.View()
	if view == "" {
		t.Fatal("theme view empty on small viewport")
	}
}

func TestThemeRenderKeepsLayoutBounded(t *testing.T) {
	m := newModel(BuildTree([]Item{{Path: "a.go"}}), Options{})
	m.themeOpen = true
	m.width, m.height = 60, 12
	view := m.View()
	if lines := strings.Count(view, "\n") + 1; lines > m.height {
		t.Errorf("theme view exceeds viewport height: %d lines", lines)
	}
}

func TestStylesDefaultPalette(t *testing.T) {
	// The classic default styles must keep their recognizable ANSI colors.
	s := defaultStyles()
	if s.border != lipgloss.Color("62") {
		t.Errorf("default border color = %q, want 62", s.border)
	}
	if s.accent != lipgloss.Color("12") {
		t.Errorf("default accent color = %q, want 12", s.accent)
	}
}

func TestThemeApplySetsHighlightPalette(t *testing.T) {
	m := newModel(BuildTree([]Item{{Path: "a.go"}}), Options{Highlight: true})
	if m.highlight.Palette == nil {
		t.Fatal("initial model has no highlight palette")
	}
	// The default Classic preset must keep the legacy ANSI rendering.
	if got := highlight.RenderLine("a.go", "func x() {}", m.highlight); !strings.Contains(got, "\033[1;34mfunc\033[0m") {
		t.Errorf("classic highlight = %q, want legacy keyword code", got)
	}

	m.applyTheme(theme.ThemePresets[theme.IndexOf(theme.ThemePresets, "tokyo-night")])
	if m.highlight.Palette == nil {
		t.Fatal("applied theme left highlight palette nil")
	}
	got := highlight.RenderLine("a.go", "func x() {}", m.highlight)
	if !strings.Contains(got, "\033[1;38;2;187;154;247mfunc\033[0m") {
		t.Errorf("tokyo night highlight = %q, want mauve keyword", got)
	}
	if strings.Contains(got, "\033[1;34m") {
		t.Errorf("legacy palette leaked into tokyo night render: %q", got)
	}
}

func TestThemeApplyIsInstanceLocalForHighlight(t *testing.T) {
	m1 := newModel(BuildTree([]Item{{Path: "a.go"}}), Options{Highlight: true})
	m2 := newModel(BuildTree([]Item{{Path: "b.go"}}), Options{Highlight: true})
	m1.applyTheme(theme.ThemePresets[theme.IndexOf(theme.ThemePresets, "dracula")])
	got := highlight.RenderLine("b.go", "func x() {}", m2.highlight)
	if !strings.Contains(got, "\033[1;34mfunc\033[0m") {
		t.Errorf("theme change leaked into another model: %q", got)
	}
}

func TestTerminalThemeFollowsANSIColors(t *testing.T) {
	idx := theme.IndexOf(theme.ThemePresets, "terminal")
	if idx < 0 {
		t.Fatal("terminal theme was not registered")
	}
	p := theme.ThemePresets[idx]
	if !p.FollowTerminal || p.ID != "terminal" {
		t.Fatalf("terminal theme = %+v", p)
	}
	for name, color := range map[string]lipgloss.Color{
		"border": p.Border, "title": p.Title, "muted": p.Muted,
		"cursor background": p.CursorBg, "cursor foreground": p.CursorFg,
		"selected": p.Selected, "notice": p.Notice,
	} {
		if strings.HasPrefix(string(color), "#") {
			t.Errorf("terminal theme %s uses fixed RGB %q", name, color)
		}
	}
	if p.Syntax[highlight.TokenKeyword].Foreground != "ansi:33" {
		t.Fatalf("terminal syntax keyword = %q, want extended ANSI index", p.Syntax[highlight.TokenKeyword].Foreground)
	}
	content := []byte("func main() { return true }")
	previous := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(previous)
	m := newModel(BuildTree([]Item{{Path: "main.go", Content: content}}), Options{Highlight: true, UITheme: "terminal"})
	m.highlight.Profile = highlight.ProfileTrueColor
	if m.styles.border != lipgloss.Color("8") || m.styles.accent != lipgloss.Color("15") {
		t.Fatalf("terminal styles = border %q accent %q", m.styles.border, m.styles.accent)
	}
	line := m.styles.title.Render("x")
	if strings.Contains(line, "38;2;") || !strings.Contains(line, "\x1b[") {
		t.Errorf("terminal title render is not ANSI-palette based: %q", line)
	}
	doc := m.syntaxCache.Get("main.go", content, m.highlight)
	rendered := highlight.RenderDocumentLine(doc, 0, m.highlight)
	if strings.Contains(rendered, "38;2;") || !strings.Contains(rendered, "38;5;33") {
		t.Errorf("terminal syntax render is not extended ANSI-palette based: %q", rendered)
	}
}

// TestWebThemeIDsResolveInTUI pins the shared theme contract: every web
// theme id selects a TUI preset with the same id, so one stored id drives
// both frontends.
