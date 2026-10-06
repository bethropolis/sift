package walker

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"sync"
	"time"
)

// walkAbort propagates the first callback error from worker goroutines back to
// the walking goroutine so traversal can be halted. It stores the first error
// and closes a channel so a blocked producer can be released.
type walkAbort struct {
	mu  sync.Mutex
	err error
	ch  chan struct{}
}

func newWalkAbort() *walkAbort {
	return &walkAbort{ch: make(chan struct{})}
}

// set records the first error only; subsequent calls are ignored.
func (a *walkAbort) set(err error) {
	a.mu.Lock()
	if a.err != nil {
		a.mu.Unlock()
		return
	}
	a.err = err
	a.mu.Unlock()
	close(a.ch)
}

// get returns the recorded error, if any.
func (a *walkAbort) get() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.err
}

// done is closed once the first callback error is recorded. It lets a blocked
// channel send in the walking goroutine abort instead of deadlocking when all
// workers have already exited.
func (a *walkAbort) done() <-chan struct{} { return a.ch }

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
	abort := newWalkAbort()

	// Start worker goroutines
	options.Logger.Debug("Starting %d workers for concurrent processing.", options.MaxWorkers)
	for i := 0; i < options.MaxWorkers; i++ {
		wg.Add(1)
		go fileProcessorWorker(i+1, absRootDir, filesChan, &wg, options, walkFn, tracker, stats, abort)
	}

	// Use a goroutine to walk the directory tree and queue files
	done := make(chan error, 1)
	walkFinished := make(chan struct{})

	go func() {
		walkErr := filepath.WalkDir(absRootDir, func(path string, d fs.DirEntry, err error) error {
			// Stop enqueueing as soon as a worker has signaled a callback error.
			if aerr := abort.get(); aerr != nil {
				return aerr
			}
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
					// Send to channel with context and abort support
					select {
					case <-options.Context.Done():
						return options.Context.Err()
					case <-abort.done():
						return abort.get()
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

	// A callback error raised by a worker after the walking goroutine already
	// finished (e.g. every file was buffered before a worker ran) must still be
	// surfaced. The walking goroutine aborts when it observes the error, but it
	// cannot when there is nothing left to enqueue.
	if aerr := abort.get(); aerr != nil {
		walkErr = aerr
	}

	if walkErr != nil && !errors.Is(walkErr, context.Canceled) && !errors.Is(walkErr, context.DeadlineExceeded) {
		options.Logger.Error("Walker: Error during directory traversal: %v", walkErr)
	}

	duration := time.Since(startTime)
	options.Logger.Debug("Walker: Total walk and processing time: %s", duration)

	return tracker.Items(), walkErr
}

// fileProcessorWorker is the goroutine function for concurrent processing.
func fileProcessorWorker(
	id int,
	root string,
	filesChan <-chan struct{ path, relativePath string },
	wg *sync.WaitGroup,
	options WalkOptions,
	walkFn WalkFunc,
	tracker *SkippedTracker,
	stats *walkStats,
	abort *walkAbort,
) {
	defer wg.Done()
	options.Logger.Debug("Worker %d: Started", id)

	for item := range filesChan {
		if err := options.Context.Err(); err != nil {
			options.Logger.Debug("Worker %d: Received cancellation signal", id)
			return
		}
		options.Logger.Debug("Worker %d: Processing file [%s]", id, item.relativePath)
		if err := processFile(root, item.path, item.relativePath, options, walkFn, tracker, stats); err != nil {
			// The callback asked to stop. Record it so the walk aborts and the
			// error surfaces; the worker exits and stops consuming further work.
			options.Logger.Debug("Worker %d: Callback requested abort on %s", id, item.relativePath)
			abort.set(err)
			return
		}
	}

	options.Logger.Debug("Worker %d: Finished", id)
}
