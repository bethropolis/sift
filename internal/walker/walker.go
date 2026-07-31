// Package walker handles directory traversal and file processing
package walker

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bethropolis/dir-dumper/internal/ignore"
)

// Walk traverses the directory tree starting from rootDir.
// It returns a list of skipped items and any critical error that occurred.
func Walk(rootDir string, matcher *ignore.IgnoreMatcher, walkFn WalkFunc, opts ...Option) ([]SkippedItem, error) {
	startTime := time.Now()

	// Apply options
	options := defaultOptions()
	for _, opt := range opts {
		opt(&options)
	}

	// Get absolute path for the root directory
	absRootDir, err := filepath.Abs(rootDir)
	if err != nil {
		return []SkippedItem{{Path: rootDir, Reason: ReasonSkippedPathError, IsDir: true}},
			fmt.Errorf("walker: failed to get absolute path for '%s': %w", rootDir, err)
	}

	// Create a tracker for skipped items
	tracker := NewSkippedTracker(100)

	// Create atomic counters for progress tracking
	var stats struct {
		totalFiles     atomic.Int64
		processedFiles atomic.Int64
		skippedFiles   atomic.Int64
		totalDirs      atomic.Int64
		skippedDirs    atomic.Int64
	}

	// Start progress reporting if enabled
	var progressCtx context.Context
	var progressCancel context.CancelFunc

	if options.ProgressFn != nil {
		// Create a separate context for progress updates
		progressCtx, progressCancel = context.WithCancel(context.Background())
		defer progressCancel()

		// Start a goroutine to periodically report progress
		go func() {
			ticker := time.NewTicker(300 * time.Millisecond)
			defer ticker.Stop()

			for {
				select {
				case <-progressCtx.Done():
					return
				case <-ticker.C:
					// Report progress with current statistics
					options.ProgressFn(ProgressStats{
						TotalFiles:     stats.totalFiles.Load(),
						ProcessedFiles: stats.processedFiles.Load(),
						SkippedFiles:   stats.skippedFiles.Load(),
						TotalDirs:      stats.totalDirs.Load(),
						SkippedDirs:    stats.skippedDirs.Load(),
					})
				}
			}
		}()
	}

	options.Logger.Debug("walker.Walk started. Root: %s, Concurrent: %v, Workers: %d",
		absRootDir, options.Concurrent, options.MaxWorkers)

	// Define the core logic for a single entry (used by both sequential and concurrent modes)
	processEntry := func(path string, d fs.DirEntry, err error) (error, bool) {
		// Check context before processing anything
		select {
		case <-options.Context.Done():
			return options.Context.Err(), false
		default:
			// Continue processing
		}

		isDir := d != nil && d.IsDir()

		// Update statistics based on entry type
		if isDir {
			stats.totalDirs.Add(1)
		} else {
			stats.totalFiles.Add(1)
		}

		relativePath, relErr := filepath.Rel(absRootDir, path)
		if relErr != nil {
			options.Logger.Error("Walker Error: Path calculation failed for %q: %v", path, relErr)
			tracker.Track(path, ReasonSkippedPathError, isDir)
			if isDir {
				stats.skippedDirs.Add(1)
			} else {
				stats.skippedFiles.Add(1)
			}
			return nil, false
		}

		options.Logger.Debug("Walker: Evaluating entry: %q (isDir: %v)", relativePath, isDir)

		// Handle walk errors
		if err != nil {
			reason := ReasonSkippedWalkError
			if os.IsPermission(err) {
				reason = ReasonSkippedPermError
			}
			options.Logger.Error("Walker Error: Walk error for %q: %v", relativePath, err)
			tracker.Track(relativePath, reason, isDir)
			if isDir {
				stats.skippedDirs.Add(1)
				if reason == ReasonSkippedPermError {
					return filepath.SkipDir, false
				}
			} else {
				stats.skippedFiles.Add(1)
			}
			return nil, false
		}

		// Skip root itself
		if path == absRootDir || relativePath == "." {
			options.Logger.Debug("Walker: Skipping root entry '.'")
			return nil, false
		}

		// Check ignore status using the matcher
		if matcher != nil && matcher.ShouldIgnore(relativePath, isDir) {
			options.Logger.Debug("Walker: Ignored %q by matcher rules", relativePath)
			tracker.Track(relativePath, ReasonIgnoredRule, isDir)
			if isDir {
				stats.skippedDirs.Add(1)
				return filepath.SkipDir, false
			}
			stats.skippedFiles.Add(1)
			return nil, false
		}

		// Only process files, not directories
		if isDir {
			options.Logger.Debug("Walker: Descending into directory %q", relativePath)
			return nil, false
		}

		// Check extension filtering if enabled
		if options.ExtensionMap != nil && len(options.ExtensionMap) > 0 {
			ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(filepath.ToSlash(relativePath)), "."))
			_, allowed := options.ExtensionMap[ext]
			options.Logger.Debug("Walker: Extension check for %q: ext='%s', allowed=%v",
				relativePath, ext, allowed)
			if !allowed {
				tracker.Track(relativePath, ReasonFilteredExtension, false)
				stats.skippedFiles.Add(1)
				return nil, false
			}
		}

		options.Logger.Debug("Walker: File %q PASSED all checks, will be processed", relativePath)
		return nil, true
	}

	// Choose between concurrent and sequential processing
	if options.Concurrent {
		var wg sync.WaitGroup
		filesChan := make(chan struct{ path, relativePath string }, options.MaxWorkers*2)

		// Start worker goroutines
		options.Logger.Debug("Starting %d workers for concurrent processing.", options.MaxWorkers)
		for i := 0; i < options.MaxWorkers; i++ {
			wg.Add(1)
			go fileProcessorWorker(i+1, filesChan, &wg, options, walkFn, tracker, &stats.processedFiles)
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
	} else {
		// Sequential processing
		options.Logger.Debug("Walker: Starting sequential walk.")
		walkErr := filepath.WalkDir(absRootDir, func(path string, d fs.DirEntry, err error) error {
			processDecisionErr, shouldProcess := processEntry(path, d, err)
			if processDecisionErr != nil {
				return processDecisionErr
			}

			if shouldProcess {
				relativePath, relErr := filepath.Rel(absRootDir, path)
				if relErr != nil {
					options.Logger.Error("Walker Error: Calculating relative path for processing %q: %v", path, relErr)
					tracker.Track(path, ReasonSkippedPathError, false)
					stats.skippedFiles.Add(1)
					return nil
				}

				// Triple check - make sure this isn't the root dir or "."
				if path != absRootDir && relativePath != "." {
					options.Logger.Debug("Walker Processing Sequentially: File [%s]", relativePath)
					processFile(path, relativePath, options, walkFn, tracker, &stats.processedFiles)
				}
			}
			return nil
		})

		duration := time.Since(startTime)
		options.Logger.Debug("Walker: Total walk and processing time: %s", duration)

		return tracker.Items(), walkErr
	}
}
