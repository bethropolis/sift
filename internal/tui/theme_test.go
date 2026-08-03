package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func TestThemeToggle(t *testing.T) {
	m := newModel(BuildTree([]Item{{Path: "a.go"}}), Options{})

	m = updateKey(m, tea.KeyRunes, 't')
	if !m.themeOpen {
		t.Fatal("t did not open theme modal")
	}
	if view := m.View(); !strings.Contains(view, "Select Color Theme") {
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
	for i := 0; i < idx; i++ {
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
