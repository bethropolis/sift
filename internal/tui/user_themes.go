package tui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/pelletier/go-toml/v2"
)

type userThemeFile struct {
	Themes []userThemeSpec `toml:"theme"`
}

type userThemeSpec struct {
	ID          string `toml:"id"`
	Name        string `toml:"name"`
	Family      string `toml:"family"`
	Variant     string `toml:"variant"`
	Description string `toml:"description"`
	Dark        *bool  `toml:"dark"`
	Extends     string `toml:"extends"`
	Border      string `toml:"border"`
	Title       string `toml:"title"`
	Muted       string `toml:"muted"`
	CursorBg    string `toml:"cursor_bg"`
	CursorFg    string `toml:"cursor_fg"`
	Selected    string `toml:"selected"`
	Notice      string `toml:"notice"`
	ModeFull    string `toml:"mode_full"`
	ModeSig     string `toml:"mode_sig"`
	ModeSkip    string `toml:"mode_skip"`
}

// LoadUserThemes loads and validates custom picker themes from a TOML file.
// Inheriting themes are resolved against built-ins and earlier definitions.
func LoadUserThemes(path string) ([]ThemePreset, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read UI theme file: %w", err)
	}
	var file userThemeFile
	if err := toml.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("parse UI theme file: %w", err)
	}
	if len(file.Themes) == 0 {
		return nil, fmt.Errorf("UI theme file contains no [[theme]] entries")
	}
	result := make([]ThemePreset, 0, len(file.Themes))
	byID := make(map[string]ThemePreset)
	for _, base := range ThemePresets {
		byID[base.ID] = base
	}
	for i, spec := range file.Themes {
		preset, err := resolveUserTheme(spec, byID)
		if err != nil {
			return nil, fmt.Errorf("theme %d: %w", i+1, err)
		}
		if preset.ID == "" || preset.Name == "" {
			return nil, fmt.Errorf("theme %d: id and name are required", i+1)
		}
		if _, exists := byID[preset.ID]; exists {
			return nil, fmt.Errorf("theme %q duplicates an existing ID", preset.ID)
		}
		byID[preset.ID] = preset
		result = append(result, preset)
	}
	return result, nil
}

func resolveUserTheme(spec userThemeSpec, byID map[string]ThemePreset) (ThemePreset, error) {
	var base ThemePreset
	if spec.Extends != "" {
		var ok bool
		base, ok = byID[spec.Extends]
		if !ok {
			for _, candidate := range ThemePresets {
				if candidate.ID == spec.Extends || strings.EqualFold(candidate.Name, spec.Extends) {
					base, ok = candidate, true
					break
				}
			}
		}
		if !ok {
			return ThemePreset{}, fmt.Errorf("extends unknown theme %q", spec.Extends)
		}
	}
	preset := base
	setString(&preset.ID, spec.ID)
	setString(&preset.Name, spec.Name)
	setString(&preset.Family, spec.Family)
	setString(&preset.Variant, spec.Variant)
	setString(&preset.Description, spec.Description)
	if spec.Dark != nil {
		preset.Dark = *spec.Dark
	}
	setColor(&preset.Border, spec.Border)
	setColor(&preset.Title, spec.Title)
	setColor(&preset.Muted, spec.Muted)
	setColor(&preset.CursorBg, spec.CursorBg)
	setColor(&preset.CursorFg, spec.CursorFg)
	setColor(&preset.Selected, spec.Selected)
	setColor(&preset.Notice, spec.Notice)
	setColor(&preset.ModeFull, spec.ModeFull)
	setColor(&preset.ModeSig, spec.ModeSig)
	setColor(&preset.ModeSkip, spec.ModeSkip)
	if spec.Border != "" || spec.Title != "" || spec.Muted != "" {
		preset.Highlight = hlPalette(string(preset.Title), string(preset.ModeFull), string(preset.ModeSig), string(preset.Muted), string(preset.CursorFg))
		preset.Syntax = semanticPalette(string(preset.Title), string(preset.ModeFull), string(preset.ModeSig), string(preset.Muted), string(preset.CursorFg), string(preset.Border), string(preset.Selected), string(preset.Title), string(preset.Muted))
	}
	return normalizeTheme(preset), nil
}

func setString(dst *string, value string) {
	if value != "" {
		*dst = value
	}
}

func setColor(dst *lipgloss.Color, value string) {
	if value != "" {
		*dst = lipgloss.Color(value)
	}
}
