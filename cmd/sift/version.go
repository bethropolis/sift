package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/bethropolis/sift/internal/config"
)

// newVersionCmd prints the program version.
func newVersionCmd(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version number",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("sift version %s\n", cfg.Version)
		},
	}
}
