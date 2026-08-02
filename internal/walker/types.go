// Package walker handles directory traversal and file processing
package walker

import (
	"sync"
)

// WalkFunc is the callback function type used by Walk
type WalkFunc func(relativePath string, content []byte, err error) error

// SkippedReason clarifies why a file/directory was not processed.
type SkippedReason string

const (
	ReasonIgnoredRule       SkippedReason = "Ignored (Gitignore/Custom Rule)"
	ReasonFilteredExtension SkippedReason = "Filtered (Extension Mismatch)"
	ReasonFilteredPath      SkippedReason = "Filtered (Path Rule)"
	ReasonSkippedSizeLimit  SkippedReason = "Skipped (Size Limit Exceeded)"
	ReasonSkippedNotRegular SkippedReason = "Skipped (Not a Regular File)"
	ReasonSkippedPermError  SkippedReason = "Skipped (Permission Error)"
	ReasonSkippedWalkError  SkippedReason = "Skipped (Walk Error)"
	ReasonSkippedReadError  SkippedReason = "Skipped (Read Error)"
	ReasonSkippedBinary     SkippedReason = "Skipped (Binary File)"
	ReasonSkippedInfoError  SkippedReason = "Skipped (File Info Error)"
	ReasonSkippedPathError  SkippedReason = "Skipped (Path Calculation Error)"
	ReasonSkippedSmart      SkippedReason = "Skipped (Smart Filter)"
)

// SkippedItem holds information about a skipped path.
type SkippedItem struct {
	Path   string        `json:"path"`
	Reason SkippedReason `json:"reason"`
	IsDir  bool          `json:"is_dir"`
}

// SkippedTracker is a struct to track skipped items
type SkippedTracker struct {
	items []SkippedItem
	mutex sync.Mutex
}

// NewSkippedTracker creates a new SkippedTracker
func NewSkippedTracker(capacity int) *SkippedTracker {
	return &SkippedTracker{
		items: make([]SkippedItem, 0, capacity),
	}
}

// Track adds a skipped item to the tracker
func (st *SkippedTracker) Track(path string, reason SkippedReason, isDir bool) {
	st.mutex.Lock()
	defer st.mutex.Unlock()
	st.items = append(st.items, SkippedItem{Path: path, Reason: reason, IsDir: isDir})
}

// Items returns the tracked skipped items
func (st *SkippedTracker) Items() []SkippedItem {
	st.mutex.Lock()
	defer st.mutex.Unlock()
	return st.items
}
