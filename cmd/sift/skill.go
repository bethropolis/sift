package main

import (
	"fmt"
	"os"
	"unicode"
	"unicode/utf8"

	"github.com/spf13/cobra"

	"github.com/bethropolis/sift/internal/skills"
	embedded "github.com/bethropolis/sift/skills"
)

var (
	skillLocalFlag  bool
	skillForceFlag  bool
	skillDryRunFlag bool
)

const defaultSkillName = "sift-project-dotfile"

func init() {
	rootCmd.AddCommand(skillCmd)
	skillCmd.AddCommand(skillInstallCmd)
	skillCmd.AddCommand(skillStatusCmd)
	skillCmd.AddCommand(skillUninstallCmd)
	skillCmd.AddCommand(skillShowCmd)

	skillInstallCmd.Flags().BoolVar(&skillLocalFlag, "local", false, "Install to the current project directory (.agents/skills)")
	skillInstallCmd.Flags().BoolVar(&skillForceFlag, "force", false, "Force overwrite existing skill files even if up to date")
	skillInstallCmd.Flags().BoolVar(&skillDryRunFlag, "dry-run", false, "Print actions without creating or modifying files")

	skillUninstallCmd.Flags().BoolVar(&skillLocalFlag, "local", false, "Uninstall from the current project directory (.agents/skills)")
}

