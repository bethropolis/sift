// Command sift packs a codebase into an LLM-friendly context document.
package main

import (
	"fmt"
	"os"

	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"

	"github.com/bethropolis/sift/internal/config"
)

// cfg holds the flags shared across commands. Flags are bound to it in init,
// and cobra populates it during parsing.
var cfg = config.New()

var rootCmd = &cobra.Command{
	Use:   "sift",
	Short: "Sift a codebase into an LLM-friendly context document",
	Long: `Sift a codebase into an LLM-friendly context document.

Running sift with no subcommand launches the interactive file picker.
Use "sift dump" for a one-shot scan of the current directory.`,
	SilenceUsage:  true,
	SilenceErrors: true,
	Version:       cfg.Version,
	// Bare invocation (no subcommand) launches the picker TUI. Outside a
	// terminal the help text is shown instead so piping stays safe.
	RunE: func(cmd *cobra.Command, args []string) error {
		if !isatty.IsTerminal(os.Stdin.Fd()) {
			return cmd.Help()
		}
		return runPick(cmd, args)
	},
}

// applyProfile loads the selected profile (or the default profile) and
// overlays it onto cfg, without overriding flags the user set explicitly.
// It now delegates to config.ResolveConfig which also handles [sift] defaults,
// target selection, extends, and top-level scoring.
func applyProfile(cmd *cobra.Command) error {
	opts := []config.ResolveOption{}
	if cfg.Target != "" {
		opts = append(opts, config.WithTarget(cfg.Target))
	}
	if err := config.ResolveConfig(cfg, cmd.Flags(), opts...); err != nil {
		return err
	}
	return nil
}

// init wires the shared config flags (--budget, --dir, --style, ...) onto the
// root command so a bare invocation like "sift --budget 60000" launches the
// picker with the same options as "sift pick". Subcommands register their own
// copies via config.RegisterFlags.
func init() {
	config.RegisterFlags(cfg, rootCmd.Flags())
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
