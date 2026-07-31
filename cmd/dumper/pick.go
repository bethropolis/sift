package main

import (
	"fmt"
	"os"
	"path/filepath"
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
	RunE: func(cmd *cobra.Command, args []string) error {
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

		files, skipped, err := application.Collect()
		if err != nil {
			return err
		}

		items := make([]tui.Item, len(files))
		for i, f := range files {
			items[i] = tui.Item{Path: filepath.ToSlash(f.Path), Tokens: f.Tokens}
		}

		selected, err := tui.Run(items)
		if err != nil {
			return fmt.Errorf("dumper pick: %w", err)
		}
		if len(selected) == 0 {
			return fmt.Errorf("dumper pick: no files selected")
		}

		keep := make(map[string]bool, len(selected))
		for _, p := range selected {
			keep[filepath.ToSlash(p)] = true
		}
		chosen := make([]format.FileEntry, 0, len(selected))
		for _, f := range files {
			if keep[filepath.ToSlash(f.Path)] {
				chosen = append(chosen, f)
			}
		}

		return application.Render(chosen, skipped, time.Since(start), nil)
	},
}
