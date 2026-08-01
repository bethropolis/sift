package main

import (
	"time"

	"github.com/spf13/cobra"

	"github.com/bethropolis/sift/internal/app"
	"github.com/bethropolis/sift/internal/config"
)

func init() {
	rootCmd.AddCommand(dumpCmd)
	config.RegisterFlags(cfg, dumpCmd.Flags())
}

// dumpCmd scans a directory and renders its contents.
var dumpCmd = &cobra.Command{
	Use:   "dump [path]",
	Short: "Scan a directory and render its contents",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 {
			cfg.RootDir = args[0]
		}

		if err := applyProfile(cmd); err != nil {
			return err
		}

		application := app.New(cfg)
		defer application.Close()

		start := time.Now()
		files, skipped, err := application.Collect()
		if err != nil {
			return err
		}
		if err := application.Render(files, skipped, time.Since(start), err); err != nil {
			return err
		}

		// Record the dump baseline so future delta dumps know what changed
		// since this scan. Failure to record is non-fatal.
		if err := recordDumpState(cfg.RootDir, files, "HEAD"); err != nil {
			application.LogError("Failed to record dump state: %v", err)
		}
		return nil
	},
}
