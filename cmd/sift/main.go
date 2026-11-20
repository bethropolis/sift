// Command sift packs a codebase into an LLM-friendly context document.
package main

import (
	"fmt"
	"os"

	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"

	"github.com/bethropolis/sift/internal/config"
)

// newRootCmd constructs a complete CLI with its own configuration state.
// Tests create independent roots so flags and profiles cannot leak between
// invocations, and commands are wired via constructors instead of init().
func newRootCmd() *cobra.Command {
	cfg := config.New()
	showAllFlags := false

	root := &cobra.Command{
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
			return runPick(cfg, cmd, args)
		},
	}

	// Shared flags on the root so "sift --budget 60000" configures the picker
	// exactly like "sift pick --budget 60000". Subcommands register their own
	// copies in their constructors.
	config.RegisterFlags(cfg, root.Flags())
	root.PersistentFlags().BoolVar(&showAllFlags, "all", false, "Show all available flags and options in help")

	root.AddCommand(
		newCheckCmd(cfg),
		newConfigCmd(cfg),
		newCloneCmd(cfg),
		newCompletionCmd(),
		newCopyCmd(),
		newDeltaCmd(cfg),
		newDesktopCmd(cfg),
		newDiffCmd(cfg),
		newDumpCmd(cfg),
		newFollowCmd(cfg),
		newMCPCmd(cfg),
		newPickCmd(cfg),
		newSelectCmd(cfg),
		newServeCmd(cfg),
		newSkillCmd(),
		newVersionCmd(cfg),
		newWatchCmd(cfg),
	)
	setupCustomHelp(root, cfg, &showAllFlags)
	return root
}

// applyProfile resolves the project/global config and the selected profile
// without overriding flags the user set explicitly.
func applyProfile(cfg *config.Config, cmd *cobra.Command) error {
	opts := []config.ResolveOption{}
	if cfg.Target != "" {
		opts = append(opts, config.WithTarget(cfg.Target))
	}
	return config.ResolveConfig(cfg, cmd.Flags(), opts...)
}

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
