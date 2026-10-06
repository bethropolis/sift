// Package walker handles directory traversal and file processing
package walker

import (
	"fmt"
	"os"
)

// processFile handles reading a file and calling the walkFn with its content.
// It returns the error returned by walkFn (when non-nil) so callers can abort
// traversal; a non-nil return signals "stop walking". Stat/read failures are
// reported to walkFn via the err argument and return nil from here (the walk
// continues), but they are counted as skipped so stats stay consistent.
//
// Symlinks are contained: a link whose target escapes root is skipped, never
// read. Links resolving inside the root keep the historical behavior of being
// read as regular files.
func processFile(root, path, relativePath string, options WalkOptions, walkFn WalkFunc, tracker *SkippedTracker, stats *walkStats) error {
	options.Logger.Debug("processFile: Reading [%s]", relativePath)
	stats.currentFile.Store(&relativePath)

	// Always stat the file so non-regular entries (sockets, devices, …) are
	// dropped before they can block a read, and oversized files skip the read
	// when a size cap is configured. Stat follows symlinks, preserving the
	// historical behavior of reading symlinked files that stay inside the
	// root; links escaping the root are dropped below.
	info, err := os.Stat(path)
	if err != nil {
		options.Logger.Error("processFile Error [%s]: Failed to get file info: %v", relativePath, err)
		tracker.Track(relativePath, ReasonSkippedInfoError, false)
		stats.skippedFiles.Add(1)
		walkFn(relativePath, nil, fmt.Errorf("failed to get file info: %w", err))
		return nil
	}

	if !info.Mode().IsRegular() {
		options.Logger.Debug("processFile Skipping [%s]: Not a regular file.", relativePath)
		tracker.Track(relativePath, ReasonSkippedNotRegular, false)
		stats.skippedFiles.Add(1)
		return nil
	}

	// Contain symlinks: resolve once and read the resolved path so the
	// check and the open cannot diverge.
	readPath := path
	if IsSymlink(path) {
		resolved, containErr := ContainPath(root, path)
		if containErr != nil {
			options.Logger.Debug("processFile Skipping [%s]: Symlink escapes root.", relativePath)
			tracker.Track(relativePath, ReasonSkippedSymlinkEscape, false)
			stats.skippedFiles.Add(1)
			return nil
		}
		readPath = resolved
		// The target may differ in size from the link; re-stat the resolved
		// path so size and regularity describe what will be read.
		if targetInfo, statErr := os.Stat(resolved); statErr != nil || !targetInfo.Mode().IsRegular() {
			tracker.Track(relativePath, ReasonSkippedNotRegular, false)
			stats.skippedFiles.Add(1)
			return nil
		} else {
			info = targetInfo
		}
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
		return nil
	}

	// Skip known-binary extensions before reading (zero I/O), then sniff the
	// bytes once read for unknown/extensionless files. Sniffing content
	// avoids the second open DetectFile would cost after a read.
	if !options.IncludeBinary && IsBinaryExt(path) {
		options.Logger.Debug("processFile Skipping [%s]: Binary extension detected", relativePath)
		tracker.Track(relativePath, ReasonSkippedBinary, false)
		stats.binarySkipped.Add(1)
		stats.skippedFiles.Add(1)
		return nil
	}

	// Read file content
	content, err := os.ReadFile(readPath)
	if err != nil {
		options.Logger.Error("processFile Error [%s]: Failed to read file: %v", relativePath, err)
		tracker.Track(relativePath, ReasonSkippedReadError, false)
		stats.skippedFiles.Add(1)
		walkFn(relativePath, nil, fmt.Errorf("failed to read file: %w", err))
		return nil
	}
	stats.bytesRead.Add(int64(len(content)))

	if !options.IncludeBinary && !IsTextExt(path) && IsBinaryContent(content) {
		options.Logger.Debug("processFile Skipping [%s]: Binary content detected", relativePath)
		tracker.Track(relativePath, ReasonSkippedBinary, false)
		stats.binarySkipped.Add(1)
		stats.skippedFiles.Add(1)
		return nil
	}

	// Call the walk function with the content
	options.Logger.Debug("processFile Success [%s]: Read %d bytes. Calling walkFn.", relativePath, len(content))
	if err := walkFn(relativePath, content, nil); err != nil {
		options.Logger.Error("processFile Error [%s]: Callback function returned error: %v", relativePath, err)
		return err
	}

	stats.processedFiles.Add(1)
	return nil
}
