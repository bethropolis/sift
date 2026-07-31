package main

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(pickCmd, diffCmd, watchCmd)
}

var errNotImplemented = errors.New("not yet implemented")

// pickCmd launches the interactive TUI (Phase 4).
var pickCmd = &cobra.Command{
	Use:   "pick [path]",
	Short: "Interactively select files and render a context document",
	RunE: func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("dumper pick: %w", errNotImplemented)
	},
}

// diffCmd renders a context document for files changed between git refs (Phase 3).
var diffCmd = &cobra.Command{
	Use:   "diff [ref]",
	Short: "Dump context for files changed by a git ref",
	RunE: func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("dumper diff: %w", errNotImplemented)
	},
}

// watchCmd re-renders on file changes (Phase 4).
var watchCmd = &cobra.Command{
	Use:   "watch [path]",
	Short: "Watch a directory and re-render on changes",
	RunE: func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("dumper watch: %w", errNotImplemented)
	},
}
