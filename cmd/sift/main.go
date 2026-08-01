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
func applyProfile(cmd *cobra.Command) error {
	name := cfg.Profile
	if name == "" {
		name, _ = config.DefaultProfileName()
	}
	if name == "" {
		return nil
	}

	profile, err := config.LoadProfile(name)
	if err != nil {
		return fmt.Errorf("load profile %q: %w", name, err)
	}
	profile.Apply(cfg, cmd.Flags())
	return nil
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
