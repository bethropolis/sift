package walker

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"
	"sync"
	"time"
)

// walkConcurrent walks the tree in the walking goroutine while worker
// goroutines read and process each file.
func walkConcurrent(
	absRootDir string,
	options WalkOptions,
	walkFn WalkFunc,
	tracker *SkippedTracker,
	stats *walkStats,
	processEntry func(path string, d fs.DirEntry, err error) (error, bool),
	startTime time.Time,
) ([]SkippedItem, error) {
	var wg sync.WaitGroup
	filesChan := make(chan struct{ path, relativePath string }, options.MaxWorkers*2)

	// Start worker goroutines
	options.Logger.Debug("Starting %d workers for concurrent processing.", options.MaxWorkers)
	for i := 0; i < options.MaxWorkers; i++ {
		wg.Add(1)
		go fileProcessorWorker(i+1, filesChan, &wg, options, walkFn, tracker, stats)
	}

	// Use a goroutine to walk the directory tree and queue files
	done := make(chan error, 1)
	walkFinished := make(chan struct{})

	go func() {
		walkErr := filepath.WalkDir(absRootDir, func(path string, d fs.DirEntry, err error) error {
			processDecisionErr, shouldProcess := processEntry(path, d, err)
			if processDecisionErr != nil {
				return processDecisionErr
			}

			if shouldProcess {
				relativePath, relErr := filepath.Rel(absRootDir, path)
				if relErr != nil {
					options.Logger.Error("Walker Error: Calculating relative path for queueing %q: %v", path, relErr)
					tracker.Track(path, ReasonSkippedPathError, false)
					stats.skippedFiles.Add(1)
					return nil
				}

				// Triple check - make sure this isn't the root dir or "."
				if path != absRootDir && relativePath != "." {
					// Send to channel with context cancellation support
					select {
					case <-options.Context.Done():
						return options.Context.Err()
					case filesChan <- struct{ path, relativePath string }{path, relativePath}:
						options.Logger.Debug("Walker Queueing: File [%s]", relativePath)
					}
				}
			}
			return nil
		})

		done <- walkErr
		close(walkFinished)
	}()

	// Wait for either context cancellation or walk completion
	select {
	case <-options.Context.Done():
		options.Logger.Debug("Walker: Context cancelled, waiting for walkDir to finish...")
		<-walkFinished // Wait for walkDir to return after it detects cancellation
	case <-walkFinished:
		options.Logger.Debug("Walker: Directory traversal completed")
	}

	// Now close the channel to signal workers to finish
	close(filesChan)

	// Wait for all workers to finish processing
	options.Logger.Debug("Walker: Waiting for workers to complete...")
	wg.Wait()

	// Get the walk error, if any
	var walkErr error
	select {
	case walkErr = <-done:
		// Got the error (or nil)
	default:
		// Should never happen but just in case
		walkErr = fmt.Errorf("walker: internal error - missing walk result")
	}

	if walkErr != nil && walkErr != context.Canceled && walkErr != context.DeadlineExceeded {
		options.Logger.Error("Walker: Error during directory traversal: %v", walkErr)
	}

	duration := time.Since(startTime)
	options.Logger.Debug("Walker: Total walk and processing time: %s", duration)

	return tracker.Items(), walkErr
}

// fileProcessorWorker is the goroutine function for concurrent processing.
func fileProcessorWorker(
	id int,
	filesChan <-chan struct{ path, relativePath string },
	wg *sync.WaitGroup,
	options WalkOptions,
	walkFn WalkFunc,
	tracker *SkippedTracker,
	stats *walkStats,
) {
	defer wg.Done()
	options.Logger.Debug("Worker %d: Started", id)

	for item := range filesChan {
		if err := options.Context.Err(); err != nil {
			options.Logger.Debug("Worker %d: Received cancellation signal", id)
			return
		}
		options.Logger.Debug("Worker %d: Processing file [%s]", id, item.relativePath)
		processFile(item.path, item.relativePath, options, walkFn, tracker, stats)
	}

	options.Logger.Debug("Worker %d: Finished", id)
}
