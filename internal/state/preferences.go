package state

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// Preferences contains user-level interactive preferences that should survive
// picker sessions but do not belong to a project's dump history.
type Preferences struct {
	UITheme string `json:"ui_theme,omitempty"`
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
	return prefs, nil
}

// SavePreferences writes user preferences atomically.
func SavePreferences(prefs Preferences) error {
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
