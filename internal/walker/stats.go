package walker

import (
	"sync/atomic"
	"time"
)

// WalkStats summarizes the work performed by a directory walk.
type WalkStats struct {
	TotalFiles     int64
	ProcessedFiles int64
	SkippedFiles   int64
	TotalDirs      int64
	SkippedDirs    int64
	BytesRead      int64
	BinarySkipped  int64
	SizeSkipped    int64
	SmartSkipped   int64
	Duration       time.Duration
}

// StatsCallback receives aggregate walk metrics after traversal and workers
// have finished.
type StatsCallback func(WalkStats)

// walkStats holds atomic counters used for progress reporting.
type walkStats struct {
	totalFiles     atomic.Int64
	processedFiles atomic.Int64
	skippedFiles   atomic.Int64
	totalDirs      atomic.Int64
	skippedDirs    atomic.Int64
	bytesRead      atomic.Int64
	binarySkipped  atomic.Int64
	sizeSkipped    atomic.Int64
	smartSkipped   atomic.Int64
	currentFile    atomic.Pointer[string]
}

func (s *walkStats) snapshot(duration time.Duration) WalkStats {
	return WalkStats{
		TotalFiles: s.totalFiles.Load(), ProcessedFiles: s.processedFiles.Load(),
		SkippedFiles: s.skippedFiles.Load(), TotalDirs: s.totalDirs.Load(),
		SkippedDirs: s.skippedDirs.Load(), BytesRead: s.bytesRead.Load(),
		BinarySkipped: s.binarySkipped.Load(), SizeSkipped: s.sizeSkipped.Load(),
		SmartSkipped: s.smartSkipped.Load(), Duration: duration,
	}
}
