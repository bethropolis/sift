package theme

import (
	"math"
	"strconv"
	"strings"
	"testing"

	"github.com/bethropolis/sift/internal/highlight"
)

func TestPresetIDsAreUnique(t *testing.T) {
	seen := make(map[string]bool)
	for _, p := range ThemePresets {
		if p.ID == "" || p.Name == "" {
			t.Errorf("preset %+v: id and name are required", p.Name)
		}
		if seen[p.ID] {
			t.Errorf("duplicate preset id %q", p.ID)
		}
		seen[p.ID] = true
	}
}

func TestDefaultIsClassic(t *testing.T) {
	if got := ThemePresets[DefaultIndex()].ID; got != "classic" {
		t.Errorf("default preset = %q, want classic", got)
	}
}

func TestLegacyIDsStillResolve(t *testing.T) {
	legacy := []string{
		"catppuccin-mocha", "tokyo-night", "dracula", "gruvbox-dark", "nord", "rose-pine",
		"classic", "gruvbox-light", "classic-dark", "classic-light", "kanagawa-wave",
		"kanagawa-dragon", "kanagawa-lotus", "everforest-dark", "everforest-light", "one-dark",
		"solarized-dark", "solarized-light", "monokai", "github-dark", "github-light",
		"night-owl", "catppuccin-latte", "tokyo-day", "rose-pine-dawn", "poimandres", "terminal",
	}
	for _, id := range legacy {
		if got := ThemePresets[IndexOf(ThemePresets, id)].ID; got != id {
			t.Errorf("persisted id %q resolves to %q", id, got)
		}
	}
}

func TestCatalogSyntaxCoversEveryToken(t *testing.T) {
	for _, p := range ThemePresets {
		if p.Syntax == nil {
			continue // Classic intentionally uses the legacy palette.
		}
		for kind := highlight.TokenComment; kind <= highlight.TokenDiffHunk; kind++ {
			if p.Syntax[kind].Foreground == "" {
				t.Errorf("%s: token kind %d has no foreground", p.ID, kind)
			}
		}
	}
}

func TestCatalogContrast(t *testing.T) {
	tests := []struct {
		name string
		min  float64
		fg   func(ThemePreset) string
	}{
		{"text", 4.5, func(p ThemePreset) string { return string(p.UI.Text) }},
		{"muted", 3.0, func(p ThemePreset) string { return string(p.UI.TextMuted) }},
		{"accent", 3.0, func(p ThemePreset) string { return string(p.UI.Accent) }},
		{"mode full", 3.0, func(p ThemePreset) string { return string(p.UI.ModeFull) }},
		{"mode sig", 3.0, func(p ThemePreset) string { return string(p.UI.ModeSig) }},
		{"mode skip", 3.0, func(p ThemePreset) string { return string(p.UI.ModeSkip) }},
	}
	for _, p := range ThemePresets {
		bg, ok := parseHex(string(p.UI.Background))
		if !ok {
			continue // ANSI-indexed presets have no fixed background.
		}
		for _, tc := range tests {
			fg, ok := parseHex(tc.fg(p))
			if !ok {
				t.Errorf("%s: %s is not a valid hex color", p.ID, tc.name)
				continue
			}
			if got := contrast(fg, bg); got < tc.min {
				t.Errorf("%s: %s contrast %.2f, want >= %.1f", p.ID, tc.name, got, tc.min)
			}
		}
	}
}

func parseHex(s string) ([3]float64, bool) {
	s = strings.TrimPrefix(s, "#")
	if len(s) != 6 {
		return [3]float64{}, false
	}
	var rgb [3]float64
	for i := range rgb {
		v, err := strconv.ParseUint(s[2*i:2*i+2], 16, 8)
		if err != nil {
			return [3]float64{}, false
		}
		rgb[i] = float64(v) / 255
	}
	return rgb, true
}

// luminance is the WCAG relative luminance of an sRGB color.
func luminance(rgb [3]float64) float64 {
	var lin [3]float64
	for i, c := range rgb {
		if c <= 0.03928 {
			lin[i] = c / 12.92
			continue
		}
		lin[i] = math.Pow((c+0.055)/1.055, 2.4)
	}
	return 0.2126*lin[0] + 0.7152*lin[1] + 0.0722*lin[2]
}

func contrast(a, b [3]float64) float64 {
	la, lb := luminance(a), luminance(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}
