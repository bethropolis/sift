package state

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"

	"github.com/bethropolis/sift/internal/fsx"
)

// prefsMu serializes in-process read-modify-write access; acquireFileLock
// handles concurrent processes.
var prefsMu sync.Mutex

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
	// ShowHidden reveals dotfiles in the serve folder browser. Default hides them.
	ShowHidden bool `json:"show_hidden,omitempty"`
	// FileSort orders the serve folder browser: "name" (default) or "updated".
	FileSort string `json:"file_sort,omitempty"`
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

// EffectiveTheme resolves the single shared theme id: Theme wins, with the
// legacy UITheme as fallback for files written before unification. Empty
// means the default theme; "system" means follow the OS (web only, the TUI
// falls back to its default).
func EffectiveTheme(p Preferences) string {
	if p.Theme != "" {
		return p.Theme
	}
	return p.UITheme
}

// SetTheme records id as the shared theme for both the TUI and serve,
// preserving every other preference. It replaces the old pattern of saving
// Preferences{UITheme: name}, which silently wiped the rest of the file.
func SetTheme(id string) error {
	return UpdatePreferences(func(prefs *Preferences) error {
		prefs.Theme = id
		prefs.UITheme = id
		return nil
	})
}
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
	switch p.FileSort {
	case "", "name", "updated":
	default:
		p.FileSort = ""
	}
	return p
}

func preferencesPath() (string, error) {
	dir, err := fsx.ConfigDir("sift")
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "preferences.json"), nil
}

// LoadPreferences loads user preferences. Missing preferences are normal and
// return the zero-value preference set.
func LoadPreferences() (Preferences, error) {
	path, err := preferencesPath()
	if err != nil {
		return Preferences{}, err
	}
	return loadPreferencesFile(path)
}

// SavePreferences writes user preferences atomically, sanitizing first.
func SavePreferences(prefs Preferences) error {
	path, err := preferencesPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	lock, err := acquireFileLock(preferencesLockPath(path))
	if err != nil {
		return err
	}
	defer lock.Release()

	prefsMu.Lock()
	defer prefsMu.Unlock()
	return savePreferencesLocked(path, prefs)
}

// UpdatePreferences runs fn against the current preferences inside the same
// cross-process lock and in-process mutex as SavePreferences, then writes the
// result. Read-modify-write callers (SetTheme, the serve settings handler)
// must use this so concurrent updates cannot clobber each other.
func UpdatePreferences(fn func(*Preferences) error) error {
	path, err := preferencesPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	lock, err := acquireFileLock(preferencesLockPath(path))
	if err != nil {
		return err
	}
	defer lock.Release()

	prefsMu.Lock()
	defer prefsMu.Unlock()

	prefs, err := loadPreferencesFile(path)
	if err != nil {
		return err
	}
	if err := fn(&prefs); err != nil {
		return err
	}
	return savePreferencesLocked(path, prefs)
}

func preferencesLockPath(path string) string {
	return filepath.Join(filepath.Dir(path), "preferences.lock")
}

// loadPreferencesFile reads one path. Callers hold the preference lock when
// performing a read-modify-write.
func loadPreferencesFile(path string) (Preferences, error) {
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

// savePreferencesLocked writes prefs atomically. Callers hold the lock.
func savePreferencesLocked(path string, prefs Preferences) error {
	prefs = SanitizePreferences(prefs)
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
