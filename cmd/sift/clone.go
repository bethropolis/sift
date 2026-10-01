package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/bethropolis/sift/internal/app"
	"github.com/bethropolis/sift/internal/config"
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
dump. Use --here to keep the checkout in the current directory.`,
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

	var repoDir string
	var cleanup func()
	if cloneHere {
		entries, err := os.ReadDir(cwd)
		if err != nil {
			return fmt.Errorf("inspect current directory: %w", err)
		}
		if len(entries) != 0 {
			return fmt.Errorf("--here requires an empty current directory (%s)", cwd)
		}
		repoDir = cwd
		cleanup = func() {}
	} else {
		tempDir, err := os.MkdirTemp("", "sift-clone-")
		if err != nil {
			return fmt.Errorf("create temporary clone directory: %w", err)
		}
		repoDir = filepath.Join(tempDir, "repo")
		cleanup = func() { _ = os.RemoveAll(tempDir) }
	}
	defer cleanup()

	gitArgs := []string{"clone", fmt.Sprintf("--depth=%d", cloneDepth), "--single-branch"}
	if cloneBranch != "" {
		gitArgs = append(gitArgs, "--branch", cloneBranch)
	}
	gitArgs = append(gitArgs, "--", args[0], repoDir)
	clone := exec.Command("git", gitArgs...)
	clone.Stdin = cmd.InOrStdin()
	clone.Stdout = cmd.ErrOrStderr()
	clone.Stderr = cmd.ErrOrStderr()
	if err := clone.Run(); err != nil {
		return fmt.Errorf("git clone: %w", err)
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
