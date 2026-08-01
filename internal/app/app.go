package app

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/fatih/color"

	"github.com/bethropolis/sift/internal/config"
	"github.com/bethropolis/sift/internal/logger"
)

// App encapsulates the main application functionality
type App struct {
	cfg    *config.Config
	log    *logger.Logger
	output io.Writer

	// outputPath is the absolute path of the output file, or "" for stdout.
	// Files matching it are excluded from the walk so the dump never contains
	// itself.
	outputPath string

	// OnlyPaths, when non-nil, restricts the walk to these relative paths.
	// Used by sift diff to dump a curated set of files.
	OnlyPaths map[string]bool
}

// New creates a new App instance
func New(cfg *config.Config) *App {
	// Resolve color usage from the terminal and output destination
	cfg.ResolveColors()

	// Configure color globally
	color.NoColor = !cfg.UseColors

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
		cfg:        cfg,
		log:        log,
		output:     output,
		outputPath: outputPath,
	}
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
