// Package walker handles directory traversal and file processing
package walker

import (
	"context"
	"strings"

	"github.com/bethropolis/sift/internal/utils"
)

type WalkOptions struct {
	Logger        utils.Logger
	Concurrent    bool
	MaxWorkers    int
	MaxFileSize   int64
	ExtensionMap  map[string]struct{}
	IncludeBinary bool
	Context       context.Context
	ProgressFn    ProgressCallback // Add progress callback function
	PreReadFilter func(relativePath string) bool
	StatsFn       StatsCallback
	// PathFilter, when set, is consulted before a file is read. Returning
	// false excludes the file from processing, so filtered paths never cost
	// I/O or token work.
	PathFilter func(relativePath string) bool
}

type ProgressCallback func(stats ProgressStats)

type ProgressStats struct {
	TotalFiles      int64  // Total files seen
	ProcessedFiles  int64  // Files that passed all filters and were processed
	SkippedFiles    int64  // Files that were skipped for any reason
	TotalDirs       int64  // Total directories seen
	SkippedDirs     int64  // Directories that were skipped
	CurrentFilePath string // Path of the current file being processed (relative)
}

func defaultOptions() WalkOptions {
	return WalkOptions{
		Logger:       &utils.NoopLogger{},
		Concurrent:   false,
		MaxWorkers:   10,
		MaxFileSize:  0,   // No limit
		ExtensionMap: nil, // No extension filtering by default
		Context:      context.Background(),
		ProgressFn:   nil,
	}
}

type Option func(*WalkOptions)

func WithLogger(logger utils.Logger) Option {
	return func(opts *WalkOptions) {
		if logger != nil {
			opts.Logger = logger
		}
	}
}

func WithConcurrency(enabled bool) Option {
	return func(opts *WalkOptions) {
		opts.Concurrent = enabled
	}
}

func WithMaxWorkers(workers int) Option {
	return func(opts *WalkOptions) {
		if workers > 0 {
			opts.MaxWorkers = workers
		}
	}
}

func WithMaxFileSize(maxBytes int64) Option {
	return func(opts *WalkOptions) {
		opts.MaxFileSize = maxBytes
	}
}

func WithExtensions(extensions []string) Option {
	return func(opts *WalkOptions) {
		extMap := make(map[string]struct{}, len(extensions))
		for _, ext := range extensions {
			extMap[strings.TrimPrefix(ext, ".")] = struct{}{}
		}
		opts.ExtensionMap = extMap
	}
}

func WithExtensionMap(extMap map[string]struct{}) Option {
	return func(opts *WalkOptions) {
		if extMap != nil {
			newMap := make(map[string]struct{}, len(extMap))
			for ext := range extMap {
				newMap[strings.TrimPrefix(ext, ".")] = struct{}{}
			}
			opts.ExtensionMap = newMap
		} else {
			opts.ExtensionMap = nil
		}
	}
}

func WithContext(ctx context.Context) Option {
	return func(opts *WalkOptions) {
		if ctx != nil {
			opts.Context = ctx
		}
	}
}

// WithIncludeBinary controls whether binary files are included in the output.
// By default binary files are skipped.
func WithIncludeBinary(include bool) Option {
	return func(opts *WalkOptions) {
		opts.IncludeBinary = include
	}
}

func WithProgress(fn ProgressCallback) Option {
	return func(o *WalkOptions) {
		o.ProgressFn = fn
	}
}

// WithPreReadFilter excludes paths before stat/binary detection and content
// reads. It is intended for cheap name-only rules such as lockfiles and
// generated bundles.
func WithPreReadFilter(fn func(relativePath string) bool) Option {
	return func(opts *WalkOptions) { opts.PreReadFilter = fn }
}

// WithStats receives one aggregate snapshot after a walk completes.
func WithStats(fn StatsCallback) Option {
	return func(opts *WalkOptions) { opts.StatsFn = fn }
}

// WithPathFilter sets a predicate deciding whether a relative path is
// processed. It runs after all other skip rules (ignore, extension) but
// before any file is read, so excluded paths never hit the disk.
func WithPathFilter(fn func(relativePath string) bool) Option {
	return func(o *WalkOptions) {
		o.PathFilter = fn
	}
}
