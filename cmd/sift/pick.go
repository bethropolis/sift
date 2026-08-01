package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"

	"github.com/bethropolis/sift/internal/app"
	"github.com/bethropolis/sift/internal/config"
	"github.com/bethropolis/sift/internal/format"
	"github.com/bethropolis/sift/internal/rank"
	"github.com/bethropolis/sift/internal/tui"
	"github.com/bethropolis/sift/internal/walker"
)

func init() {
	rootCmd.AddCommand(pickCmd)
	config.RegisterFlags(cfg, pickCmd.Flags())
}

// pickCmd interactively selects files, then renders the chosen context.
var pickCmd = &cobra.Command{
	Use:   "pick [path]",
	Short: "Interactively select files and render a context document",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runPick,
}

// runPick is shared by the pick subcommand and the bare root command, which
// launches the picker when invoked without a subcommand.
func runPick(cmd *cobra.Command, args []string) error {
	if !isatty.IsTerminal(os.Stdin.Fd()) {
		return fmt.Errorf("sift pick requires an interactive terminal")
	}

	if len(args) > 0 {
		cfg.RootDir = args[0]
	}
	if err := applyProfile(cmd); err != nil {
		return err
	}

	// The smart filter is always active in the picker so generated, lockfile,
	// minified, and oversized files never clutter the tree.
	cfg.SmartFilter = true

	// Suppress summary log lines to os.Stderr while the BubbleTea alt-screen
	// is active so generation never bleeds text onto the TUI.
	cfg.Quiet = true

	start := time.Now()
	application := app.New(cfg)
	defer application.Close()

	// CollectPicker keeps both full and signature-only content so the TUI can
	// offer per-file modes and live token tallies.
	files, skipped, err := application.CollectPicker()
	if err != nil {
		return err
	}

	// Analyze the last five commits to hint preferred modes: bulk commits map
	// to signatures, focused ones and working-tree edits to full content. An
	// empty map (non-git or error) leaves every file with no preference.
	var preferredModes map[string]string
	if absRoot, err := filepath.Abs(cfg.RootDir); err == nil {
		preferredModes = rank.New(absRoot).AnalyzeCommitHistory(5)
	}

	items := make([]tui.Item, len(files))
	for i, f := range files {
		items[i] = tui.Item{
			Path:          f.Path,
			Content:       f.Content,
			SigContent:    f.SigContent,
			TokensFull:    f.TokensFull,
			TokensSig:     f.TokensSig,
			SecretCount:   f.SecretCount,
			RankScore:     f.RankScore,
			PreferredMode: tui.CompressMode(preferredModes[f.Path]),
		}
	}

	result, err := tui.Run(items, tui.Options{
		Budget:  cfg.Budget,
		Style:   cfg.EffectiveStyle(),
		UseNerd: !cfg.NoNerdFonts,
		OnCopy: func(sel []tui.Selection) error {
			return copySelection(application, files, sel)
		},
		OnGenerate: func(sel []tui.Selection) error {
			return generateSelection(application, files, skipped, start, sel)
		},
		Delta: buildDeltaInfo(application, files),
		OnDelta: func(sel tui.DeltaSelection) error {
			return performDelta(application, files, sel)
		},
	})
	if err != nil {
		return fmt.Errorf("sift pick: %w", err)
	}
	if result.DeltaDone {
		// The delta dump already wrote its own output and recorded state.
		return nil
	}

	selected := result.Selections
	if len(selected) == 0 {
		return fmt.Errorf("sift pick: no files selected")
	}

	chosen := applySelection(files, selected)
	if err := application.RenderFinal(chosen, skipped, time.Since(start), nil); err != nil {
		return err
	}

	// Record the dump baseline so future delta dumps know what changed since
	// this selection. Non-fatal on failure.
	if err := recordDumpState(cfg.RootDir, chosen, "HEAD"); err != nil {
		application.LogError("Failed to record dump state: %v", err)
	}
	return nil
}

// applySelection maps the TUI's per-file modes onto the collected entries. A
// file in FULL mode keeps its raw content; SIGS uses the signature summary;
// SKIP entries are dropped.
func applySelection(files []format.FileEntry, selected []tui.Selection) []format.FileEntry {
	byPath := make(map[string]format.FileEntry, len(files))
	for _, f := range files {
		byPath[f.Path] = f
	}

	chosen := make([]format.FileEntry, 0, len(selected))
	for _, sel := range selected {
		f, ok := byPath[sel.Path]
		if !ok {
			continue
		}
		switch sel.Mode {
		case tui.ModeSignatures:
			if f.SigContent != nil {
				f.Content = f.SigContent
				f.Tokens = f.TokensSig
				f.IsCompressed = true
			} else {
				f.Tokens = f.TokensFull
			}
		case tui.ModeSkip:
			continue
		default: // ModeFull
			f.Tokens = f.TokensFull
			f.IsCompressed = false
		}
		chosen = append(chosen, f)
	}
	return chosen
}

// copySelection renders the current selection to the system clipboard without
// touching the picker's output destination.
func copySelection(application *app.App, files []format.FileEntry, selected []tui.Selection) error {
	chosen := applySelection(files, selected)
	if len(chosen) == 0 {
		return fmt.Errorf("nothing selected")
	}
	return application.RenderToClipboard(chosen)
}

// generateSelection renders the current selection to the output document
// without leaving the picker (pressing g). The output file is truncated first
// so repeated generates replace the previous dump instead of appending to it.
func generateSelection(application *app.App, files []format.FileEntry, skipped []walker.SkippedItem, start time.Time, selected []tui.Selection) error {
	chosen := applySelection(files, selected)
	if len(chosen) == 0 {
		return fmt.Errorf("nothing selected")
	}
	if f, ok := application.Output().(*os.File); ok {
		if err := f.Truncate(0); err != nil {
			return err
		}
		if _, err := f.Seek(0, 0); err != nil {
			return err
		}
	}
	if err := application.RenderFinal(chosen, skipped, time.Since(start), nil); err != nil {
		return err
	}
	// Update the baseline so a later delta dump knows what was just rendered.
	return recordDumpState(cfg.RootDir, chosen, "HEAD")
}
