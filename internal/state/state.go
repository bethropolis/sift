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
