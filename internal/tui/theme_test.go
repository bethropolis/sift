package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"

	"github.com/bethropolis/sift/internal/highlight"
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
	idx := 1 // e.g. Tokyo Night
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
	p := ThemePresets[idx]
	if m.styles.border != p.Border {
		t.Errorf("border color = %q, want %q", m.styles.border, p.Border)
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
	m.applyTheme(ThemePresets[1])
	second := m.syntaxCache.Get("main.go", content, m.highlight)
	if first != second {
		t.Fatal("theme switch reparsed cached document")
	}
	oldLine := highlight.RenderDocumentLine(first, 0, m.highlight)
	m.applyTheme(ThemePresets[2])
	newLine := highlight.RenderDocumentLine(second, 0, m.highlight)
	if oldLine == newLine {
		t.Fatal("theme switch did not change semantic rendering")
	}
}

func TestPersistedThemeInitializesModel(t *testing.T) {
	m := newModel(BuildTree([]Item{{Path: "a.go"}}), Options{UITheme: ThemePresets[1].Name})
	if m.themeIndex != 1 || m.themeCursor != 1 {
		t.Fatalf("theme indexes = (%d, %d), want (1, 1)", m.themeIndex, m.themeCursor)
	}
	if m.styles.border != ThemePresets[1].Border {
		t.Errorf("initial border = %q, want %q", m.styles.border, ThemePresets[1].Border)
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
	m1.applyTheme(ThemePresets[1])
	if after := m2.styles.title.Render("x"); after != before {
		t.Error("theme change on one model leaked into another model's title style")
	}
}

func TestThemeNormalizationCompletesSemanticRoles(t *testing.T) {
	for _, preset := range ThemePresets {
		normalized := normalizeTheme(preset)
		roles := []lipgloss.Color{
			normalized.UI.Text, normalized.UI.TextMuted, normalized.UI.TextSubtle,
			normalized.UI.Accent, normalized.UI.AccentSoft, normalized.UI.Border,
			normalized.UI.BorderActive, normalized.UI.CursorBg, normalized.UI.CursorFg,
			normalized.UI.SelectionBg, normalized.UI.SelectionFg, normalized.UI.TreeGuide,
			normalized.UI.Scrollbar, normalized.UI.ModeFull, normalized.UI.ModeSig, normalized.UI.ModeSkip,
			normalized.Status.Info, normalized.Status.Success, normalized.Status.Warning, normalized.Status.Error,
		}
		for i, role := range roles {
			if role == "" {
				t.Errorf("theme %q role %d normalized empty", preset.ID, i)
			}
		}
	}
}

func TestThemeIdentityAndLegacyResolution(t *testing.T) {
	seen := map[string]bool{}
	for i, preset := range ThemePresets {
		if preset.ID == "" || seen[preset.ID] {
			t.Fatalf("theme %d has missing or duplicate ID %q", i, preset.ID)
		}
		seen[preset.ID] = true
		if themeIndex(preset.ID) != i {
			t.Errorf("theme ID %q did not resolve to index %d", preset.ID, i)
		}
	}
	for _, legacy := range []string{"Catppuccin Mocha", "Tokyo Night", "Rose Pine", "Classic (Default)"} {
		if got := themeIndex(legacy); got == defaultThemeIndex() && legacy != "Classic (Default)" {
			t.Errorf("legacy theme %q fell back to Classic", legacy)
		}
	}
}

func TestThemeSelectionPersistsStableID(t *testing.T) {
	var got string
	m := newModel(BuildTree([]Item{{Path: "a.go"}}), Options{OnThemeChange: func(id string) error { got = id; return nil }})
	m = updateKey(m, tea.KeyRunes, 't')
	m = updateKey(m, tea.KeyUp)
	m = updateKey(m, tea.KeyEnter)
	if got != themeID(5) {
		t.Fatalf("persisted theme = %q, want %q", got, themeID(5))
	}
}

func TestThemeMoveClamps(t *testing.T) {
	m := newModel(BuildTree([]Item{{Path: "a.go"}}), Options{})
	m.themeOpen = true

	// Scrolling far below the list clamps to the last preset.
	for i := 0; i < len(ThemePresets)+5; i++ {
		m = updateKey(m, tea.KeyDown)
	}
	if m.themeCursor != len(ThemePresets)-1 {
		t.Fatalf("themeCursor = %d, want %d", m.themeCursor, len(ThemePresets)-1)
	}

	// Scrolling far above clamps back to the first preset.
	for i := 0; i < len(ThemePresets)+5; i++ {
		m = updateKey(m, tea.KeyUp)
	}
	if m.themeCursor != 0 {
		t.Fatalf("themeCursor = %d after scroll up, want 0", m.themeCursor)
	}
}

func TestDefaultThemeIsClassic(t *testing.T) {
	if idx := defaultThemeIndex(); idx < 0 || idx >= len(ThemePresets) {
		t.Fatalf("defaultThemeIndex = %d out of range", idx)
	}
	if p := ThemePresets[defaultThemeIndex()]; p.Name != "Classic (Default)" {
		t.Errorf("default theme = %q, want Classic (Default)", p.Name)
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
	if m.styles.accent != ThemePresets[active].Title {
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

	m.applyTheme(ThemePresets[1]) // Tokyo Night
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
	m1.applyTheme(ThemePresets[2]) // Dracula
	got := highlight.RenderLine("b.go", "func x() {}", m2.highlight)
	if !strings.Contains(got, "\033[1;34mfunc\033[0m") {
		t.Errorf("theme change leaked into another model: %q", got)
	}
}

func TestTerminalThemeFollowsANSIColors(t *testing.T) {
	idx := themeIndex("terminal")
	if idx < 0 {
		t.Fatal("terminal theme was not registered")
	}
	p := ThemePresets[idx]
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
func TestWebThemeIDsResolveInTUI(t *testing.T) {
	webIDs := []string{
		"catppuccin-mocha", "tokyo-night", "nord", "gruvbox-dark", "gruvbox-light",
		"dracula", "rose-pine", "one-dark", "github-dark", "github-light",
		"monokai", "classic-dark", "classic-light",
	}
	for _, id := range webIDs {
		idx := themeIndex(id)
		if ThemePresets[idx].ID != id {
			t.Errorf("web theme %q resolves to TUI preset %q", id, ThemePresets[idx].ID)
		}
	}
}
