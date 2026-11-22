package main

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/bethropolis/sift/internal/app"
	"github.com/bethropolis/sift/internal/config"
	"github.com/bethropolis/sift/internal/rank"
)

// newDiffCmd renders a context document for files changed by a git ref.
func newDiffCmd(cfg *config.Config) *cobra.Command {
	withContext := false
	contextDepth := 1
	cmd := &cobra.Command{
		Use:   "diff [ref]",
		Short: "Dump context for files changed by a git ref",
		Long: `Dump a context document for the files that differ between the working
tree and a git ref (default: HEAD, i.e. uncommitted changes).

Files changed relative to the ref are collected from git and the walk is
restricted to them, so the output contains exactly the edited surface.

With --with-context the unified diff is emitted first, followed by the
import-graph neighbors of the changed files as signatures.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ref := ""
			if len(args) > 0 {
				ref = args[0]
			}

			rootDir := cfg.RootDir
			absRootDir, err := filepath.Abs(rootDir)
			if err != nil {
				return fmt.Errorf("invalid root directory path '%s': %w", rootDir, err)
			}

			g := rank.New(absRootDir)
			if !g.Available() {
				return fmt.Errorf("sift diff: %q is not inside a git repository", absRootDir)
			}
			if ref == "" {
				ref = "HEAD"
			}

			changed := g.ChangedSinceRef(ref)
			if len(changed) == 0 {
				return fmt.Errorf("sift diff: no files changed since %s", ref)
			}

			only := make(map[string]bool, len(changed))
			for _, p := range changed {
				only[filepath.ToSlash(p)] = true
			}

			if err := applyProfile(cfg, cmd); err != nil {
				return err
			}

			application, err := app.New(cfg)
			if err != nil {
				return err
			}
			defer application.Close()
			start := time.Now()
			var metrics app.RenderMetrics
			var rendered []byte
			if withContext {
				files, skipped, collectErr := application.CollectPicker()
				if collectErr != nil && !errors.Is(collectErr, context.DeadlineExceeded) {
					return collectErr
				}
				neighbors := app.DiffNeighborFiles(cfg, files, changed, contextDepth)
				patch := strings.TrimSpace(g.RawWorktreePatch(ref))
				if historyEnabled(cfg) {
					metrics, rendered, err = application.RenderPatchWithMetricsCaptured(neighbors, patch, skipped, time.Since(start), collectErr)
				} else {
					metrics, err = application.RenderPatchWithMetrics(neighbors, patch, skipped, time.Since(start), collectErr)
				}
			} else {
				application.OnlyPaths = only
				files, skipped, collectErr := application.Collect()
				if collectErr != nil && !errors.Is(collectErr, context.DeadlineExceeded) {
					return collectErr
				}
				if historyEnabled(cfg) {
					metrics, rendered, err = application.RenderWithMetricsCaptured(files, skipped, time.Since(start), collectErr)
				} else {
					metrics, err = application.RenderWithMetrics(files, skipped, time.Since(start), collectErr)
				}
			}
			if err != nil {
				return err
			}
			recordHistoryAt(cfg, absRootDir, "diff", rendered, metrics)

			// Record the diffed ref as the dump baseline so a later delta resumes
			// from exactly what this diff covered. Non-fatal on failure.
			if err := recordRenderMetrics(absRootDir, metrics, ref); err != nil {
				application.LogError("Failed to record dump state: %v", err)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&withContext, "with-context", withContext, "Emit the unified diff plus import-graph neighbors in signatures mode")
	cmd.Flags().IntVar(&contextDepth, "context-depth", contextDepth, "Import hops to include around changed files when --with-context is set")
	config.RegisterFlags(cfg, cmd.Flags())
	config.RegisterHistoryFlags(cfg, cmd.Flags())
	return cmd
}
