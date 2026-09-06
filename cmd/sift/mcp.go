package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/bethropolis/sift/internal/config"
	"github.com/bethropolis/sift/internal/mcp"
	"github.com/bethropolis/sift/internal/state"
)

func init() {
	rootCmd.AddCommand(mcpCmd)
	config.RegisterFlags(cfg, mcpCmd.Flags())
}

// mcpCmd starts a stateless MCP server over stdio for coding agents: three
// tools (pack_context, pack_diff, list_tree), hand-rolled JSON-RPC, no new
// dependencies. It never reads or writes the delta baseline.
var mcpCmd = &cobra.Command{
	Use:   "mcp [path]",
	Short: "Start an MCP server over stdio",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 {
			cfg.RootDir = args[0]
		}

		if err := applyProfile(cmd); err != nil {
			return err
		}

		absRoot, err := state.CanonicalRoot(cfg.RootDir)
		if err != nil {
			return err
		}
		cfg.RootDir = absRoot
		cfg.Quiet = true // keep stdout pure JSON-RPC; the logger writes to stderr

		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		return mcp.Run(ctx, os.Stdin, os.Stdout, cfg)
	},
}
