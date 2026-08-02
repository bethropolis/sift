package main

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/bethropolis/sift/internal/app"
	"github.com/bethropolis/sift/internal/config"
	"github.com/bethropolis/sift/internal/rank"
)

func init() {
	rootCmd.AddCommand(diffCmd)
	config.RegisterFlags(cfg, diffCmd.Flags())
}

// diffCmd renders a context document for files changed by a git ref.
var diffCmd = &cobra.Command{
	Use:   "diff [ref]",
	Short: "Dump context for files changed by a git ref",
	Long: `Dump a context document for the files that differ between the working
tree and a git ref (default: HEAD, i.e. uncommitted changes).

Files changed relative to the ref are collected from git and the walk is
restricted to them, so the output contains exactly the edited surface.`,
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

		if err := applyProfile(cmd); err != nil {
			return err
		}

		application, err := app.New(cfg)
		if err != nil {
			return err
		}
		defer application.Close()
		application.OnlyPaths = only

		start := time.Now()
		files, skipped, err := application.Collect()
		if err != nil {
			return err
		}
		if err := application.Render(files, skipped, time.Since(start), err); err != nil {
			return err
		}

		// Record the diffed ref as the dump baseline so a later delta resumes
		// from exactly what this diff covered. Non-fatal on failure.
		if err := recordDumpState(absRootDir, files, ref); err != nil {
			application.LogError("Failed to record dump state: %v", err)
		}
		return nil
	},
}
