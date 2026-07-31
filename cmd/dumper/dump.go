package main

import (
	"github.com/spf13/cobra"

	"github.com/bethropolis/dir-dumper/internal/app"
	"github.com/bethropolis/dir-dumper/internal/config"
)

func init() {
	rootCmd.AddCommand(dumpCmd)
	config.RegisterFlags(cfg, dumpCmd.Flags())
}

// dumpCmd scans a directory and renders its contents.
var dumpCmd = &cobra.Command{
	Use:   "dump [path]",
	Short: "Scan a directory and render its contents",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 {
			cfg.RootDir = args[0]
		}

		if err := applyProfile(cmd); err != nil {
			return err
		}

		application := app.New(cfg)
		defer application.Close()
		return application.Run()
	},
}
