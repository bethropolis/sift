// Package walker handles directory traversal and file processing
package walker

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/bethropolis/sift/internal/ignore"
)

// heavyDirBasenames are directory basenames to prune immediately at entry
// when the ignore rules also cover them, so we never descend into them.
var heavyDirBasenames = map[string]bool{
	"node_modules":     true,
	"target":           true,
	".next":            true,
	".nuxt":            true,
	"vendor":           true,
	"__pycache__":      true,
	".gradle":          true,
	"dist":             true,
	"build":            true,
	"out":              true,
	".venv":            true,
	"venv":             true,
	"coverage":         true,
	".turbo":           true,
	".cache":           true,
	"Pods":             true,
	"DerivedData":      true,
	"bower_components": true,
	".pnpm-store":      true,
	"site-packages":    true,
	".tox":             true,
	".nox":             true,
	".pytest_cache":    true,
	".eggs":            true,
	".mypy_cache":      true,
}

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
	stats := &walkStats{}

	// Start progress reporting if enabled
	var progressCtx context.Context
	var progressCancel context.CancelFunc

	if options.ProgressFn != nil {
		// Create a separate context for progress updates
		progressCtx, progressCancel = context.WithCancel(context.Background())
		defer progressCancel()

		// Start a goroutine to periodically report progress.
		// It is the only caller of ProgressFn, so progress output never races.
		var printed atomic.Bool
		go func() {
			ticker := time.NewTicker(300 * time.Millisecond)
			defer ticker.Stop()

			for {
				select {
				case <-progressCtx.Done():
					// Clear the progress line so it does not linger after the walk.
					if printed.Load() {
						fmt.Fprintln(os.Stderr)
					}
					return
				case <-ticker.C:
					printed.Store(true)
					current := ""
					if cp := stats.currentFile.Load(); cp != nil {
						current = *cp
					}
					options.ProgressFn(ProgressStats{
						CurrentFilePath: current,
						TotalFiles:      stats.totalFiles.Load(),
						ProcessedFiles:  stats.processedFiles.Load(),
						SkippedFiles:    stats.skippedFiles.Load(),
						TotalDirs:       stats.totalDirs.Load(),
						SkippedDirs:     stats.skippedDirs.Load(),
					})
				}
			}
		}()
	}

	options.Logger.Debug("walker.Walk started. Root: %s, Concurrent: %v, Workers: %d",
		absRootDir, options.Concurrent, options.MaxWorkers)

	processEntry := newProcessEntry(absRootDir, options, matcher, tracker, stats)

	// Choose between concurrent and sequential processing
	if options.Concurrent {
		return walkConcurrent(absRootDir, options, walkFn, tracker, stats, processEntry, startTime)
	}

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
				processFile(path, relativePath, options, walkFn, tracker, stats)
			}
		}
		return nil
	})

	duration := time.Since(startTime)
	options.Logger.Debug("Walker: Total walk and processing time: %s", duration)

	return tracker.Items(), walkErr
}

// newProcessEntry builds the per-entry decision closure shared by Walk (full
// content) and WalkMeta (structure only). It reports whether an entry should
// be processed, or an error to stop the walk (e.g. SkipDir).
func newProcessEntry(
	absRootDir string,
	options WalkOptions,
	matcher *ignore.IgnoreMatcher,
	tracker *SkippedTracker,
	stats *walkStats,
) func(path string, d fs.DirEntry, err error) (error, bool) {
	return func(path string, d fs.DirEntry, err error) (error, bool) {
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

		// Pre-prune common heavy build directories at entry, before the full
		// ignore pass. Gated on the matcher so committed vendor/target dirs
		// that are not ignored are still walked.
		if isDir && heavyDirBasenames[filepath.Base(path)] &&
			matcher != nil && matcher.ShouldIgnore(relativePath, true) {
			options.Logger.Debug("Walker: Pruning heavy dir %q", relativePath)
			tracker.Track(relativePath, ReasonIgnoredRule, true)
			stats.skippedDirs.Add(1)
			return filepath.SkipDir, false
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

		// Path filter: drop files that the caller knows are unwanted before
		// any I/O happens.
		if options.PathFilter != nil && !options.PathFilter(relativePath) {
			options.Logger.Debug("Walker: File %q excluded by path filter", relativePath)
			tracker.Track(relativePath, ReasonFilteredPath, false)
			stats.skippedFiles.Add(1)
			return nil, false
		}

		options.Logger.Debug("Walker: File %q PASSED all checks, will be processed", relativePath)
		return nil, true
	}
}

// FileMeta describes a file without its content, backing the picker's
// structure-first skeleton so the TUI can open before any reads happen.
type FileMeta struct {
	Path    string
	Size    int64
	IsDir   bool
	ModTime time.Time
}

// WalkMeta enumerates a directory tree and returns file metadata only: no
// file is read. It applies the same ignore, extension, heavy-dir, and path
// filters as Walk, making it the cheap structure pass for progressive UIs.
func WalkMeta(rootDir string, matcher *ignore.IgnoreMatcher, opts ...Option) ([]FileMeta, []SkippedItem, error) {
	startTime := time.Now()

	options := defaultOptions()
	for _, opt := range opts {
		opt(&options)
	}

	absRootDir, err := filepath.Abs(rootDir)
	if err != nil {
		return nil, []SkippedItem{{Path: rootDir, Reason: ReasonSkippedPathError, IsDir: true}},
			fmt.Errorf("walker: failed to get absolute path for '%s': %w", rootDir, err)
	}

	tracker := NewSkippedTracker(100)
	stats := &walkStats{}
	processEntry := newProcessEntry(absRootDir, options, matcher, tracker, stats)

	options.Logger.Debug("walker.WalkMeta started. Root: %s", absRootDir)

	metas := make([]FileMeta, 0, 128)
	walkErr := filepath.WalkDir(absRootDir, func(path string, d fs.DirEntry, err error) error {
		processDecisionErr, shouldProcess := processEntry(path, d, err)
		if processDecisionErr != nil {
			return processDecisionErr
		}
		if !shouldProcess {
			return nil
		}

		relativePath, relErr := filepath.Rel(absRootDir, path)
		if relErr != nil {
			options.Logger.Error("Walker Error: Calculating relative path for meta %q: %v", path, relErr)
			tracker.Track(path, ReasonSkippedPathError, false)
			stats.skippedFiles.Add(1)
			return nil
		}
		if path == absRootDir || relativePath == "." {
			return nil
		}

		info, infoErr := d.Info()
		if infoErr != nil {
			options.Logger.Error("Walker Error: Failed to stat meta %q: %v", relativePath, infoErr)
			tracker.Track(relativePath, ReasonSkippedInfoError, false)
			stats.skippedFiles.Add(1)
			return nil
		}

		metas = append(metas, FileMeta{
			Path:    filepath.ToSlash(relativePath),
			Size:    info.Size(),
			IsDir:   info.IsDir(),
			ModTime: info.ModTime(),
		})
		return nil
	})

	options.Logger.Debug("Walker: Meta walk took %s, %d files", time.Since(startTime), len(metas))
	return metas, tracker.Items(), walkErr
}
