package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/bethropolis/sift/internal/format"
	"github.com/bethropolis/sift/internal/ignore"
	"github.com/bethropolis/sift/internal/rank"
	"github.com/bethropolis/sift/internal/scan"
	"github.com/bethropolis/sift/internal/setup"
	"github.com/bethropolis/sift/internal/smart"
	"github.com/bethropolis/sift/internal/walker"
)

// pickDefaultMaxFileSizeMB caps per-file reads in the picker when no explicit
// limit is set. The smart filter would drop oversized files anyway, but this
// avoids reading them into memory in the first place.
const pickDefaultMaxFileSizeMB int64 = 10

// ErrFileSkipped is returned by ReadEntry when the smart filter rejects the
// requested file (e.g. a lockfile or generated artifact). It aliases the
// processor's sentinel so callers can unwrap the underlying reason.
var ErrFileSkipped = scan.ErrFileSkipped

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
	ctx, cancel := a.scanContext()
	defer cancel()

	var mu sync.Mutex
	var files []format.FileEntry
	skipped, err := a.walkAndCollect(picker, ctx, func(e format.FileEntry) error {
		mu.Lock()
		files = append(files, e)
		mu.Unlock()
		return nil
	})
	if err != nil {
		return files, skipped, err
	}

	// Order files by git relevance so the most important context survives the
	// token budget. Outside a git repository the order is left unchanged.
	absRootDir, absErr := a.absRoot()
	if absErr == nil {
		a.applyRank(picker, absRootDir, &files)
	}
	return files, skipped, nil
}

// scanContext builds a cancellable context honoring the configured timeout.
func (a *App) scanContext() (context.Context, context.CancelFunc) {
	if a.cfg.Timeout > 0 {
		return context.WithTimeout(context.Background(), a.cfg.Timeout)
	}
	return context.WithCancel(context.Background())
}

// walkAndCollect runs the walk and per-file processing, calling emit for every
// accepted file. It never sorts or ranks; the blocking collectors do that
// after the walk, and streaming callers patch ranks asynchronously.
func (a *App) walkAndCollect(picker bool, ctx context.Context, emit func(format.FileEntry) error) ([]walker.SkippedItem, error) {
	absRootDir, err := a.absRoot()
	if err != nil {
		return nil, err
	}

	matcher, walkOptions, err := a.walkerOptions(absRootDir, ctx, picker)
	if err != nil {
		return nil, err
	}

	// --- Create the per-file processor. It owns token counting, smart
	// filtering, and signature compression. The tokenizer codec is safe for
	// concurrent Count calls and the compressor pools its parsers, so the
	// walker's workers share one instance. ---
	processor, err := scan.New(scan.Options{
		TokenizeModel:  a.cfg.TokenizeModel,
		SmartFilter:    a.cfg.SmartFilter,
		SmartMaxTokens: a.cfg.SmartMaxTokens,
		Logger:         a.log,
	})
	if err != nil {
		return nil, err
	}
	if a.cfg.SmartFilter {
		a.log.Debug("Smart filter enabled (max %d tokens/file)", a.cfg.SmartMaxTokens)
	}
	if picker || a.cfg.Mode == "signatures" {
		a.log.Debug("Compression mode: signatures (tree-sitter)")
	}

	// Smart-skipped items are tracked separately and appended to the walker's
	// skip list so --show-skipped surfaces them.
	var smartMu sync.Mutex
	var smartSkipped []walker.SkippedItem

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

		mode := scan.ModeFull
		if picker {
			mode = scan.ModePicker
		} else if a.cfg.Mode == "signatures" {
			mode = scan.ModeSignatures
		}

		entry, err := processor.Process(relativePath, content, mode)
		if err != nil {
			if errors.Is(err, scan.ErrFileSkipped) {
				a.log.Debug("SmartFilter skipping %s: %v", relativePath, err)
				smartMu.Lock()
				smartSkipped = append(smartSkipped, walker.SkippedItem{Path: relativePath, Reason: walker.ReasonSkippedSmart})
				smartMu.Unlock()
				return nil
			}
			a.log.Warn("Processing %s failed: %v", relativePath, err)
			return nil
		}
		return emit(entry)
	}

	// --- Start the directory walk ---
	a.infoLog("Scanning directory: %s", absRootDir)
	if a.cfg.Concurrent {
		a.infoLog("Using concurrent processing with %d workers.", a.cfg.MaxWorkers)
	}

	skippedItems, err := a.walkDirectory(absRootDir, matcher, printFunc, walkOptions)

	// Fold smart-filtered files into the walker's skip list.
	if len(smartSkipped) > 0 {
		smartMu.Lock()
		skippedItems = append(skippedItems, smartSkipped...)
		smartMu.Unlock()
	}
	return skippedItems, err
}

// StreamPicker walks the directory for the interactive picker without
// blocking: emit is called for each accepted file as soon as it is processed
// (possibly concurrently), and rank scores are not applied here so the caller
// can patch them asynchronously. The returned skip list is complete only
// after the walk finishes.
func (a *App) StreamPicker(ctx context.Context, emit func(format.FileEntry) error) ([]walker.SkippedItem, error) {
	return a.walkAndCollect(true, ctx, emit)
}

