package state

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// maxRecents caps the shared recent-projects list. The browser sends no
// history of its own; this file is the single source the TUI and `serve`
// both read and write.
const maxRecents = 25

// Recent is one entry in the shared recent-projects list.
type Recent struct {
	Root       string `json:"root"`
	Name       string `json:"name"`
	LastOpened string `json:"last_opened"`
	Branch     string `json:"branch,omitempty"`
}

var recentsMu sync.Mutex

func recentsPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "sift", "recents.json"), nil
}

func recentsLockPath() (string, error) {
	p, err := recentsPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(p), "recents.lock"), nil
}

func loadRecentsLocked() ([]Recent, error) {
	path, err := recentsPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	if len(data) == 0 {
		return nil, nil
	}
	var recents []Recent
	if err := json.Unmarshal(data, &recents); err != nil {
		return nil, err
	}
	return recents, nil
}

func saveRecentsLocked(recents []Recent) error {
	path, err := recentsPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(recents, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".recents-*.json")
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

// mutateRecents runs fn inside the recents file lock plus the in-process
// mutex, loading, applying, capping, and saving atomically.
func mutateRecents(fn func(recents []Recent) []Recent) error {
	lockPath, err := recentsLockPath()
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

	recentsMu.Lock()
	defer recentsMu.Unlock()

	recents, err := loadRecentsLocked()
	if err != nil {
		return err
	}
	recents = fn(recents)
	if len(recents) > maxRecents {
		recents = recents[:maxRecents]
	}
	return saveRecentsLocked(recents)
}

// ListRecents returns the shared recent-projects list, most recent first.
// A corrupt file resets to empty so one bad write never wedges the UI.
func ListRecents() ([]Recent, error) {
	recentsMu.Lock()
	defer recentsMu.Unlock()
	recents, err := loadRecentsLocked()
	if err != nil {
		return nil, nil
	}
	return recents, nil
}

// branchForRoot reads .git/HEAD directly (no git subprocess) to name the
// checked-out branch. Detached or missing HEAD yields "".
func branchForRoot(root string) string {
	data, err := os.ReadFile(filepath.Join(root, ".git", "HEAD"))
	if err != nil {
		return ""
	}
	line := strings.TrimSpace(string(data))
	if after, ok := strings.CutPrefix(line, "ref: refs/heads/"); ok {
		return after
	}
	return ""
}

// RecordRecent moves root to the front of the shared recents list,
// refreshing its branch and timestamp. It never fails the caller: state
// bookkeeping must not break a project open.
func RecordRecent(root string) {
	name := filepath.Base(root)
	if name == "" || name == "." || name == "/" {
		name = root
	}
	_ = mutateRecents(func(recents []Recent) []Recent {
		kept := recents[:0]
		for _, r := range recents {
			if r.Root != root {
				kept = append(kept, r)
			}
		}
		entry := Recent{
			Root:       root,
			Name:       name,
			LastOpened: time.Now().UTC().Format(time.RFC3339),
			Branch:     branchForRoot(root),
		}
		return append([]Recent{entry}, kept...)
	})
}

// RemoveRecent drops root from the shared recents list.
func RemoveRecent(root string) error {
	return mutateRecents(func(recents []Recent) []Recent {
		kept := recents[:0]
		for _, r := range recents {
			if r.Root != root {
				kept = append(kept, r)
			}
		}
		// Keep the file meaningful: nil encodes as null, so normalize.
		if len(kept) == 0 {
			return nil
		}
		out := make([]Recent, len(kept))
		copy(out, kept)
		return out
	})
}
