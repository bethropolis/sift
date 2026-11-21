package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/bethropolis/sift/internal/app"
	"github.com/bethropolis/sift/internal/config"
	"github.com/bethropolis/sift/internal/watch"
)

// runWatch is a seam for tests; production always calls watch.Run.
var runWatch = watch.Run

// newWatchCmd re-renders the context document whenever files change.
func newWatchCmd(cfg *config.Config) *cobra.Command {
	allTargets := false
	cmd := &cobra.Command{
		Use:   "watch [path]",
		Short: "Watch a directory and re-render on changes",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				cfg.RootDir = args[0]
			}

			// A watch is persistent; a per-collect timeout would abort renders.
			cfg.Timeout = 0

			if allTargets {
				if cfg.Target != "" {
					return fmt.Errorf("--all-targets cannot be combined with --target")
				}
				// Resolve targets before applyProfile: otherwise default_profile
				// would mutate the base config and leak into targets that select
				// their own profile. dump.go branches the same way.
				initial := *cfg
				runs, err := prepareTargetRuns(initial, cmd.Flags())
				if err != nil {
					return err
				}
				defer closeTargetRuns(runs)
				for _, run := range runs {
					run.cfg.Timeout = 0
				}
				absRootDir, err := filepath.Abs(cfg.RootDir)
				if err != nil {
					return fmt.Errorf("invalid root directory path '%s': %w", cfg.RootDir, err)
				}
				if err := renderTargetRuns(runs); err != nil {
					return err
				}
				outputs := map[string]bool{}
				for _, run := range runs {
					if p := run.app.OutputPath(); p != "" {
						outputs[p] = true
					}
				}
				ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
				defer stop()
				return runWatch(ctx, watch.Options{
					Root: absRootDir,
					Ignore: func(path string) bool {
						if strings.Contains(path, string(filepath.Separator)+".git") {
							return true
						}
						return outputs[path]
					},
					OnChange: func() error {
						return renderTargetRuns(runs)
					},
				})
			}

			if err := applyProfile(cfg, cmd); err != nil {
				return err
			}
			// applyProfile can restore a configured collect timeout; a watch is
			// persistent, so clear it again after resolution.
			cfg.Timeout = 0
			absRootDir, err := filepath.Abs(cfg.RootDir)
			if err != nil {
				return fmt.Errorf("invalid root directory path '%s': %w", cfg.RootDir, err)
			}

			application, err := app.New(cfg)
			if err != nil {
				return err
			}
			defer application.Close()

			// First render.
			start := time.Now()
			files, skipped, err := application.Collect()
			if err != nil && !errors.Is(err, context.DeadlineExceeded) {
				return err
			}
			metrics, err := application.RenderWithMetrics(files, skipped, time.Since(start), err)
			if err != nil {
				return err
			}
			if err := recordRenderMetrics(absRootDir, metrics, "HEAD"); err != nil {
				application.LogError("Failed to record dump state: %v", err)
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

			return runWatch(ctx, watch.Options{
				Root:   absRootDir,
				Ignore: ignore,
				OnChange: func() error {
					start := time.Now()
					files, skipped, err := application.Collect()
					if err != nil {
						application.LogError("re-render: %v", err)
						return nil
					}
					metrics, err := application.RenderWithMetrics(files, skipped, time.Since(start), err)
					if err != nil {
						return err
					}
					if err := recordRenderMetrics(absRootDir, metrics, "HEAD"); err != nil {
						application.LogError("Failed to record dump state: %v", err)
					}
					return nil
				},
			})
		},
	}
	cmd.Flags().BoolVar(&allTargets, "all-targets", allTargets, "Watch and render every [[targets]] entry (rejects --target)")
	config.RegisterFlags(cfg, cmd.Flags())
	return cmd
}
