// Command dumper packs a codebase into an LLM-friendly context document.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/bethropolis/dir-dumper/internal/config"
)

// cfg holds the flags shared across commands. Flags are bound to it in init,
// and cobra populates it during parsing.
var cfg = config.New()

var rootCmd = &cobra.Command{
	Use:           "dumper",
	Short:         "Pack a codebase into an LLM-friendly context document",
	SilenceUsage:  true,
	SilenceErrors: true,
	Version:       cfg.Version,
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
