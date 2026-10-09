package theme

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestThemeNormalizationCompletesSemanticRoles(t *testing.T) {
	for _, preset := range ThemePresets {
		normalized := Normalize(preset)
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
		if IndexOf(ThemePresets, preset.ID) != i {
			t.Errorf("theme ID %q did not resolve to index %d", preset.ID, i)
		}
	}
	for _, legacy := range []string{"Catppuccin Mocha", "Tokyo Night", "Rose Pine", "Classic (Default)"} {
		if got := IndexOf(ThemePresets, legacy); got == DefaultIndex() && legacy != "Classic (Default)" {
			t.Errorf("legacy theme %q fell back to Classic", legacy)
		}
	}
}

func TestDefaultThemeIsClassic(t *testing.T) {
	if idx := DefaultIndex(); idx < 0 || idx >= len(ThemePresets) {
		t.Fatalf("defaultThemeIndex = %d out of range", idx)
	}
	if p := ThemePresets[DefaultIndex()]; p.Name != "Classic (Default)" {
		t.Errorf("default theme = %q, want Classic (Default)", p.Name)
	}
}

func TestWebThemeIDsResolveInTUI(t *testing.T) {
	webIDs := []string{
		"catppuccin-mocha", "tokyo-night", "nord", "gruvbox-dark", "gruvbox-light",
		"dracula", "rose-pine", "one-dark", "github-dark", "github-light",
		"monokai", "classic-dark", "classic-light", "everforest-dark",
		"kanagawa-wave", "night-owl", "catppuccin-latte", "tokyo-day",
		"rose-pine-dawn",
	}
	for _, id := range webIDs {
		idx := IndexOf(ThemePresets, id)
		if ThemePresets[idx].ID != id {
			t.Errorf("web theme %q resolves to TUI preset %q", id, ThemePresets[idx].ID)
		}
	}
}
