package main

import (
	"context"
	"fmt"
	"os"

	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"

	"github.com/bethropolis/sift/internal/app"
	"github.com/bethropolis/sift/internal/config"
	"github.com/bethropolis/sift/internal/picker"
	"github.com/bethropolis/sift/internal/state"
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
// launches the picker when invoked without a subcommand. It assembles the
// picker's options and delegates the entire workflow to the picker service.
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
	if prefs, err := state.LoadPreferences(); err == nil {
		if cfg.UITheme == "" {
			cfg.UITheme = prefs.UITheme
		}
	}

	// The smart filter is always active in the picker so generated, lockfile,
	// minified, and oversized files never clutter the tree.
	cfg.SmartFilter = true

	// Suppress summary log lines to os.Stderr while the BubbleTea alt-screen
	// is active so generation never bleeds text onto the TUI.
	cfg.Quiet = true

	application, err := app.New(cfg)
	if err != nil {
		return err
	}
	defer application.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	result, err := picker.Run(ctx, cfg, picker.Env{
		App:         application,
		RecordDump:  recordDumpState,
		RecordDelta: recordDeltaState,
		CountTokens: func(content []byte) int { return countTokensString(cfg, content) },
	})
	if err != nil {
		return fmt.Errorf("sift pick: %w", err)
	}
	if result.NoEligible {
		// Every file was filtered by ignore, binary, size, or smart rules, so
		// the picker never opened. Report the cause instead of a selection
		// mistake; --show-skipped still lists the filtered paths.
		return fmt.Errorf("sift pick: no eligible files found after applying ignore, binary, size, and smart filters")
	}
	if result.DeltaDone {
		// The delta dump already wrote its own output and recorded state.
		return nil
	}
	if len(result.Selections) == 0 {
		return fmt.Errorf("sift pick: no files selected")
	}
	return nil
}
