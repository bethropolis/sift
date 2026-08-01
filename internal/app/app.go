package app

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/fatih/color"

	"github.com/bethropolis/dir-dumper/internal/clipboard"
	"github.com/bethropolis/dir-dumper/internal/compress"
	"github.com/bethropolis/dir-dumper/internal/config"
	"github.com/bethropolis/dir-dumper/internal/format"
	"github.com/bethropolis/dir-dumper/internal/ignore"
	"github.com/bethropolis/dir-dumper/internal/logger"
	"github.com/bethropolis/dir-dumper/internal/rank"
	"github.com/bethropolis/dir-dumper/internal/secrets"
	"github.com/bethropolis/dir-dumper/internal/setup"
	"github.com/bethropolis/dir-dumper/internal/summary"
	"github.com/bethropolis/dir-dumper/internal/tokenize"
	"github.com/bethropolis/dir-dumper/internal/walker"
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
	// Used by dumper diff to dump a curated set of files.
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

// Run executes the main application logic.
// It returns a non-nil error when the scan failed, including on timeout.
func (a *App) Run() error {
	startTime := time.Now() // Start timer for overall execution

	// Show version and exit if requested
	if a.cfg.ShowVersion {
		fmt.Printf("dir-dumper version %s\n", a.cfg.Version)
		return nil
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

	files, skippedItems, err := a.Collect()
	return a.Render(files, skippedItems, time.Since(startTime), err)
}

// Collect walks and processes the configured directory, returning the
// collected file entries ordered by git relevance. The token budget is
// deliberately not applied here so callers can curate the selection first.
func (a *App) Collect() ([]format.FileEntry, []walker.SkippedItem, error) {
	return a.collect(false)
}

// CollectPicker walks the directory for the interactive picker. Unlike
// Collect it keeps both the full and signature-only content and token counts
// per file (plus secret counts and git rank scores) so the TUI can show live
// previews, token tallies, and smart auto-selection. No global compression
// mode is applied; callers pick the active view per file.
func (a *App) CollectPicker() ([]format.FileEntry, []walker.SkippedItem, error) {
	return a.collect(true)
}

func (a *App) collect(picker bool) ([]format.FileEntry, []walker.SkippedItem, error) {
	// Handle timeout if specified
	var ctx context.Context
	var cancel context.CancelFunc

	if a.cfg.Timeout > 0 {
		ctx, cancel = context.WithTimeout(context.Background(), a.cfg.Timeout)
	} else {
		ctx, cancel = context.WithCancel(context.Background())
	}
	defer cancel()

	// --- Directory validation ---
	absRootDir, err := filepath.Abs(a.cfg.RootDir)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid root directory path '%s': %w", a.cfg.RootDir, err)
	}

	// Check if directory exists
	dirInfo, err := os.Stat(absRootDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, fmt.Errorf("root directory '%s' not found", absRootDir)
		}
		return nil, nil, fmt.Errorf("could not access root directory '%s': %w", absRootDir, err)
	}
	if !dirInfo.IsDir() {
		return nil, nil, fmt.Errorf("specified path '%s' is not a directory", absRootDir)
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

	matcher, walkOptions, err := setup.ConfigureWalker(walkerConfig, a.infoLog)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to configure walker: %w", err)
	}

	// --- Create the token counter ---
	tokenizer, err := tokenize.New(a.cfg.TokenizeModel)
	if err != nil {
		return nil, nil, err
	}

	// --- Create the secret scanner ---
	var scanner *secrets.Scanner
	if a.cfg.SecretScan && !a.cfg.ForceSecrets {
		scanner = secrets.New()
	}

	// --- Create the signature compressor. The picker needs it for every file
	// so it can offer per-file FULL/SIGS modes; dump/diff/watch only compress
	// when the global mode requests signatures. ---
	var compressor *compress.Compressor
	if picker || a.cfg.Mode == "signatures" {
		compressor = compress.New()
		a.log.Debug("Compression mode: signatures (tree-sitter)")
	}

	var filesMu sync.Mutex
	var files []format.FileEntry

	// --- Define walk function ---
	printFunc := func(relativePath string, content []byte, err error) error {
		if err != nil {
			a.log.Warn("Skipping file '%s' due to error: %v", relativePath, err)
			return nil // Error handled by logging
		}

		if content == nil {
			// This case shouldn't happen if err is nil, but good to log if it does
			a.log.Warn("printFunc called for '%s' with nil content and nil error.", relativePath)
			return nil
		}

		if a.OnlyPaths != nil && !a.OnlyPaths[filepath.ToSlash(relativePath)] {
			return nil
		}

		if a.outputPath != "" {
			absFile := filepath.Join(absRootDir, filepath.FromSlash(relativePath))
			if filepath.Clean(absFile) == filepath.Clean(a.outputPath) {
				return nil
			}
		}

		// Redact secrets before the content is stored for rendering.
		var detections []secrets.Detection
		if scanner != nil {
			var redacted []byte
			redacted, detections = scanner.Redact(content)
			for _, d := range detections {
				a.log.Warn("Redacted %s in %s", d.RuleName, relativePath)
			}
			content = redacted
		}

		entry := format.FileEntry{
			Path:        relativePath,
			Content:     content,
			Tokens:      -1,
			SecretCount: len(detections),
		}

		if picker {
			// Keep both full and signature-only variants plus their token
			// counts so the TUI can switch modes without re-reading files.
			// The active view is decided per file by the picker's Mode; the
			// entry's Tokens/IsCompressed are recomputed at selection time.
			entry.TokensFull = a.countTokens(tokenizer, entry.Content, relativePath)
			if compressor != nil {
				if lang, ok := compressor.LanguageForPath(relativePath); ok {
					compressed, didCompress := compressor.Compress(entry.Content, lang)
					if didCompress {
						entry.SigContent = []byte(compressed)
						entry.Language = lang.String()
					}
				}
			}
			if entry.SigContent != nil {
				entry.TokensSig = a.countTokens(tokenizer, entry.SigContent, relativePath)
			} else {
				entry.TokensSig = entry.TokensFull
			}
		} else {
			if compressor != nil {
				if lang, ok := compressor.LanguageForPath(relativePath); ok {
					var compressed string
					var didCompress bool
					compressed, didCompress = compressor.Compress(content, lang)
					content = []byte(compressed)
					entry.IsCompressed = didCompress
					entry.Language = lang.String()
				}
			}

			entry.Tokens = a.countTokens(tokenizer, content, relativePath)
			entry.TokensFull = entry.Tokens
			entry.TokensSig = entry.Tokens
		}

		filesMu.Lock()
		files = append(files, entry)
		filesMu.Unlock()
		return nil // Indicate success to walker
	}

	// --- Start the directory walk ---
	a.infoLog("Scanning directory: %s", absRootDir)
	if a.cfg.Concurrent {
		a.infoLog("Using concurrent processing with %d workers.", a.cfg.MaxWorkers)
	}

	skippedItems, err := a.walkDirectory(absRootDir, matcher, printFunc, walkOptions)

	// Order files by git relevance so the most important context survives the
	// token budget. Outside a git repository the order is left unchanged.
	if g := rank.New(absRootDir); g.Available() {
		a.log.Debug("Ranking %d files by git relevance", len(files))
		paths := make([]string, len(files))
		for i, f := range files {
			paths[i] = f.Path
		}
		scores := g.Score(absRootDir, paths)
		for i := range files {
			files[i].RankScore = scores[files[i].Path]
		}
		sort.SliceStable(files, func(i, j int) bool {
			return scores[files[i].Path] > scores[files[j].Path]
		})
	} else if picker {
		for i := range files {
			files[i].RankScore = rank.ScoreBaseline
		}
	}

	return files, skippedItems, err
}

