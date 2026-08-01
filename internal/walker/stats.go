package walker

import "sync/atomic"

// walkStats holds atomic counters used for progress reporting.
type walkStats struct {
	totalFiles     atomic.Int64
	processedFiles atomic.Int64
	skippedFiles   atomic.Int64
	totalDirs      atomic.Int64
	skippedDirs    atomic.Int64
	currentFile    atomic.Pointer[string]
}
