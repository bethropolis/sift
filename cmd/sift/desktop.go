package main

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/bethropolis/sift/internal/desktop"
)

func init() {
	rootCmd.AddCommand(desktopCmd)
	desktopCmd.AddCommand(desktopInstallCmd)
	desktopCmd.AddCommand(desktopUninstallCmd)
	desktopCmd.AddCommand(desktopStatusCmd)
}

var desktopCmd = &cobra.Command{
	Use:   "desktop",
	Short: "Install or remove the Sift desktop launcher",
	Long: `Manage the Sift desktop launcher, which opens "sift serve --app".

Linux writes ~/.local/share/applications/sift.desktop plus an icon, then
refreshes the desktop database. macOS builds ~/Applications/Sift.app (the
icon needs sips, which ships with macOS; without it the bundle installs
iconless). First launch on macOS may show a Gatekeeper prompt for the
unsigned bundle — allow it in System Settings and it will not ask again.

Everything is user-local and marked, so uninstall only removes files sift
created and never touches the binary, config, or state.`,
}

var desktopInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install the Sift desktop launcher",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		rep, err := desktop.Install(cfg.Version)
		if err != nil {
			return err
		}
		for _, f := range rep.Files {
			cmd.Printf("installed %s\n", f)
		}
		if !rep.Icon && runtime.GOOS == "darwin" {
			cmd.Println("note: installed without an icon (sips not found)")
		}
		return nil
	},
}

var desktopUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove the Sift desktop launcher",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		rep, err := desktop.Uninstall()
		if err != nil {
			return err
		}
		if len(rep.Files) == 0 {
			cmd.Println("nothing installed")
			return nil
		}
		for _, f := range rep.Files {
			cmd.Printf("removed %s\n", f)
		}
		return nil
	},
}

var desktopStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show whether the Sift desktop launcher is installed",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		rep, err := desktop.Status()
		if err != nil {
			return err
		}
		if len(rep.Files) == 0 {
			cmd.Println("not installed")
			return nil
		}
		for _, f := range rep.Files {
			cmd.Printf("installed %s\n", f)
		}
		if !rep.Icon {
			fmt.Fprintln(cmd.OutOrStdout(), "icon: missing")
		}
		return nil
	},
}
