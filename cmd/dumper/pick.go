package main

import (
	"fmt"
	"os"
	"time"

	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"

	"github.com/bethropolis/dir-dumper/internal/app"
	"github.com/bethropolis/dir-dumper/internal/config"
	"github.com/bethropolis/dir-dumper/internal/format"
	"github.com/bethropolis/dir-dumper/internal/tui"
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
		return fmt.Errorf("dumper pick requires an interactive terminal")
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

	selected, err := tui.Run(items, tui.Options{
		Budget:  cfg.Budget,
		Style:   cfg.EffectiveStyle(),
		UseNerd: !cfg.NoNerdFonts,
		OnCopy: func(sel []tui.Selection) error {
			return copySelection(application, files, sel)
		},
	})
	if err != nil {
		return fmt.Errorf("dumper pick: %w", err)
	}
	if len(selected) == 0 {
		return fmt.Errorf("dumper pick: no files selected")
	}

	chosen := applySelection(files, selected)
	return application.RenderFinal(chosen, skipped, time.Since(start), nil)
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
