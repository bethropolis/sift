// Package walker handles directory traversal and file processing
package walker

import (
	"fmt"
	"os"
)

// processFile handles reading a file and calling the walkFn with its content
func processFile(path, relativePath string, options WalkOptions, walkFn WalkFunc, tracker *SkippedTracker, stats *walkStats) {
	options.Logger.Debug("processFile: Reading [%s]", relativePath)
	stats.currentFile.Store(&relativePath)

	// Always stat the file so non-regular entries (sockets, devices, …) are
	// dropped before they can block a read, and oversized files skip the read
	// when a size cap is configured. Stat follows symlinks, preserving the
	// historical behavior of reading symlinked files.
	info, err := os.Stat(path)
	if err != nil {
		options.Logger.Error("processFile Error [%s]: Failed to get file info: %v", relativePath, err)
		tracker.Track(relativePath, ReasonSkippedInfoError, false)
		walkFn(relativePath, nil, fmt.Errorf("failed to get file info: %w", err))
		return
	}

	if !info.Mode().IsRegular() {
		options.Logger.Debug("processFile Skipping [%s]: Not a regular file.", relativePath)
		tracker.Track(relativePath, ReasonSkippedNotRegular, false)
		stats.skippedFiles.Add(1)
		return
	}

	if options.MaxFileSize > 0 && info.Size() > options.MaxFileSize {
		// Expected policy skip: recorded in the skipped items, never surfaced
		// as a callback error so the app does not log it as a processing
		// warning. Binary skips below use the same pattern.
		options.Logger.Debug("processFile Skipping [%s]: Exceeds size limit (%d > %d bytes)",
			relativePath, info.Size(), options.MaxFileSize)
		tracker.Track(relativePath, ReasonSkippedSizeLimit, false)
		stats.sizeSkipped.Add(1)
		stats.skippedFiles.Add(1)
		return
	}

	// Skip binary files before reading them, using extension fast-paths and
	// magic-number sniffing as a fallback for unknown/extensionless files.
	if !options.IncludeBinary && IsBinaryFile(path) {
		options.Logger.Debug("processFile Skipping [%s]: Binary file detected", relativePath)
		tracker.Track(relativePath, ReasonSkippedBinary, false)
		stats.binarySkipped.Add(1)
		stats.skippedFiles.Add(1)
		return
	}

	// Read file content
	content, err := os.ReadFile(path)
	if err != nil {
		options.Logger.Error("processFile Error [%s]: Failed to read file: %v", relativePath, err)
		tracker.Track(relativePath, ReasonSkippedReadError, false)
		walkFn(relativePath, nil, fmt.Errorf("failed to read file: %w", err))
		return
	}
	stats.bytesRead.Add(int64(len(content)))

	// Call the walk function with the content
	options.Logger.Debug("processFile Success [%s]: Read %d bytes. Calling walkFn.", relativePath, len(content))
	if err := walkFn(relativePath, content, nil); err != nil {
		options.Logger.Error("processFile Error [%s]: Callback function returned error: %v", relativePath, err)
	}

	stats.processedFiles.Add(1)
}
