// Package picker runs the interactive picker workflow: structure-first
// skeleton launch, a background streaming scan, git-relevance rank patching,
// and selection-to-output conversion. It owns the scan lifecycle and
// selection callbacks so the CLI command only assembles options. The Bubble
// Tea model stays in internal/tui; this package bridges it.
package picker

import (
	"context"
	"path/filepath"
	"sync"
	"time"

	"github.com/bethropolis/sift/internal/app"
	"github.com/bethropolis/sift/internal/config"
	"github.com/bethropolis/sift/internal/format"
	"github.com/bethropolis/sift/internal/rank"
	"github.com/bethropolis/sift/internal/tui"
	"github.com/bethropolis/sift/internal/walker"
)

// Env supplies the picker with services beyond config: the application handle
// for scanning, rendering, and output, plus callbacks for recording dump and
// delta state and counting raw payloads, which live in the command layer.
type Env struct {
	App         *app.App
	RecordDump  func(rootDir string, files []format.FileEntry, ref string) error
	RecordDelta func(rootDir, ref string, files, tokens int) error
	CountTokens func(content []byte) int
}

// Result carries the picker workflow's outcome to the command layer.
type Result struct {
	// Selections is the user's file selection and modes, empty when the
	// session ended via a delta dump, the user selected nothing, or no
	// eligible files existed.
	Selections []tui.Selection
	// DeltaDone reports that the session ended by performing a delta dump,
	// whose output replaces any picker selection.
	DeltaDone bool
	// NoEligible reports that no files survived the ignore, binary, size, and
	// smart filters, so the picker never opened. The command surfaces this
	// distinctly from a plain non-selection.
	NoEligible bool
}

// Run executes the picker workflow and returns its outcome. When the user
// makes no selection the result carries empty Selections and DeltaDone false;
// callers distinguish "nothing eligible" via NoEligible.
func Run(ctx context.Context, cfg *config.Config, env Env) (Result, error) {
	s := &service{cfg: cfg, env: env}
	return s.run(ctx)
}

type service struct {
	cfg *config.Config
	env Env
}

func (s *service) run(ctx context.Context) (Result, error) {
	application := s.env.App
	application.EnablePickerVisibility()
	start := time.Now()

	// Structure-first launch: a cheap metadata pass builds the skeleton the
	// TUI shows instantly; a background walk then streams enriched entries in.
	metas, skipped, err := application.SkeletonPicker(ctx)
	if err != nil {
		return Result{}, err
	}

	// Analyze the last five commits to hint preferred modes before any
	// enrichment streams in. An empty map (non-git or error) leaves every
	// file with no preference.
	var preferredModes map[string]string
	absRoot, absErr := filepath.Abs(s.cfg.RootDir)
	if absErr == nil {
		preferredModes = rank.New(absRoot).AnalyzeCommitHistory(5)
	}

	skeletonItems, deltaFiles := buildSkeleton(metas, preferredModes)

	// Nothing survived the ignore, binary, size, and smart metadata filters:
	// do not open a picker that has nothing to choose.
	if len(skeletonItems) == 0 {
		return Result{NoEligible: true}, nil
	}

	// Shared collection state: the background walk fills it in and the
	// selection callbacks snapshot it on demand, so a slow walk never blocks
	// the picker and quitting early still sees every streamed file.
	var stateMu sync.Mutex
	var collected []format.FileEntry
	var skippedMu sync.Mutex

	snapshotFiles := func() []format.FileEntry {
		stateMu.Lock()
		defer stateMu.Unlock()
		return append([]format.FileEntry(nil), collected...)
	}
	snapshotSkipped := func() []walker.SkippedItem {
		skippedMu.Lock()
		defer skippedMu.Unlock()
		return append([]walker.SkippedItem(nil), skipped...)
	}

	totalFiles, totalDirs := 0, 0
	skeletonFilePaths := make([]string, 0, len(metas))
	for _, m := range metas {
		if m.IsDir {
			totalDirs++
		} else {
			totalFiles++
			skeletonFilePaths = append(skeletonFilePaths, m.Path)
		}
	}

	stream := newPickStream()
	go streamScan(ctx, application, preferredModes, absRoot, skeletonFilePaths,
		&stateMu, &collected, &skippedMu, &skipped,
		totalFiles, totalDirs, stream)

	result, err := tui.RunStreaming(skeletonItems, tui.Options{
		Budget:            s.cfg.Budget,
		Style:             s.cfg.EffectiveStyle(),
		UseNerd:           !s.cfg.NoNerdFonts,
		Highlight:         s.cfg.Highlight && !s.cfg.NoHighlight && !s.cfg.NoColor,
		Theme:             s.cfg.Theme,
		HighlightMaxBytes: s.cfg.HighlightMaxBytes,
		WindowTitle:       pickerWindowTitle(s.cfg),
		OnCopy: func(sel []tui.Selection) error {
			return s.copySelection(snapshotFiles(), sel)
		},
		OnGenerate: func(sel []tui.Selection) error {
			return s.generateSelection(snapshotFiles(), snapshotSkipped(), start, sel)
		},
		Delta: s.buildDeltaInfo(deltaFiles),
		OnDelta: func(sel tui.DeltaSelection) error {
			return s.performDelta(snapshotFiles(), sel)
		},
	}, stream.tuiStream())
	if err != nil {
		return Result{}, err
	}
	if result.DeltaDone {
		// The delta dump already wrote its own output and recorded state.
		return Result{DeltaDone: true}, nil
	}

	selected := result.Selections
	if len(selected) == 0 {
		return Result{}, nil
	}

	chosen := applySelection(application, snapshotFiles(), selected)
	if err := application.RenderFinal(chosen, snapshotSkipped(), time.Since(start), nil); err != nil {
		return Result{}, err
	}

	// Record the dump baseline so future delta dumps know what changed since
	// this selection. Non-fatal on failure.
	if err := s.env.RecordDump(s.cfg.RootDir, chosen, "HEAD"); err != nil {
		application.LogError("Failed to record dump state: %v", err)
	}
	return Result{Selections: selected}, nil
}

func pickerWindowTitle(cfg *config.Config) string {
	if cfg.NoWindowTitle {
		return ""
	}
	if cfg.WindowTitle != "" {
		return cfg.WindowTitle
	}
	root := filepath.Base(filepath.Clean(cfg.RootDir))
	if root == "." || root == string(filepath.Separator) || root == "" {
		root = "directory"
	}
	return "sift | " + root
}
