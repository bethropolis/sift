package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/bethropolis/sift/internal/compress"
	"github.com/bethropolis/sift/internal/format"
	"github.com/bethropolis/sift/internal/rank"
	"github.com/bethropolis/sift/internal/secrets"
	"github.com/bethropolis/sift/internal/setup"
	"github.com/bethropolis/sift/internal/tokenize"
	"github.com/bethropolis/sift/internal/walker"
)

// Run executes the main application logic.
// It returns a non-nil error when the scan failed, including on timeout.
func (a *App) Run() error {
	startTime := time.Now() // Start timer for overall execution

	// Show version and exit if requested
	if a.cfg.ShowVersion {
		fmt.Printf("sift version %s\n", a.cfg.Version)
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