// countTokens counts tokens for content, logging a warning on failure and
// returning 0 so a counting error never aborts the scan.
func (a *App) countTokens(tokenizer *tokenize.Tokenizer, content []byte, path string) int {
	tokens, err := tokenizer.Count(content)
	if err != nil {
		a.log.Warn("Failed to count tokens for %s: %v", path, err)
		return 0
	}
	return tokens
}

// Render applies the token budget to files and writes the rendered document.
// A non-nil runErr from Collect is reported but rendering still happens so the
// output stays well-formed (e.g. valid JSON on timeout).
func (a *App) Render(files []format.FileEntry, skippedItems []walker.SkippedItem, duration time.Duration, runErr error) error {
	return a.render(files, skippedItems, duration, runErr, true)
}

// RenderFinal renders exactly the given files without re-applying the token
// budget. It is used by the interactive picker, whose TUI owns budget
// accounting (including smart auto-selection); nothing is dropped afterwards.
func (a *App) RenderFinal(files []format.FileEntry, skippedItems []walker.SkippedItem, duration time.Duration, runErr error) error {
	return a.render(files, skippedItems, duration, runErr, false)
}

// RenderToClipboard renders the given files and copies the result to the
// system clipboard, without writing to the configured output destination. It
// backs the picker's live copy action.
func (a *App) RenderToClipboard(files []format.FileEntry) error {
	renderer, err := format.NewRenderer(format.ParseStyle(a.cfg.EffectiveStyle()), a.cfg.UseColors)
	if err != nil {
		return err
	}

	total := 0
	paths := make([]string, 0, len(files))
	for _, f := range files {
		total += f.Tokens
		paths = append(paths, f.Path)
	}
	doc := &format.Document{
		DirectoryTree: format.BuildTree(paths),
		Files:         files,
		TotalTokens:   total,
		Instructions:  a.cfg.Prompt,
	}

	var buf bytes.Buffer
	if err := renderer.Render(doc, &buf); err != nil {
		return err
	}
	return clipboard.Copy(buf.Bytes())
}

