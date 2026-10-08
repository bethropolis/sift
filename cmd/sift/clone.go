package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/bethropolis/sift/internal/app"
	"github.com/bethropolis/sift/internal/config"
	"github.com/bethropolis/sift/internal/gitclone"
)

var (
	cloneDepth  int
	cloneHere   bool
	cloneBranch string
)

func init() {
	cloneCmd.Flags().IntVar(&cloneDepth, "depth", 1, "Git history depth to clone")
	cloneCmd.Flags().StringVarP(&cloneBranch, "branch", "b", "", "Branch or tag to clone (default: remote's default branch)")
	cloneCmd.Flags().BoolVar(&cloneHere, "here", false, "Clone into the current directory instead of a temporary directory")
	rootCmd.AddCommand(cloneCmd)
	config.RegisterFlags(cfg, cloneCmd.Flags())
}

var cloneCmd = &cobra.Command{
	Use:   "clone <repository>",
	Short: "Shallow-clone a repository and dump its context",
	Long: `Clone a repository with limited history, then write codebase.md in the
current directory. By default the checkout is temporary and removed after the
dump. Use --here to keep the checkout in the current directory.

Only remote repositories are supported (https, ssh, git, or user@host:path).
Local paths and file:// URLs are refused. Authentication is whatever your git
already does: credential helpers, ssh keys, and the ssh agent.`,
	Args: cobra.ExactArgs(1),
	RunE: runClone,
}

func runClone(cmd *cobra.Command, args []string) error {
	if cloneDepth < 1 {
		return fmt.Errorf("--depth must be at least 1")
	}
	if err := applyProfile(cmd); err != nil {
		return err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get current directory: %w", err)
	}

	// Validate before touching the disk so a bad URL never leaves a temp dir.
	if err := gitclone.ValidateURL(args[0]); err != nil {
		return err
	}
	if err := gitclone.ValidateRef(cloneBranch); err != nil {
		return err
	}

	var repoDir string
	if cloneHere {
		entries, err := os.ReadDir(cwd)
		if err != nil {
			return fmt.Errorf("inspect current directory: %w", err)
		}
		if len(entries) != 0 {
			return fmt.Errorf("--here requires an empty current directory (%s)", cwd)
		}
		repoDir = cwd
	} else {
		tmp, err := gitclone.NewTemp("repo")
		if err != nil {
			return err
		}
		repoDir = tmp.Repo
		defer func() { _ = tmp.Remove() }()
	}

	// Interactive: git may prompt on this terminal (credentials, host keys).
	if err := gitclone.Clone(cmd.Context(), gitclone.Options{
		URL:         args[0],
		Branch:      cloneBranch,
		Depth:       cloneDepth,
		Dest:        repoDir,
		Interactive: true,
		Stdin:       cmd.InOrStdin(),
		Output:      cmd.ErrOrStderr(),
	}); err != nil {
		return err
	}

	cfg.RootDir = repoDir
	if cfg.OutputFile != "" && cfg.OutputFile != "-" && !filepath.IsAbs(cfg.OutputFile) {
		cfg.OutputFile = filepath.Join(cwd, cfg.OutputFile)
	}
	application, err := app.New(cfg)
	if err != nil {
		return err
	}
	defer application.Close()

	start := time.Now()
	files, skipped, err := application.Collect()
	if err != nil {
		return err
	}
	if err := application.Render(files, skipped, time.Since(start), nil); err != nil {
		return err
	}
	if err := recordDumpState(repoDir, files, "HEAD"); err != nil {
		application.LogError("Failed to record dump state: %v", err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Dump written to %s\n", cfg.OutputFile)
	return nil
}
