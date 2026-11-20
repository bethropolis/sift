package state

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/bethropolis/sift/internal/fsx"
)

// maxPresetsPerProject bounds a project's saved selections.
const maxPresetsPerProject = 50

// Preset is a named per-project selection plus its output settings.
type Preset struct {
	Name       string            `json:"name"`
	Selections map[string]string `json:"selections"`
	Prompt     string            `json:"prompt,omitempty"`
	Style      string            `json:"style,omitempty"`
	Budget     int               `json:"budget,omitempty"`
	UpdatedAt  time.Time         `json:"updated_at"`
}

// presetStore is the on-disk shape: project key → presets.
type presetStore struct {
	Projects map[string][]Preset `json:"projects"`
}

var presetsMu sync.Mutex

func presetsPath() (string, error) {
	dir, err := fsx.ConfigDir("sift")
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "presets.json"), nil
}

func presetsLockPath() (string, error) {
	p, err := presetsPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(p), "presets.lock"), nil
}

func loadPresetsLocked() (*presetStore, error) {
	path, err := presetsPath()
	if err != nil {
		return nil, err
	}
	store := &presetStore{Projects: map[string][]Preset{}}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return store, nil
		}
		return nil, err
	}
	if len(data) == 0 {
		return store, nil
	}
	if err := json.Unmarshal(data, store); err != nil {
		return nil, err
	}
	if store.Projects == nil {
		store.Projects = map[string][]Preset{}
	}
	return store, nil
}

func savePresetsLocked(store *presetStore) error {
	path, err := presetsPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".presets-*.json")
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

func mutatePresets(fn func(store *presetStore) error) error {
	lockPath, err := presetsLockPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o700); err != nil {
		return err
	}
	lock, err := acquireFileLock(lockPath)
	if err != nil {
		return err
	}
	defer lock.Release()

	presetsMu.Lock()
	defer presetsMu.Unlock()

	store, err := loadPresetsLocked()
	if err != nil {
		return err
	}
	if err := fn(store); err != nil {
		return err
	}
	return savePresetsLocked(store)
}

// ListPresets returns a project's presets ordered by name. A missing file is
// not an error.
func ListPresets(root string) ([]Preset, error) {
	key := GetProjectKey(filepath.Clean(root))
	presetsMu.Lock()
	defer presetsMu.Unlock()
	store, err := loadPresetsLocked()
	if err != nil {
		return nil, err
	}
	out := append([]Preset(nil), store.Projects[key]...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// SavePreset inserts or replaces a named preset and returns the stored copy.
func SavePreset(root string, preset Preset) (Preset, error) {
	preset.Name = strings.TrimSpace(preset.Name)
	if preset.Name == "" {
		return Preset{}, errors.New("preset name is required")
	}
	if len(preset.Name) > 80 {
		return Preset{}, errors.New("preset name is too long")
	}
	if len(preset.Selections) == 0 {
		return Preset{}, errors.New("preset selections are empty")
	}
	preset.Selections = copySelections(preset.Selections)
	preset.UpdatedAt = time.Now().UTC()
	key := GetProjectKey(filepath.Clean(root))
	err := mutatePresets(func(store *presetStore) error {
		list := store.Projects[key]
		replaced := false
		for i := range list {
			if list[i].Name == preset.Name {
				list[i] = preset
				replaced = true
				break
			}
		}
		if !replaced {
			if len(list) >= maxPresetsPerProject {
				return errors.New("too many presets for this project")
			}
			list = append(list, preset)
		}
		sort.SliceStable(list, func(i, j int) bool { return list[i].Name < list[j].Name })
		store.Projects[key] = list
		return nil
	})
	return preset, err
}

// DeletePreset removes a named preset. Unknown names are ignored.
func DeletePreset(root, name string) error {
	key := GetProjectKey(filepath.Clean(root))
	return mutatePresets(func(store *presetStore) error {
		list := store.Projects[key]
		filtered := list[:0]
		for _, p := range list {
			if p.Name != name {
				filtered = append(filtered, p)
			}
		}
		store.Projects[key] = filtered
		return nil
	})
}

func copySelections(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
