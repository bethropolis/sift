package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/bethropolis/sift/internal/app"
	"github.com/bethropolis/sift/internal/config"
)

// newDumpCmd scans a directory and renders its contents.
func newDumpCmd(cfg *config.Config) *cobra.Command {
	allTargets := false
	cmd := &cobra.Command{
		Use:   "dump [path]",
		Short: "Scan a directory and render its contents",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				cfg.RootDir = args[0]
				cfg.Roots = append([]string(nil), args...)
			}
			if allTargets {
				if cfg.Target != "" {
					return fmt.Errorf("--all-targets cannot be combined with --target")
				}
				if len(args) > 1 {
					return fmt.Errorf("--all-targets does not support multiple roots")
				}
				initial := *cfg
				runs, err := prepareTargetRuns(initial, cmd.Flags())
				if err != nil {
					return err
				}
				defer closeTargetRuns(runs)
				return renderTargetRuns(runs)
			}
			if err := applyProfile(cfg, cmd); err != nil {
				return err
			}
			roots := cfg.Roots
			if len(args) > 0 {
				roots = args
			} else if len(roots) == 0 {
				roots = []string{cfg.RootDir}
			}
			if len(roots) > 1 {
				if allTargets {
					return fmt.Errorf("--all-targets does not support multiple roots")
				}
				return renderMultiRoot(cfg, roots)
			}
			if len(cfg.Roots) == 1 {
				cfg.RootDir = cfg.Roots[0]
			}

			application, err := app.New(cfg)
			if err != nil {
				return err
			}
			defer application.Close()

			start := time.Now()
			files, skipped, err := application.Collect()
			if err != nil && !errors.Is(err, context.DeadlineExceeded) {
				return err
			}
			var metrics app.RenderMetrics
			var rendered []byte
			if historyEnabled(cfg) {
				metrics, rendered, err = application.RenderWithMetricsCaptured(files, skipped, time.Since(start), err)
			} else {
				metrics, err = application.RenderWithMetrics(files, skipped, time.Since(start), err)
			}
			if err != nil {
				return err
			}
			recordHistory(cfg, "dump", rendered, metrics)

			// Record the dump baseline so future delta dumps know what changed
			// since this scan. Failure to record is non-fatal.
			if err := recordRenderMetrics(cfg.RootDir, metrics, "HEAD"); err != nil {
				application.LogError("Failed to record dump state: %v", err)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&allTargets, "all-targets", allTargets, "Render every [[targets]] entry (rejects --target)")
	config.RegisterFlags(cfg, cmd.Flags())
	config.RegisterHistoryFlags(cfg, cmd.Flags())
	return cmd
}
