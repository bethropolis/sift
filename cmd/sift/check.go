package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/bethropolis/sift/internal/config"
)

// newCheckCmd validates .sift.toml configuration syntax and semantics.
func newCheckCmd(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "check [path]",
		Aliases: []string{"validate"},
		Short:   "Validate .sift.toml configuration file syntax and semantics",
		Long: `Validate a .sift.toml configuration file for syntax, profile inheritance,
mode mappings, token budgets, and safety constraints.

If path is omitted, checks .sift.toml in the current directory (or --config if set).
If path is a directory, checks .sift.toml in that directory.
If path is a file, checks that specific file directly.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCheck(cfg, cmd, args)
		},
	}
	cmd.Flags().StringVar(&cfg.ConfigFile, "config", "", "Path to custom configuration file")
	cmd.Flags().BoolVarP(&cfg.Quiet, "quiet", "q", false, "Do not print output; exit code indicates validity")
	return cmd
}

// newConfigCmd groups configuration inspection commands.
func newConfigCmd(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Inspect and manage Sift configuration",
		Long:  "Inspect, validate, and manage Sift configuration files (.sift.toml).",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	check := &cobra.Command{
		Use:     "check [path]",
		Aliases: []string{"validate"},
		Short:   "Validate .sift.toml configuration file syntax and semantics",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCheck(cfg, cmd, args)
		},
	}
	check.Flags().StringVar(&cfg.ConfigFile, "config", "", "Path to custom configuration file")
	check.Flags().BoolVarP(&cfg.Quiet, "quiet", "q", false, "Do not print output; exit code indicates validity")
	cmd.AddCommand(check)
	return cmd
}

func runCheck(cfg *config.Config, cmd *cobra.Command, args []string) error {
	target := ""
	if len(args) > 0 {
		target = args[0]
	} else if cfg.ConfigFile != "" {
		target = cfg.ConfigFile
	}

	rootDir := cfg.RootDir
	if rootDir == "" {
		rootDir = "."
	}

	result, err := config.ValidateConfigFile(target, rootDir)
	if err != nil {
		if !cfg.Quiet {
			fmt.Fprintf(os.Stderr, "✗ Error: %v\n", err)
		}
		return err
	}

	if cfg.Quiet {
		if !result.Valid {
			return fmt.Errorf("configuration invalid")
		}
		return nil
	}

	w := os.Stdout
	useColor := isTerminalWriter(w) && os.Getenv("NO_COLOR") == "" && !cfg.NoColor

	// Styles
	successStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#a6e3a1"))
	errorStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#f38ba8"))
	warnStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#fab387"))
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6c7086"))
	valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#cdd6f4"))
	pathStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#89dceb"))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6c7086"))

	relPath, relErr := filepath.Rel(rootDir, result.Path)
	displayPath := result.Path
	if relErr == nil && !strings.HasPrefix(relPath, "..") {
		displayPath = relPath
	}

	if result.Valid {
		if useColor {
			fmt.Fprintf(w, "%s %s %s\n\n",
				successStyle.Render("✓"),
				pathStyle.Render(displayPath),
				successStyle.Render("is valid"),
			)
		} else {
			fmt.Fprintf(w, "✓ %s is valid\n\n", displayPath)
		}

		// Print Summary
		sum := result.Summary
		printRow := func(label, value string) {
			if useColor {
				fmt.Fprintf(w, "  %-18s %s\n", labelStyle.Render(label+":"), valueStyle.Render(value))
			} else {
				fmt.Fprintf(w, "  %-18s %s\n", label+":", value)
			}
		}

		if sum.DefaultProfile != "" {
			printRow("Default Profile", sum.DefaultProfile)
		}
		if len(sum.Profiles) > 0 {
			printRow("Profiles", fmt.Sprintf("%s (%d defined)", strings.Join(sum.Profiles, ", "), len(sum.Profiles)))
		}
		if len(sum.Targets) > 0 {
			printRow("Targets", fmt.Sprintf("%s (%d defined)", strings.Join(sum.Targets, ", "), len(sum.Targets)))
		}
		if sum.Modes != nil {
			var modeParts []string
			if sum.Modes.FullCount > 0 {
				modeParts = append(modeParts, fmt.Sprintf("%d full", sum.Modes.FullCount))
			}
			if sum.Modes.SigsCount > 0 {
				modeParts = append(modeParts, fmt.Sprintf("%d signatures", sum.Modes.SigsCount))
			}
			if sum.Modes.SkipCount > 0 {
				modeParts = append(modeParts, fmt.Sprintf("%d skip", sum.Modes.SkipCount))
			}
			if len(modeParts) > 0 {
				printRow("Modes Mapping", strings.Join(modeParts, ", "))
			}
		}
		if sum.FallbackMode != "" {
			thresh := "100%"
			if sum.FallbackThreshold != nil {
				thresh = fmt.Sprintf("%.0f%%", *sum.FallbackThreshold*100)
			}
			printRow("Fallback Mode", fmt.Sprintf("%s (threshold: %s)", sum.FallbackMode, thresh))
		}
		if sum.Budget > 0 {
			printRow("Token Budget", formatTokens(int64(sum.Budget)))
		}
		if sum.Style != "" || sum.Output != "" {
			styleStr := sum.Style
			if styleStr == "" {
				styleStr = config.New().Style
			}
			outStr := sum.Output
			if outStr == "" {
				outStr = config.DefaultOutputFile(styleStr)
			}
			printRow("Output", fmt.Sprintf("%s (%s)", outStr, styleStr))
		}
		if sum.IgnoreCount > 0 {
			printRow("Ignore Patterns", fmt.Sprintf("%d rules", sum.IgnoreCount))
		}

		// Print any warnings
		if len(result.Warnings()) > 0 {
			fmt.Fprintln(w)
			if useColor {
				fmt.Fprintf(w, "%s %s:\n", warnStyle.Render("⚠"), warnStyle.Render(fmt.Sprintf("Warnings (%d)", len(result.Warnings()))))
			} else {
				fmt.Fprintf(w, "⚠ Warnings (%d):\n", len(result.Warnings()))
			}
			for _, warn := range result.Warnings() {
				msg := warn.Message
				if warn.Field != "" {
					msg = fmt.Sprintf("%s: %s", warn.Field, warn.Message)
				}
				if useColor {
					fmt.Fprintf(w, "  %s %s\n", dimStyle.Render("•"), valueStyle.Render(msg))
				} else {
					fmt.Fprintf(w, "  • %s\n", msg)
				}
			}
		}
		return nil
	}

	// Invalid
	if useColor {
		fmt.Fprintf(w, "%s %s %s:\n\n",
			errorStyle.Render("✗"),
			pathStyle.Render(displayPath),
			errorStyle.Render(fmt.Sprintf("is invalid (%d error(s))", len(result.Errors()))),
		)
	} else {
		fmt.Fprintf(w, "✗ %s is invalid (%d error(s)):\n\n", displayPath, len(result.Errors()))
	}

	for _, issue := range result.Errors() {
		msg := issue.Message
		if issue.Field != "" {
			msg = fmt.Sprintf("[%s] %s", issue.Field, issue.Message)
		}
		if useColor {
			fmt.Fprintf(w, "  %s %s\n", errorStyle.Render("•"), valueStyle.Render(msg))
		} else {
			fmt.Fprintf(w, "  • %s\n", msg)
		}
	}

	if len(result.Warnings()) > 0 {
		fmt.Fprintln(w)
		for _, warn := range result.Warnings() {
			msg := warn.Message
			if warn.Field != "" {
				msg = fmt.Sprintf("[%s] %s", warn.Field, warn.Message)
			}
			if useColor {
				fmt.Fprintf(w, "  %s %s\n", warnStyle.Render("•"), dimStyle.Render(msg))
			} else {
				fmt.Fprintf(w, "  • [warning] %s\n", msg)
			}
		}
	}

	return fmt.Errorf("configuration validation failed with %d error(s)", len(result.Errors()))
}

// formatTokens formats an integer token count with commas (e.g. 150000 -> "150,000").
func formatTokens(n int64) string {
	in := fmt.Sprintf("%d", n)
	if len(in) <= 3 {
		return in
	}
	var out []byte
	rem := len(in) % 3
	if rem > 0 {
		out = append(out, in[:rem]...)
		if len(in) > rem {
			out = append(out, ',')
		}
	}
	for i := rem; i < len(in); i += 3 {
		out = append(out, in[i:i+3]...)
		if i+3 < len(in) {
			out = append(out, ',')
		}
	}
	return string(out)
}