// SkeletonPicker returns file metadata (no content) for the picker's
// structure-first launch. It applies the same ignore, extension, prune, and
// smart name rules as the full walk so the skeleton matches the files that
// will later be enriched. Smart-skipped entries are folded into the skip list.
func (a *App) SkeletonPicker(ctx context.Context) ([]walker.FileMeta, []walker.SkippedItem, error) {
	absRootDir, err := a.absRoot()
	if err != nil {
		return nil, nil, err
	}

	matcher, walkOptions, err := a.walkerOptions(absRootDir, ctx, true)
	if err != nil {
		return nil, nil, err
	}

	metas, skipped, err := walker.WalkMeta(absRootDir, matcher, walkOptions...)
	if err != nil {
		return metas, skipped, err
	}

	var smartEvaluator *smart.Evaluator
	if a.cfg.SmartFilter {
		smartEvaluator = smart.New(a.cfg.SmartMaxTokens)
	}

	if smartEvaluator != nil {
		kept := metas[:0]
		for _, m := range metas {
			// Approximate token guardrail (bytes/4) mirrors the content-phase
			// check so skeleton and enrichment agree on which files survive.
			if skip, _ := smartEvaluator.ShouldSkipMeta(m.Path, int(m.Size/4)); skip {
				skipped = append(skipped, walker.SkippedItem{Path: m.Path, Reason: walker.ReasonSkippedSmart})
				continue
			}
			kept = append(kept, m)
		}
		metas = kept
	}

	return metas, skipped, nil
}

// ReadEntry reads and processes a single relative path exactly as the picker
// would, returning a fully enriched entry. It backs the picker's fallback when
// a selected file has not finished streaming in yet, and returns ErrFileSkipped
// when the smart filter rejects the file.
func (a *App) ReadEntry(relativePath string) (format.FileEntry, error) {
	absRootDir, err := a.absRoot()
	if err != nil {
		return format.FileEntry{}, err
	}
	processor, err := scan.New(scan.Options{
		TokenizeModel:  a.cfg.TokenizeModel,
		SmartFilter:    a.cfg.SmartFilter,
		SmartMaxTokens: a.cfg.SmartMaxTokens,
		Logger:         a.log,
	})
	if err != nil {
		return format.FileEntry{}, err
	}
	absFile, err := resolveWithinRoot(absRootDir, relativePath)
	if err != nil {
		return format.FileEntry{}, err
	}
	content, err := os.ReadFile(absFile)
	if err != nil {
		return format.FileEntry{}, err
	}
	entry, err := processor.Process(relativePath, content, scan.ModePicker)
	if err != nil {
		return format.FileEntry{}, err
	}
	entry.Tokens = entry.TokensFull
	return entry, nil
}

// absRoot resolves and validates the scan root directory.
func (a *App) absRoot() (string, error) {
	absRootDir, err := filepath.Abs(a.cfg.RootDir)
	if err != nil {
		return "", fmt.Errorf("invalid root directory path '%s': %w", a.cfg.RootDir, err)
	}

	dirInfo, err := os.Stat(absRootDir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("root directory '%s' not found", absRootDir)
		}
		return "", fmt.Errorf("could not access root directory '%s': %w", absRootDir, err)
	}
	if !dirInfo.IsDir() {
		return "", fmt.Errorf("specified path '%s' is not a directory", absRootDir)
	}
	return absRootDir, nil
}

// walkerOptions assembles the ignore matcher and walker options for the scan
// root, including the picker's default size cap and the OnlyPaths/output-path
// path filter.
func (a *App) walkerOptions(absRootDir string, ctx context.Context, picker bool) (*ignore.IgnoreMatcher, []walker.Option, error) {
	maxFileSizeMB := a.cfg.MaxFileSizeMB
	if picker && maxFileSizeMB <= 0 {
		// The picker's smart filter would reject oversized files anyway, but
		// reading them into memory first is wasted work. Cap the read so huge
		// files are skipped by the walker before content is loaded.
		maxFileSizeMB = pickDefaultMaxFileSizeMB
	}
	walkerConfig := setup.WalkerConfig{
		RootDir:       absRootDir,
		Concurrent:    a.cfg.Concurrent,
		MaxWorkers:    a.cfg.MaxWorkers,
		MaxFileSizeMB: maxFileSizeMB,
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

	// Skip OnlyPaths and the output document before any file is read. These
	// path-only rules belong in the walker decision stage so excluded files
	// never cost disk I/O or token work.
	if a.OnlyPaths != nil || a.outputPath != "" {
		walkOptions = append(walkOptions, walker.WithPathFilter(func(relativePath string) bool {
			if a.OnlyPaths != nil && !a.OnlyPaths[filepath.ToSlash(relativePath)] {
				return false
			}
			if a.outputPath != "" {
				absFile, err := resolveWithinRoot(absRootDir, relativePath)
				if err == nil && filepath.Clean(absFile) == filepath.Clean(a.outputPath) {
					return false
				}
			}
			return true
		}))
	}
	return matcher, walkOptions, nil
}

// applyRank scores and sorts files by the unified relevance score for the
// blocking collect path. Outside a git repository picker files keep the
// baseline score.
func (a *App) applyRank(picker bool, absRootDir string, files *[]format.FileEntry) {
	if g := rank.New(absRootDir); g.Available() {
		a.log.Debug("Ranking %d files by git relevance", len(*files))
		params := make([]rank.ScoringParams, len(*files))
		for i, f := range *files {
			params[i] = rank.ScoringParams{
				Path:        f.Path,
				TokensFull:  f.TokensFull,
				TokensSig:   f.TokensSig,
				DidCompress: f.IsCompressed || f.SigContent != nil,
			}
		}
		results := g.CalculateUnifiedScores(absRootDir, params)
		for i := range *files {
			(*files)[i].RankScore = results[(*files)[i].Path].Score
		}
		sort.SliceStable(*files, func(i, j int) bool {
			return results[(*files)[i].Path].Score > results[(*files)[j].Path].Score
		})
	} else if picker {
		for i := range *files {
			(*files)[i].RankScore = rank.ScoreBaseline
		}
	}
}