func (a *App) render(files []format.FileEntry, skippedItems []walker.SkippedItem, duration time.Duration, runErr error, applyBudget bool) error {
	// --- Create the renderer ---
	renderer, err := format.NewRenderer(format.ParseStyle(a.cfg.EffectiveStyle()), a.cfg.UseColors)
	if err != nil {
		return err
	}
	a.log.Debug("Output style: %s", a.cfg.EffectiveStyle())

	// Apply the token budget, keeping the highest-priority files.
	var usedTokens int
	if applyBudget && a.cfg.Budget > 0 {
		a.infoLog("Applying token budget: %d", a.cfg.Budget)
		var kept []format.FileEntry
		kept, usedTokens = tokenize.FitToBudget(files, a.cfg.Budget)
		files = kept
		if len(files) == 0 {
			a.log.Warn("Token budget %d is too small for any file.", a.cfg.Budget)
		}
	} else {
		for _, f := range files {
			usedTokens += f.Tokens
		}
	}
	tokenTotal := int64(usedTokens)

	paths := make([]string, 0, len(files))
	for _, f := range files {
		paths = append(paths, f.Path)
	}
	doc := &format.Document{
		DirectoryTree: format.BuildTree(paths),
		Files:         files,
		TotalTokens:   int(tokenTotal),
		Instructions:  a.cfg.Prompt,
	}

	// --- Render output ---
	if a.cfg.Clipboard {
		var buf bytes.Buffer
		renderErr := renderer.Render(doc, &buf)
		if renderErr != nil {
			a.log.Error("Error rendering output: %v", renderErr)
		} else if err := clipboard.Copy(buf.Bytes()); err != nil {
			a.log.Error("Failed to copy output to clipboard: %v", err)
		} else {
			a.infoLog("Copied %d files (%d tokens) to clipboard.", len(files), tokenTotal)
		}
	} else if renderErr := renderer.Render(doc, a.output); renderErr != nil {
		a.log.Error("Error rendering output: %v", renderErr)
	}

	// --- Handle walk errors ---
	if runErr != nil {
		if errors.Is(runErr, context.DeadlineExceeded) {
			a.log.Warn("Timeout of %v reached. Scan stopped.", a.cfg.Timeout)
		} else {
			a.log.Error("Critical error during directory walk: %v", runErr)
		}
		return runErr
	}

	// --- Show results summary ---
	summary.DisplayResults(a.log, int64(len(files)), tokenTotal, duration, a.cfg.Quiet)

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
