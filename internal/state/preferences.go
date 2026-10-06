package state

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// Preferences contains user-level interactive preferences that should survive
// picker sessions but do not belong to a project's dump history. The TUI and
// `sift serve` share this file: theme ids and defaults set in either surface
// take effect in both.
type Preferences struct {
	UITheme string `json:"ui_theme,omitempty"`
	// Theme is the web UI theme id ([a-z0-9-]{1,32}); "system" follows the OS.
	Theme string `json:"theme,omitempty"`
	// DefaultStyle is the default output style (xml, markdown, plain).
	DefaultStyle string `json:"default_style,omitempty"`
	// DefaultBudget is the default token budget for fresh sessions.
	DefaultBudget int `json:"default_budget,omitempty"`
}

// validThemeID reports whether id is a safe theme identifier.
func validThemeID(id string) bool {
	if id == "system" {
		return true
	}
	if len(id) < 1 || len(id) > 32 {
		return false
	}
	for _, r := range id {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' {
			continue
		}
		return false
	}
	return true
}

// SanitizePreferences drops invalid values so a corrupt or hostile file can
// never inject an unexpected theme id or negative budget.
func SanitizePreferences(p Preferences) Preferences {
	if !validThemeID(p.Theme) {
		p.Theme = ""
	}
	switch p.DefaultStyle {
	case "", "xml", "markdown", "plain", "text", "json":
	default:
		p.DefaultStyle = ""
	}
	if p.DefaultBudget < 0 {
		p.DefaultBudget = 0
	}
	return p
}

func preferencesPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "sift", "preferences.json"), nil
}

// LoadPreferences loads user preferences. Missing preferences are normal and
// return the zero-value preference set.
func LoadPreferences() (Preferences, error) {
	path, err := preferencesPath()
	if err != nil {
		return Preferences{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Preferences{}, nil
		}
		return Preferences{}, err
	}
	var prefs Preferences
	if len(data) == 0 {
		return prefs, nil
	}
	if err := json.Unmarshal(data, &prefs); err != nil {
		return Preferences{}, err
	}
	return SanitizePreferences(prefs), nil
}

// SavePreferences writes user preferences atomically, sanitizing first.
func SavePreferences(prefs Preferences) error {
	prefs = SanitizePreferences(prefs)
	path, err := preferencesPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(prefs, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".preferences-*.json")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
