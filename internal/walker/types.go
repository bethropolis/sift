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
	// ReasonSkippedSymlinkEscape marks a symlink whose target resolves
	// outside the scan root. The link is never read.
	ReasonSkippedSymlinkEscape SkippedReason = "Skipped (Symlink Escapes Root)"
)

// SkippedItem holds information about a skipped path.
type SkippedItem struct {
	Path   string        `json:"path"`
	Reason SkippedReason `json:"reason"`
	IsDir  bool          `json:"is_dir"`
}

// defaultMaxSkippedItems caps the number of skipped paths a tracker retains.
// A heavily filtered tree (every node_modules file, extension-filtered file,
// etc.) can otherwise grow the skip list without bound and balloon memory.
const defaultMaxSkippedItems = 10000

// SkippedTracker is a struct to track skipped items
type SkippedTracker struct {
	items    []SkippedItem
	mutex    sync.Mutex
	maxItems int // retention cap; additional skips are counted as dropped
	dropped  int // skips not recorded because the cap was reached
}

// NewSkippedTracker creates a new SkippedTracker
func NewSkippedTracker(capacity int) *SkippedTracker {
	st := &SkippedTracker{
		items:    make([]SkippedItem, 0, capacity),
		maxItems: defaultMaxSkippedItems,
	}
	// A caller that asks for more than the default cap gets what it asked for.
	if capacity > st.maxItems {
		st.maxItems = capacity
	}
	return st
}

// Track adds a skipped item to the tracker, up to the retention cap. Beyond
// that the item is dropped and counted so callers can detect overflow.
func (st *SkippedTracker) Track(path string, reason SkippedReason, isDir bool) {
	st.mutex.Lock()
	defer st.mutex.Unlock()
	if len(st.items) >= st.maxItems {
		st.dropped++
		return
	}
	st.items = append(st.items, SkippedItem{Path: path, Reason: reason, IsDir: isDir})
}

// Items returns a copy of the tracked skipped items.
func (st *SkippedTracker) Items() []SkippedItem {
	st.mutex.Lock()
	defer st.mutex.Unlock()
	return append([]SkippedItem(nil), st.items...)
}

// Dropped reports how many skip events were not recorded because the retention
// cap was reached, so callers can know the skip list was truncated.
func (st *SkippedTracker) Dropped() int {
	st.mutex.Lock()
	defer st.mutex.Unlock()
	return st.dropped
}
