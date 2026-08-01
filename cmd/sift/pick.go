package main

import (
	"fmt"
	"os"
	"time"

	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"

	"github.com/bethropolis/sift/internal/app"
	"github.com/bethropolis/sift/internal/config"
	"github.com/bethropolis/sift/internal/format"
	"github.com/bethropolis/sift/internal/tui"
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

	start := time.Now()
	application := app.New(cfg)
	defer application.Close()

	// CollectPicker keeps both full and signature-only content so the TUI can
	// offer per-file modes and live token tallies.
	files, skipped, err := application.CollectPicker()
	if err != nil {
		return err
	}

	items := make([]tui.Item, len(files))
	for i, f := range files {
		items[i] = tui.Item{
			Path:        f.Path,
			Content:     f.Content,
			SigContent:  f.SigContent,
			TokensFull:  f.TokensFull,
			TokensSig:   f.TokensSig,
			SecretCount: f.SecretCount,
			RankScore:   f.RankScore,
		}
	}

	result, err := tui.Run(items, tui.Options{
		Budget:  cfg.Budget,
		Style:   cfg.EffectiveStyle(),
		UseNerd: !cfg.NoNerdFonts,
		OnCopy: func(sel []tui.Selection) error {
			return copySelection(application, files, sel)
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
