package app

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/bethropolis/sift/internal/config"
	"github.com/bethropolis/sift/internal/logger"
	"github.com/bethropolis/sift/internal/scan"
	"github.com/bethropolis/sift/internal/smart"
)

type App struct {
	cfg    *config.Config
	log    *logger.Logger
	output io.Writer
	outMu  sync.Mutex

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

	// processors caches one scan.Processor per collectMode so ReadEntry
	// reuses the tokenizer and compressor instead of rebuilding them.
	procMu     sync.Mutex
	processors map[collectMode]*scan.Processor

	// evalOnce guards the App-wide smart evaluator shared by the skeleton
	// pass, the walker pre-read filter, and every scan.Processor, so path
	// decisions are computed once and memoized across all three.
	evalOnce  sync.Once
	smartEval *smart.Evaluator
}

// Config returns the resolved application configuration. It is exposed to
// picker orchestration so streaming rank updates use the same scoring policy
// as non-interactive selection.
func (a *App) Config() *config.Config { return a.cfg }

// EnablePickerVisibility lets the interactive picker receive hidden and
// gitignored metadata so the TUI can reveal it without rescanning.
func (a *App) EnablePickerVisibility() { a.pickerVisibility = true }

// New creates a new App instance. It resolves color usage from the terminal
// and output destination and determines the configured output destination.
// Output file creation is deferred until output is actually rendered so the
// disk is never touched prematurely. Color decisions are passed explicitly to
// the logger and renderer rather than mutating global state.
func New(cfg *config.Config) (*App, error) {
	cfg.ResolveColors()
	cfg.ResolveOutputFile()

	var output io.Writer
	var outputPath string
	if cfg.OutputFile == "" || cfg.OutputFile == "-" {
		output = os.Stdout
	} else {
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
	}

	log := logger.New(os.Stderr, cfg.Verbose, cfg.UseColors)

	if cfg.LogLevel != "" {
		log.SetLevel(cfg.LogLevel)
	} else if cfg.Quiet {
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

// ensureOutput opens the configured output destination lazily on first access.
func (a *App) ensureOutput() (io.Writer, error) {
	a.outMu.Lock()
	defer a.outMu.Unlock()

	if a.output != nil {
		return a.output, nil
	}
	if a.outputPath == "" || a.cfg.OutputFile == "-" {
		a.output = os.Stdout
		return a.output, nil
	}
	file, err := os.Create(a.outputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create output file: %w", err)
	}
	a.output = file
	return a.output, nil
}

func (a *App) Close() {
	a.outMu.Lock()
	defer a.outMu.Unlock()

	if f, ok := a.output.(*os.File); ok && f != os.Stdout && f != os.Stderr {
		f.Close()
		a.output = nil
	}
}

func (a *App) infoLog(format string, args ...interface{}) {
	if !a.cfg.Quiet {
		a.log.Info(format, args...)
	}
}

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
	w, err := a.ensureOutput()
	if err != nil {
		a.log.Error("failed to create output file: %v", err)
		return io.Discard
	}
	return w
}
