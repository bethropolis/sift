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

// NewChanges returns an empty Changes with pre-normalized lookup sets.
func NewChanges() *Changes {
	return &Changes{
		Modified:  map[string]bool{},
		Diffed:    map[string]bool{},
		Committed: map[string]bool{},
	}
}

// Normalize cleans all tier sets once after bulk insertion, so Score lookups
// are O(depth) map probes instead of O(len(set)) scans with per-key cleans.
func (c *Changes) Normalize() {
	c.Modified = NormalizeSet(c.Modified)
	c.Diffed = NormalizeSet(c.Diffed)
	c.Committed = NormalizeSet(c.Committed)
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
// directory. keys must be pre-normalized with NormalizeSet.
func matches(set map[string]bool, path string) bool {
	path = filepath.ToSlash(filepath.Clean(path))
	if set[path] {
		return true
	}
	// Directory-prefix match: walk up the parents instead of scanning every
	// key, so lookup is O(depth) rather than O(len(set)).
	for {
		idx := strings.LastIndex(path, "/")
		if idx < 0 {
			return false
		}
		path = path[:idx]
		if set[path] {
			return true
		}
	}
}

// NormalizeSet cleans a git-reported path list into a lookup set once, so
// per-file Score calls never re-clean every key.
func NormalizeSet(paths map[string]bool) map[string]bool {
	out := make(map[string]bool, len(paths))
	for p := range paths {
		out[filepath.ToSlash(filepath.Clean(p))] = true
	}
	return out
}
