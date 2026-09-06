// Package state persists per-project dump records in the user's application
// data directory so delta dumps can know the last commit a project was dumped
// from, without polluting the repository itself.
package state

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// DumpRecord describes the most recent dump of a project.
type DumpRecord struct {
	LastCommitHash string    `json:"last_commit_hash"`
	LastCommitMsg  string    `json:"last_commit_msg"`
	DumpTimestamp  time.Time `json:"dump_timestamp"`
	FilesDumped    int       `json:"files_dumped"`
	TokensDumped   int       `json:"tokens_dumped"`
}

// AppState holds the dump record of every project that has been dumped,
// keyed by GetProjectKey.
type AppState struct {
	Projects map[string]DumpRecord `json:"projects"`
}

// mu guards concurrent access to AppState in-process (e.g. watch mode
// re-renders from multiple goroutines).
var mu sync.Mutex

// GetProjectKey returns a stable 16-hex-char key for an absolute project path.
func GetProjectKey(absPath string) string {
	hash := sha256.Sum256([]byte(filepath.Clean(absPath)))
	return hex.EncodeToString(hash[:8])
}

// CanonicalRoot resolves rootDir to the absolute, cleaned path every
// state-key site must use. Writers (record) and readers (delta, diff,
// picker) share this helper so a dump recorded as "." is found again as
// ".", an absolute path, or any equivalent spelling.
func CanonicalRoot(rootDir string) (string, error) {
	abs, err := filepath.Abs(rootDir)
	if err != nil {
		return "", err
	}
	return abs, nil
}

// StatePath returns the JSON state file location in the user's config
// directory: ~/.config/sift/state.json on Linux, %APPDATA% on Windows.
func StatePath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "sift", "state.json"), nil
}

// Load reads the state file, returning an empty state when it does not exist.
func Load() (*AppState, error) {
	mu.Lock()
	defer mu.Unlock()
	return loadLocked()
}

// loadLocked is Load without the in-process mutex, for use inside MutateState
// which already holds it across the whole transaction.
func loadLocked() (*AppState, error) {
	path, err := StatePath()
	if err != nil {
		return nil, err
	}
	st := &AppState{Projects: map[string]DumpRecord{}}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return st, nil
		}
		return nil, err
	}
	if len(data) == 0 {
		return st, nil
	}
	if err := json.Unmarshal(data, st); err != nil {
		return nil, fmt.Errorf("state: parse %s: %w", path, err)
	}
	if st.Projects == nil {
		st.Projects = map[string]DumpRecord{}
	}
	return st, nil
}

// Save writes the state to disk atomically so a crash never truncates it.
func (s *AppState) Save() error {
	mu.Lock()
	defer mu.Unlock()
	return s.saveLocked()
}

// saveLocked is Save without the in-process mutex, for use inside MutateState
// which already holds it across the whole transaction.
func (s *AppState) saveLocked() error {
	path, err := StatePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), ".state-*.json")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

// LockPath returns the advisory lock file alongside state.json. It must be
// held across every read-modify-write so concurrent processes (e.g. a watch
// session and a manual dump) cannot clobber each other's records.
func LockPath() (string, error) {
	path, err := StatePath()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(path), "state.lock"), nil
}

// MutateState runs fn inside a cross-process file lock plus the in-process
// mutex, loading the state, applying fn, and saving atomically. All state
// mutations must go through here; Load/Save remain for lock-free reads and
// single-shot writes.
func MutateState(fn func(st *AppState) error) error {
	lockPath, err := LockPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		return err
	}
	lock, err := acquireFileLock(lockPath)
	if err != nil {
		return err
	}
	defer lock.Release()

	mu.Lock()
	defer mu.Unlock()

	st, err := loadLocked()
	if err != nil {
		return err
	}
	if err := fn(st); err != nil {
		return err
	}
	return st.saveLocked()
}

// Get returns the dump record for a project key, if present.
func (s *AppState) Get(key string) (DumpRecord, bool) {
	rec, ok := s.Projects[key]
	return rec, ok
}

// Record stores a new dump record for key.
func (s *AppState) Record(key, commitHash, commitMsg string, files, tokens int) {
	s.Projects[key] = DumpRecord{
		LastCommitHash: commitHash,
		LastCommitMsg:  commitMsg,
		DumpTimestamp:  time.Now(),
		FilesDumped:    files,
		TokensDumped:   tokens,
	}
}
