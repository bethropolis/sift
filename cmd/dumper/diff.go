package main

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/bethropolis/dir-dumper/internal/app"
	"github.com/bethropolis/dir-dumper/internal/config"
	"github.com/bethropolis/dir-dumper/internal/rank"
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
			return fmt.Errorf("dumper diff: %q is not inside a git repository", absRootDir)
		}
		if ref == "" {
			ref = "HEAD"
		}

		changed := g.ChangedSinceRef(ref)
		if len(changed) == 0 {
			return fmt.Errorf("dumper diff: no files changed since %s", ref)
		}

		only := make(map[string]bool, len(changed))
		for _, p := range changed {
			only[filepath.ToSlash(p)] = true
		}

		if err := applyProfile(cmd); err != nil {
			return err
		}

		application := app.New(cfg)
		defer application.Close()
		application.OnlyPaths = only
		return application.Run()
	},
}
