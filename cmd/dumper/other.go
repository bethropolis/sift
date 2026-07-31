package main

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/bethropolis/dir-dumper/internal/config"
)

func init() {
	rootCmd.AddCommand(pickCmd, watchCmd)
	config.RegisterFlags(cfg, pickCmd.Flags())
	config.RegisterFlags(cfg, watchCmd.Flags())
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

// watchCmd re-renders on file changes (Phase 4).
var watchCmd = &cobra.Command{
	Use:   "watch [path]",
	Short: "Watch a directory and re-render on changes",
	RunE: func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("dumper watch: %w", errNotImplemented)
	},
}
