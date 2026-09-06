package app

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/bethropolis/sift/internal/config"
	"github.com/bethropolis/sift/internal/logger"
	"github.com/bethropolis/sift/internal/scan"
)

// App encapsulates the main application functionality
type App struct {
	cfg    *config.Config
	log    *logger.Logger
	output io.Writer

	// outputPath is the absolute path of the output file, or "" for stdout.
	// Files matching it are excluded from the walk so the dump never contains
	// itself.
	outputPath       string
	pickerVisibility bool

	// OnlyPaths, when non-nil, restricts the walk to these relative paths.
	// Used by sift diff to dump a curated set of files.
	OnlyPaths map[string]bool

	// contentCache memoizes token counts and signature summaries by content
	// hash across Collect calls, so watch re-renders skip tree-sitter and
	// BPE work for unchanged files.
	contentCache *scan.ContentCache
}

// EnablePickerVisibility lets the interactive picker receive hidden and
// gitignored metadata so the TUI can reveal it without rescanning.
func (a *App) EnablePickerVisibility() { a.pickerVisibility = true }

// New creates a new App instance. It resolves color usage from the terminal
// and output destination and opens the configured output file; an error is
// returned when the output file cannot be created. Color decisions are passed
// explicitly to the logger and renderer rather than mutating global state.
func New(cfg *config.Config) (*App, error) {
	// Resolve color usage from the terminal and output destination
	cfg.ResolveColors()

	// Set up output destination. A dash means stdout; otherwise the dump is
	// written to a file (codebase.md by default). Relative paths resolve
	// against the scanned root so the dump lands next to the codebase.
	var output io.Writer = os.Stdout
	var outputPath string
	if cfg.OutputFile != "" && cfg.OutputFile != "-" {
		outputPath = cfg.OutputFile
		if !filepath.IsAbs(outputPath) {
			base := cfg.RootDir
			if base == "" {
				base = "."
			}
			if absBase, err := filepath.Abs(base); err == nil {
				outputPath = filepath.Join(absBase, outputPath)
			}
		}
		outputPath, _ = filepath.Abs(outputPath)
		file, err := os.Create(outputPath)
		if err != nil {
			return nil, fmt.Errorf("failed to create output file: %w", err)
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
		cfg:          cfg,
		log:          log,
		output:       output,
		outputPath:   outputPath,
		contentCache: scan.NewContentCache(),
	}, nil
}

// Close performs cleanup, such as closing the output file if one was opened.
func (a *App) Close() {
	if f, ok := a.output.(*os.File); ok && f != os.Stdout && f != os.Stderr {
		f.Close()
	}
}

// infoLog logs at INFO level unless quiet mode is active.
func (a *App) infoLog(format string, args ...interface{}) {
	if !a.cfg.Quiet {
		a.log.Info(format, args...)
	}
}

// LogError logs an error message through the app's logger.
func (a *App) LogError(format string, args ...interface{}) {
	a.log.Error(format, args...)
}

// OutputPath returns the absolute path of the output file, or "" when writing
// to stdout.
func (a *App) OutputPath() string {
	return a.outputPath
}

// Output returns the writer the rendered document is written to, backing the
// configured output file or stdout. Callers that produce output outside the
// standard render path (e.g. delta patches) reuse it to honor --output.
func (a *App) Output() io.Writer {
	return a.output
}
