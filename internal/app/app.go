package app

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/bethropolis/dir-dumper/internal/config"
	"github.com/bethropolis/dir-dumper/internal/ignore"
	"github.com/bethropolis/dir-dumper/internal/logger"
	"github.com/bethropolis/dir-dumper/internal/printer"
	"github.com/bethropolis/dir-dumper/internal/setup"
	"github.com/bethropolis/dir-dumper/internal/summary"
	"github.com/bethropolis/dir-dumper/internal/walker"
	"github.com/fatih/color"
)

// App encapsulates the main application functionality
type App struct {
	cfg    *config.Config
	log    *logger.Logger
	Output io.Writer // Changed from output to Output (exported)
}

// New creates a new App instance
func New(cfg *config.Config) *App {
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
		Output: output,
	}
}

// Run executes the main application logic
func (a *App) Run() {
	startTime := time.Now() // Start timer for overall execution

	// Show version and exit if requested
	if a.cfg.ShowVersion {
		fmt.Printf("dir-dumper version %s\n", a.cfg.Version)
		os.Exit(0)
	}

	// Handle timeout if specified
	var ctx context.Context
	var cancel context.CancelFunc

	if a.cfg.Timeout > 0 {
		ctx, cancel = context.WithTimeout(context.Background(), a.cfg.Timeout)
		defer cancel()

		go func() {
			<-ctx.Done()
			if ctx.Err() == context.DeadlineExceeded {
				fmt.Fprintf(os.Stderr, "\nTimeout of %v reached. Exiting.\n", a.cfg.Timeout)
				os.Exit(1)
			}
		}()
	} else {
		ctx, cancel = context.WithCancel(context.Background())
		defer cancel()
	}

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
		a.log.Error("Invalid root directory path '%s': %v", a.cfg.RootDir, err)
		os.Exit(1)
	}

	// Check if directory exists
	dirInfo, err := os.Stat(absRootDir)
	if err != nil {
		if os.IsNotExist(err) {
			a.log.Error("Root directory '%s' not found.", absRootDir)
		} else {
			a.log.Error("Could not access root directory '%s': %v", absRootDir, err)
		}
		os.Exit(1)
	}
	if !dirInfo.IsDir() {
		a.log.Error("Specified path '%s' is not a directory.", absRootDir)
		os.Exit(1)
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
		ShowProgress:  a.cfg.ShowProgress,
		Timeout:       ctx,
		Quiet:         a.cfg.Quiet,
		Logger:        a.log,
	}

	matcher, walkOptions, err := setup.ConfigureWalker(walkerConfig, infoLog)
	if err != nil {
		a.log.Error("%v", err)
		os.Exit(1)
	}

	// --- Create the printer ---
	p := printer.New()
	p.WithOutput(a.Output)
	p.WithColors(a.cfg.UseColors)

	// Enable JSON output if requested
	if a.cfg.JSONOutput {
		a.log.Debug("JSON output mode enabled")
		p.WithJSON(true)
		// Disable colors in JSON mode regardless of other settings
		p.WithColors(false)
	} else if a.cfg.MarkdownOutput {
		a.log.Debug("Markdown output mode enabled")
		p.WithMarkdown(true)
		// Disable colors in Markdown mode regardless of other settings
		p.WithColors(false)
	}

	// --- Define walk function ---
	printFunc := func(relativePath string, content []byte, err error) error {
		if err != nil {
			a.log.Warn("Skipping file '%s' due to error: %v", relativePath, err)
			return nil // Error handled by logging
		}

		// Debug: Log every file that reaches the printFunc
		a.log.Debug("Walk callback received file: %s (content nil? %v)",
			relativePath, content == nil)

		if content != nil { // Ensure content was actually read
			// Debug info before printing
			a.log.Debug("About to print file: %s (%d bytes)", relativePath, len(content))
			p.PrintFile(relativePath, content)
			// Debug info after printing
			a.log.Debug("After printing file: %s (printer count: %d)", relativePath, p.GetCount())
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

	// --- Handle walk errors ---
	if err != nil {
		a.log.Error("Critical error during directory walk: %v", err)
		os.Exit(1)
	}

	// --- Show results summary ---
	summary.DisplayResults(a.log, p.GetCount(), time.Since(startTime), a.cfg.Quiet)

	// Finalize the printer (important for JSON output to close the array)
	p.Finalize()

	// --- Show Skipped Items (if requested) ---
	if a.cfg.ShowSkipped {
		summary.DisplaySkippedItems(a.log, skippedItems, os.Stderr, a.cfg.Quiet)
	}
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
