package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// newCompletionCmd prints shell completion scripts to stdout.
func newCompletionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "completion [bash|zsh|fish|powershell]",
		Short: "Generate shell completion scripts",
		Long: `Generate a shell completion script and print it to standard output.

To load completions in your current session:

  Bash:        source <(sift completion bash)
  Zsh:         source <(sift completion zsh)
  Fish:        sift completion fish | source
  PowerShell:  sift completion powershell | Out-String | Invoke-Expression

To install them persistently, write the output to your shell's completion
directory (the sift installers do this automatically when possible).`,
		Args:      cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		ValidArgs: []string{"bash", "zsh", "fish", "powershell"},
		RunE: func(cmd *cobra.Command, args []string) error {
			root := cmd.Root()
			switch args[0] {
			case "bash":
				return root.GenBashCompletion(os.Stdout)
			case "zsh":
				return root.GenZshCompletion(os.Stdout)
			case "fish":
				return root.GenFishCompletion(os.Stdout, true)
			case "powershell":
				return root.GenPowerShellCompletionWithDesc(os.Stdout)
			default:
				return fmt.Errorf("unsupported shell %q (choose bash, zsh, fish, or powershell)", args[0])
			}
		},
	}
}
