package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/bethropolis/dir-dumper/internal/app"
	"github.com/bethropolis/dir-dumper/internal/config"
	"github.com/bethropolis/dir-dumper/internal/watch"
)

func init() {
	rootCmd.AddCommand(watchCmd)
	config.RegisterFlags(cfg, watchCmd.Flags())
}

// watchCmd re-renders the context document whenever files change.
var watchCmd = &cobra.Command{
	Use:   "watch [path]",
	Short: "Watch a directory and re-render on changes",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 {
			cfg.RootDir = args[0]
		}
		if err := applyProfile(cmd); err != nil {
			return err
		}

		absRootDir, err := filepath.Abs(cfg.RootDir)
		if err != nil {
			return fmt.Errorf("invalid root directory path '%s': %w", cfg.RootDir, err)
		}

		// A watch is persistent; a per-collect timeout would abort renders.
		cfg.Timeout = 0

		application := app.New(cfg)
		defer application.Close()

		// First render.
		start := time.Now()
		files, skipped, err := application.Collect()
		if err != nil {
			return err
		}
		if err := application.Render(files, skipped, time.Since(start), err); err != nil {
			return err
		}

		outputPath := application.OutputPath()
		ignore := func(path string) bool {
			if strings.Contains(path, string(filepath.Separator)+".git") {
				return true
			}
			return outputPath != "" && path == outputPath
		}

		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		return watch.Run(ctx, watch.Options{
			Root:   absRootDir,
			Ignore: ignore,
			OnChange: func() error {
				start := time.Now()
				files, skipped, err := application.Collect()
				if err != nil {
					application.LogError("re-render: %v", err)
					return nil
				}
				return application.Render(files, skipped, time.Since(start), err)
			},
		})
	},
}
