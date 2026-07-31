package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fatih/color"

	"github.com/bethropolis/dir-dumper/internal/config"
	"github.com/bethropolis/dir-dumper/internal/format"
	"github.com/bethropolis/dir-dumper/internal/ignore"
	"github.com/bethropolis/dir-dumper/internal/logger"
	"github.com/bethropolis/dir-dumper/internal/setup"
	"github.com/bethropolis/dir-dumper/internal/summary"
	"github.com/bethropolis/dir-dumper/internal/walker"
)

// App encapsulates the main application functionality
type App struct {
	cfg    *config.Config
	log    *logger.Logger
	output io.Writer
}

// New creates a new App instance
func New(cfg *config.Config) *App {
	// Resolve color usage from the terminal and output destination
	cfg.ResolveColors()

	// Configure color globally
	color.NoColor = !cfg.UseColors

	// Set up output destination
	var output io.Writer = os.Stdout
	if cfg.OutputFile != "" {
		file, err := os.Create(cfg.OutputFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: Failed to create output file: %v\n", err)
			os.Exit(1)
		}
		// Note: file will be closed by main function
		output = file
	}

	// Set up logger
	log := logger.New(os.Stderr, cfg.Verbose, cfg.UseColors)

	// Apply log level if specified (overrides verbose/quiet flags)
	if cfg.LogLevel != "" {
		log.SetLevel(cfg.LogLevel)
	} else if cfg.Quiet {
		// For backward compatibility
		log.WithLevel(logger.LevelWarn)
	}

	return &App{
		cfg:    cfg,
		log:    log,
		output: output,
	}
}

// Close performs cleanup, such as closing the output file if one was opened.
func (a *App) Close() {
	if f, ok := a.output.(*os.File); ok && f != os.Stdout && f != os.Stderr {
		f.Close()
	}
}

