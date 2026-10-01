package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/bethropolis/sift/internal/clipboard"
)

func init() { rootCmd.AddCommand(copyCmd) }

// copyCmd copies a rendered context document to the system clipboard.
var copyCmd = &cobra.Command{
	Use:   "copy [file]",
	Short: "Copy a context document to the clipboard",
	Long:  "Copy the generated codebase.md document to the clipboard so it can be pasted as text. An alternate file can be provided.",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := "codebase.md"
		if len(args) > 0 {
			path = args[0]
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read context document %q: %w", path, err)
		}
		if err := clipboard.Copy(data); err != nil {
			return fmt.Errorf("copy context document to clipboard: %w", err)
		}
		abs, _ := filepath.Abs(path)
		fmt.Fprintf(cmd.OutOrStdout(), "Copied %s to clipboard (%d bytes)\n", abs, len(data))
		return nil
	},
}
