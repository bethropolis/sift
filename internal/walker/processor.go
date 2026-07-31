// Package walker handles directory traversal and file processing
package walker

import (
	"fmt"
	"os"
	"sync"
	"sync/atomic"
)

// processFile handles reading a file and calling the walkFn with its content
func processFile(path, relativePath string, options WalkOptions, walkFn WalkFunc, tracker *SkippedTracker, processed *atomic.Int64) {
	options.Logger.Debug("processFile: Reading [%s]", relativePath)

	// Update progress info with current file if progress reporting is enabled
	if options.ProgressFn != nil {
		options.ProgressFn(ProgressStats{
			CurrentFilePath: relativePath,
		})
	}

	// Only perform file stats if we have a size limit configured
	if options.MaxFileSize > 0 {
		info, err := os.Lstat(path)
		if err != nil {
			options.Logger.Error("processFile Error [%s]: Failed to get file info: %v", relativePath, err)
			tracker.Track(relativePath, ReasonSkippedInfoError, false)
			walkFn(relativePath, nil, fmt.Errorf("failed to get file info: %w", err))
			return
		}

		if !info.Mode().IsRegular() {
			options.Logger.Debug("processFile Skipping [%s]: Not a regular file.", relativePath)
			tracker.Track(relativePath, ReasonSkippedNotRegular, false)
			return
		}

		if info.Size() > options.MaxFileSize {
			options.Logger.Debug("processFile Skipping [%s]: Exceeds size limit (%d > %d bytes)",
				relativePath, info.Size(), options.MaxFileSize)
			tracker.Track(relativePath, ReasonSkippedSizeLimit, false)
			walkFn(relativePath, nil, fmt.Errorf("file size %d exceeds limit %d bytes", info.Size(), options.MaxFileSize))
			return
		}
	}

	// Read file content
	content, err := os.ReadFile(path)
	if err != nil {
		options.Logger.Error("processFile Error [%s]: Failed to read file: %v", relativePath, err)
		tracker.Track(relativePath, ReasonSkippedReadError, false)
		walkFn(relativePath, nil, fmt.Errorf("failed to read file: %w", err))
		return
	}

	// Call the walk function with the content
	options.Logger.Debug("processFile Success [%s]: Read %d bytes. Calling walkFn.", relativePath, len(content))
	if err := walkFn(relativePath, content, nil); err != nil {
		options.Logger.Error("processFile Error [%s]: Callback function returned error: %v", relativePath, err)
	}

	if processed != nil {
		processed.Add(1)
	}
}

// fileProcessorWorker is the goroutine function for concurrent processing.
func fileProcessorWorker(
	id int,
	filesChan <-chan struct{ path, relativePath string },
	wg *sync.WaitGroup,
	options WalkOptions,
	walkFn WalkFunc,
	tracker *SkippedTracker,
	processed *atomic.Int64,
) {
	defer wg.Done()
	options.Logger.Debug("Worker %d: Started", id)

	for item := range filesChan {
		select {
		case <-options.Context.Done():
			options.Logger.Debug("Worker %d: Received cancellation signal", id)
			return
		default:
			options.Logger.Debug("Worker %d: Processing file [%s]", id, item.relativePath)
			processFile(item.path, item.relativePath, options, walkFn, tracker, processed)
		}
	}

	options.Logger.Debug("Worker %d: Finished", id)
}