// Run executes the main application logic.
// It returns a non-nil error when the scan failed, including on timeout.
func (a *App) Run() error {
	startTime := time.Now() // Start timer for overall execution

	// Show version and exit if requested
	if a.cfg.ShowVersion {
		fmt.Printf("dir-dumper version %s\n", a.cfg.Version)
		return nil
	}

	// Handle timeout if specified
	var ctx context.Context
	var cancel context.CancelFunc

	if a.cfg.Timeout > 0 {
		ctx, cancel = context.WithTimeout(context.Background(), a.cfg.Timeout)
	} else {
		ctx, cancel = context.WithCancel(context.Background())
	}
	defer cancel()

	// Helper for info messages, suppressed by quiet flag
	infoLog := func(format string, args ...interface{}) {
		if !a.cfg.Quiet {
			a.log.Info(format, args...)
		}
	}

	if a.log.VerboseMode {
		a.log.Debug("Verbose mode enabled")
		a.log.Debug("Color output: %v", a.cfg.UseColors)
		a.log.Debug("Directory: %s", a.cfg.RootDir)
		a.log.Debug("Concurrent mode: %v (workers: %d)", a.cfg.Concurrent, a.cfg.MaxWorkers)
		a.log.Debug("Max file size: %d MB", a.cfg.MaxFileSizeMB)
		a.log.Debug("Ignore settings: hidden=%v, git=%v",
			a.cfg.IgnoreHidden, a.cfg.IgnoreGit)
		if a.cfg.CustomIgnore != "" {
			a.log.Debug("Custom ignore patterns: %s", a.cfg.CustomIgnore)
		}
		if a.cfg.Extensions != "" {
			a.log.Debug("Extensions filter: %s", a.cfg.Extensions)
		}
	}

	// --- Directory validation ---
	absRootDir, err := filepath.Abs(a.cfg.RootDir)
	if err != nil {
		return fmt.Errorf("invalid root directory path '%s': %w", a.cfg.RootDir, err)
	}

	// Check if directory exists
	dirInfo, err := os.Stat(absRootDir)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("root directory '%s' not found", absRootDir)
		}
		return fmt.Errorf("could not access root directory '%s': %w", absRootDir, err)
	}
	if !dirInfo.IsDir() {
		return fmt.Errorf("specified path '%s' is not a directory", absRootDir)
	}

	// Configure the walker using the setup package
	walkerConfig := setup.WalkerConfig{
		RootDir:       absRootDir,
		Concurrent:    a.cfg.Concurrent,
		MaxWorkers:    a.cfg.MaxWorkers,
		MaxFileSizeMB: a.cfg.MaxFileSizeMB,
		Extensions:    a.cfg.Extensions,
		IgnoreHidden:  a.cfg.IgnoreHidden,
		IgnoreGit:     a.cfg.IgnoreGit,
		CustomIgnore:  a.cfg.CustomIgnore,
		IncludeBinary: a.cfg.IncludeBinary,
		ShowProgress:  a.cfg.ShowProgress,
		Ctx:           ctx,
		Quiet:         a.cfg.Quiet,
		Logger:        a.log,
	}

	matcher, walkOptions, err := setup.ConfigureWalker(walkerConfig, infoLog)
	if err != nil {
		return fmt.Errorf("failed to configure walker: %w", err)
	}

	// --- Create the renderer ---
	renderer, err := format.NewRenderer(format.ParseStyle(a.cfg.EffectiveStyle()), a.cfg.UseColors)
	if err != nil {
		return err
	}
	if a.cfg.JSONOutput || a.cfg.MarkdownOutput || format.ParseStyle(a.cfg.Style) != format.StylePlain {
		a.log.Debug("Output style: %s", a.cfg.EffectiveStyle())
	}

	var filesMu sync.Mutex
	var files []format.FileEntry

	// --- Define walk function ---
	printFunc := func(relativePath string, content []byte, err error) error {
		if err != nil {
			a.log.Warn("Skipping file '%s' due to error: %v", relativePath, err)
			return nil // Error handled by logging
		}

		if content != nil { // Ensure content was actually read
			filesMu.Lock()
			files = append(files, format.FileEntry{Path: relativePath, Content: content})
			filesMu.Unlock()
		} else {
			// This case shouldn't happen if err is nil, but good to log if it does
			a.log.Warn("printFunc called for '%s' with nil content and nil error.", relativePath)
		}
		return nil // Indicate success to walker
	}

	// --- Start the directory walk ---
	infoLog("Scanning directory: %s", absRootDir)
	if a.cfg.Concurrent {
		infoLog("Using concurrent processing with %d workers.", a.cfg.MaxWorkers)
	}

	skippedItems, err := a.walkDirectory(absRootDir, matcher, printFunc, walkOptions)

	// Render whatever was collected, even if the walk stopped early, so the
	// output stays well-formed (e.g. valid JSON on timeout).
	paths := make([]string, 0, len(files))
	for _, f := range files {
		paths = append(paths, f.Path)
	}
	doc := &format.Document{
		DirectoryTree: format.BuildTree(paths),
		Files:         files,
	}
	renderErr := renderer.Render(doc, a.output)

	// --- Handle walk errors ---
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			a.log.Warn("Timeout of %v reached. Scan stopped.", a.cfg.Timeout)
		} else {
			a.log.Error("Critical error during directory walk: %v", err)
		}
		if renderErr != nil {
			a.log.Error("Error rendering output: %v", renderErr)
		}
		return err
	}
	if renderErr != nil {
		return renderErr
	}

	// --- Show results summary ---
	summary.DisplayResults(a.log, int64(len(files)), time.Since(startTime), a.cfg.Quiet)

	// --- Show Skipped Items (if requested) ---
	if a.cfg.ShowSkipped {
		summary.DisplaySkippedItems(a.log, skippedItems, os.Stderr, a.cfg.Quiet)
	}

	return nil
}

// walkDirectory is a helper method that performs the actual directory walk
func (a *App) walkDirectory(
	rootDir string,
	matcher *ignore.IgnoreMatcher,
	walkFn walker.WalkFunc,
	options []walker.Option,
) ([]walker.SkippedItem, error) {
	return walker.Walk(rootDir, matcher, walkFn, options...)
}