var skillCmd = &cobra.Command{
	Use:   "skill",
	Short: "Manage Sift agent skills for AI coding tools",
	Long: `Manage Sift agent skills for AI coding tools.

Installs, updates, or checks Sift skills (such as sift-project-dotfile) for
supported AI agents: OpenCode, Antigravity, Claude Code, Codex, and Standard Agents.
Skills are only installed for programs that are already installed on your machine.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

var skillInstallCmd = &cobra.Command{
	Use:   "install [target...]",
	Short: "Install the Sift skill into detected AI coding tools",
	Long: `Install the Sift skill into detected AI coding tools.

By default, sift inspects your environment and installs the skill only into
programs whose parent configuration directories exist (e.g. ~/.config/opencode for OpenCode,
~/.gemini or ~/.config/antigravity for Antigravity, ~/.claude for Claude Code, ~/.codex for Codex).

Use --local to install into the current project workspace (.agents/skills).`,
	Example: `  sift skill install
  sift skill install opencode
  sift skill install claude antigravity
  sift skill install --local
  sift skill install --force`,
	RunE: func(cmd *cobra.Command, args []string) error {
		mgr := skills.NewManager()
		out := cmd.OutOrStdout()

		if skillLocalFlag {
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("get current directory: %w", err)
			}
			if skillDryRunFlag {
				st := mgr.CheckLocal(cwd, defaultSkillName)
				fmt.Fprintf(out, "[dry-run] would install %s to %s\n", defaultSkillName, st.SkillFile)
				return nil
			}
			res, err := mgr.InstallLocal(cwd, defaultSkillName, skillForceFlag)
			if err != nil {
				return err
			}
			fmt.Fprintf(out, "%s %s skill at %s\n", capitalize(res.Action), res.DisplayName, res.Path)
			return nil
		}

		if len(args) > 0 {
			for _, targetName := range args {
				target, ok := mgr.ResolveTarget(targetName)
				if !ok {
					return fmt.Errorf("unknown target %q (available: opencode, antigravity, claude, codex, agents)", targetName)
				}
				if skillDryRunFlag {
					st := mgr.CheckTarget(target, defaultSkillName)
					if !st.Detected {
						fmt.Fprintf(out, "[dry-run] target %s not detected; would skip\n", target.DisplayName)
					} else {
						fmt.Fprintf(out, "[dry-run] would install %s to %s\n", defaultSkillName, st.SkillFile)
					}
					continue
				}
				res, err := mgr.Install(target, defaultSkillName, skillForceFlag)
				if err != nil {
					return err
				}
				fmt.Fprintf(out, "%s %s skill at %s\n", capitalize(res.Action), res.DisplayName, res.Path)
			}
			return nil
		}

		// Auto-detect installed agents
		statuses := mgr.StatusAll(defaultSkillName)
		var detected []skills.TargetStatus
		for _, st := range statuses {
			if st.Detected {
				detected = append(detected, st)
			}
		}

		if len(detected) == 0 {
			fmt.Fprintln(out, "No supported AI agent parent directories detected on this machine.")
			fmt.Fprintln(out, "Use 'sift skill install --local' to install into the current project (.agents/skills).")
			return nil
		}

		if skillDryRunFlag {
			for _, st := range detected {
				fmt.Fprintf(out, "[dry-run] would install %s to %s (%s)\n", defaultSkillName, st.SkillFile, st.Target.DisplayName)
			}
			return nil
		}

		results, err := mgr.InstallDetected(defaultSkillName, skillForceFlag)
		if err != nil {
			return err
		}

		for _, res := range results {
			fmt.Fprintf(out, "%s %s skill: %s\n", capitalize(res.Action), res.DisplayName, res.Path)
		}
		return nil
	},
}

var skillStatusCmd = &cobra.Command{
	Use:     "status",
	Aliases: []string{"list", "check"},
	Short:   "Check which AI coding tools are detected and skill status",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSkillStatus(cmd, args)
	},
}

func runSkillStatus(cmd *cobra.Command, _ []string) error {
	mgr := skills.NewManager()
	out := cmd.OutOrStdout()

	fmt.Fprintf(out, "Sift Skill: %s\n\n", defaultSkillName)
	fmt.Fprintf(out, "%-18s %-16s %-22s %s\n", "TARGET", "PROGRAM", "SKILL STATUS", "PATH")
	fmt.Fprintf(out, "%-18s %-16s %-22s %s\n", "------", "-------", "------------", "----")

	for _, st := range mgr.StatusAll(defaultSkillName) {
		programStatus := "not detected"
		if st.Detected {
			programStatus = "installed"
		}

		skillStatus := "not installed"
		if !st.Detected {
			skillStatus = "-"
		} else if st.Installed {
			if st.UpToDate {
				skillStatus = "installed (current)"
			} else {
				skillStatus = "installed (outdated)"
			}
		}

		path := "-"
		if st.Detected {
			path = st.SkillFile
		}

		fmt.Fprintf(out, "%-18s %-16s %-22s %s\n", st.Target.DisplayName, programStatus, skillStatus, path)
	}

	// Also show current workspace status if inside a directory
	cwd, err := os.Getwd()
	if err == nil {
		localSt := mgr.CheckLocal(cwd, defaultSkillName)
		localSkillStatus := "not installed"
		if localSt.Installed {
			if localSt.UpToDate {
				localSkillStatus = "installed (current)"
			} else {
				localSkillStatus = "installed (outdated)"
			}
		}
		fmt.Fprintf(out, "%-18s %-16s %-22s %s\n", "Local Project", "workspace", localSkillStatus, localSt.SkillFile)
	}

	return nil
}

var skillUninstallCmd = &cobra.Command{
	Use:   "uninstall [target...]",
	Short: "Uninstall the Sift skill from AI coding tools",
	RunE: func(cmd *cobra.Command, args []string) error {
		mgr := skills.NewManager()
		out := cmd.OutOrStdout()

		if skillLocalFlag {
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("get current directory: %w", err)
			}
			removed, err := mgr.UninstallLocal(cwd, defaultSkillName)
			if err != nil {
				return err
			}
			if removed {
				fmt.Fprintln(out, "Uninstalled skill from local workspace (.agents/skills)")
			} else {
				fmt.Fprintln(out, "Skill was not installed in local workspace")
			}
			return nil
		}

		if len(args) > 0 {
			for _, targetName := range args {
				target, ok := mgr.ResolveTarget(targetName)
				if !ok {
					return fmt.Errorf("unknown target %q", targetName)
				}
				removed, err := mgr.Uninstall(target, defaultSkillName)
				if err != nil {
					return err
				}
				if removed {
					fmt.Fprintf(out, "Uninstalled %s skill\n", target.DisplayName)
				} else {
					fmt.Fprintf(out, "%s skill was not installed\n", target.DisplayName)
				}
			}
			return nil
		}

		removed, err := mgr.UninstallAll(defaultSkillName)
		if err != nil {
			return err
		}
		if len(removed) == 0 {
			fmt.Fprintln(out, "No installed Sift skills found to uninstall.")
			return nil
		}
		for _, name := range removed {
			fmt.Fprintf(out, "Uninstalled %s skill\n", name)
		}
		return nil
	},
}

var skillShowCmd = &cobra.Command{
	Use:   "show [name]",
	Short: "Display the content of a bundled skill",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := defaultSkillName
		if len(args) > 0 {
			name = args[0]
		}

		content, err := embedded.ReadSkill(name)
		if err != nil {
			return err
		}
		_, err = cmd.OutOrStdout().Write(content)
		return err
	},
}

// capitalize upper-cases the first rune, replacing the deprecated
// strings.Title for the short status words printed by skill commands.
func capitalize(s string) string {
	if s == "" {
		return s
	}
	r, size := utf8.DecodeRuneInString(s)
	return string(unicode.ToUpper(r)) + s[size:]
}
