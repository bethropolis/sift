package rank

import (
	"path/filepath"
	"strings"
)

const (
	// ScoreBaseline is the score for files with no git signal.
	ScoreBaseline = 0.1
	// ScoreModified is the score for files with uncommitted changes.
	ScoreModified = 1.0
	// ScoreDiffed is the score for files touched by the recent diff.
	ScoreDiffed = 0.8
	// ScoreCommitted is the score for files in the latest commit.
	ScoreCommitted = 0.5
)

// Changes holds the file sets for each git relevance tier, keyed by
// slash-separated paths relative to the repository root.
type Changes struct {
	Modified  map[string]bool
	Diffed    map[string]bool
	Committed map[string]bool
}

// NewChanges returns an empty Changes.
func NewChanges() *Changes {
	return &Changes{
		Modified:  map[string]bool{},
		Diffed:    map[string]bool{},
		Committed: map[string]bool{},
	}
}

// Score returns the highest tier score matching path.
func (c *Changes) Score(path string) float64 {
	if matches(c.Modified, path) {
		return ScoreModified
	}
	if matches(c.Diffed, path) {
		return ScoreDiffed
	}
	if matches(c.Committed, path) {
		return ScoreCommitted
	}
	return ScoreBaseline
}

// matches reports whether path equals one of the keys or lives under a keyed
// directory.
func matches(set map[string]bool, path string) bool {
	path = filepath.ToSlash(filepath.Clean(path))
	for p := range set {
		p = filepath.ToSlash(filepath.Clean(p))
		if p == path || strings.HasPrefix(path, p+"/") {
			return true
		}
	}
	return false
}
