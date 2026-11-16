package summary

import (
	"fmt"
	"io"
	"os"
	"sort"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-isatty"

	"github.com/bethropolis/sift/internal/utils"
	"github.com/bethropolis/sift/internal/walker"
)

// DumpStats contains metrics and metadata about a completed dump.
type DumpStats struct {
	FileCount         int
	FullCount         int
	SigCount          int
	SkippedCount      int
	TotalTokens       int64
	Budget            int
	Duration          time.Duration
	OutputFile        string
	OutputStyle       string
	IsClipboard       bool
	SecretCount       int
	FallbackMode      string
	FallbackThreshold float64
	FallbackApplied   bool
}

// PrintDumpSummary renders a human-friendly standard output summary of a dump operation.
func PrintDumpSummary(w io.Writer, stats DumpStats, useColors bool, quiet bool) {
	if quiet {
		return
	}

	durStr := stats.Duration.Round(time.Millisecond).String()
	if stats.Duration < time.Millisecond {
		durStr = fmt.Sprintf("%dµs", stats.Duration.Microseconds())
	}

	// Styles
	successStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#a6e3a1"))
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#cdd6f4"))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6c7086"))
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6c7086"))
	valStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#cdd6f4"))
	pathStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#89dceb"))
	warnStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#fab387"))
	alertStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#f38ba8"))

	if useColors {
		fmt.Fprintf(w, "%s %s %s\n\n",
			successStyle.Render("✓"),
			titleStyle.Render("Dump complete"),
			dimStyle.Render("in "+durStr),
		)
	} else {
		fmt.Fprintf(w, "✓ Dump complete in %s\n\n", durStr)
	}

	printRow := func(label, value string) {
		if useColors {
			fmt.Fprintf(w, "  %-14s %s\n", labelStyle.Render(label+":"), valStyle.Render(value))
		} else {
			fmt.Fprintf(w, "  %-14s %s\n", label+":", value)
		}
	}

	// 1. Output destination
	style := stats.OutputStyle
	if style == "" {
		style = "markdown"
	}
	if stats.IsClipboard {
		if useColors {
			fmt.Fprintf(w, "  %-14s %s %s\n",
				labelStyle.Render("Output:"),
				pathStyle.Render("Clipboard"),
				dimStyle.Render("("+style+")"),
			)
		} else {
			printRow("Output", fmt.Sprintf("Clipboard (%s)", style))
		}
	} else if stats.OutputFile == "" || stats.OutputFile == "-" {
		if useColors {
			fmt.Fprintf(w, "  %-14s %s %s\n",
				labelStyle.Render("Output:"),
				pathStyle.Render("stdout"),
				dimStyle.Render("("+style+")"),
			)
		} else {
			printRow("Output", fmt.Sprintf("stdout (%s)", style))
		}
	} else {
		if useColors {
			fmt.Fprintf(w, "  %-14s %s %s\n",
				labelStyle.Render("Output:"),
				pathStyle.Render(stats.OutputFile),
				dimStyle.Render("("+style+")"),
			)
		} else {
			printRow("Output", fmt.Sprintf("%s (%s)", stats.OutputFile, style))
		}
	}

	// 2. Files breakdown
	filesStr := fmt.Sprintf("%d included", stats.FileCount)
	if stats.SigCount > 0 {
		filesStr = fmt.Sprintf("%d included (%d full, %d signatures)", stats.FileCount, stats.FullCount, stats.SigCount)
	} else if stats.FileCount > 0 {
		filesStr = fmt.Sprintf("%d included (%d full)", stats.FileCount, stats.FullCount)
	}
	if stats.SkippedCount > 0 {
		filesStr += fmt.Sprintf(" · %d skipped", stats.SkippedCount)
	}
	printRow("Files", filesStr)

	// 3. Tokens & Budget
	tokensFormatted := formatTokenNumber(stats.TotalTokens)
	if stats.Budget > 0 {
		pct := float64(stats.TotalTokens) / float64(stats.Budget) * 100
		budgetFormatted := formatTokenNumber(int64(stats.Budget))
		pctStr := fmt.Sprintf("%.1f%%", pct)

		if useColors {
			pctColored := dimStyle.Render("(" + pctStr + ")")
			if pct >= 100 {
				pctColored = alertStyle.Render("(" + pctStr + ")")
			} else if pct >= 80 {
				pctColored = warnStyle.Render("(" + pctStr + ")")
			}
			fmt.Fprintf(w, "  %-14s %s / %s %s\n",
				labelStyle.Render("Tokens:"),
				valStyle.Render(tokensFormatted),
				dimStyle.Render(budgetFormatted),
				pctColored,
			)
		} else {
			printRow("Tokens", fmt.Sprintf("%s / %s (%s)", tokensFormatted, budgetFormatted, pctStr))
		}
	} else {
		if useColors {
			fmt.Fprintf(w, "  %-14s %s %s\n",
				labelStyle.Render("Tokens:"),
				valStyle.Render(tokensFormatted),
				dimStyle.Render("(unlimited)"),
			)
		} else {
			printRow("Tokens", fmt.Sprintf("%s (unlimited)", tokensFormatted))
		}
	}

	// 4. Secrets
	if stats.SecretCount > 0 {
		if useColors {
			fmt.Fprintf(w, "  %-14s %s\n",
				labelStyle.Render("Secrets:"),
				warnStyle.Render(fmt.Sprintf("⚠ %d redacted", stats.SecretCount)),
			)
		} else {
			printRow("Secrets", fmt.Sprintf("%d redacted", stats.SecretCount))
		}
	} else {
		if useColors {
			fmt.Fprintf(w, "  %-14s %s\n",
				labelStyle.Render("Secrets:"),
				dimStyle.Render("None detected"),
			)
		} else {
			printRow("Secrets", "None detected")
		}
	}

	// 5. Fallback details (if configured)
	if stats.FallbackMode != "" {
		threshStr := "100%"
		if stats.FallbackThreshold > 0 {
			threshStr = fmt.Sprintf("%.0f%%", stats.FallbackThreshold*100)
		}
		fallbackVal := fmt.Sprintf("%s (threshold: %s)", stats.FallbackMode, threshStr)
		if stats.FallbackApplied {
			fallbackVal += " [active]"
		}
		printRow("Fallback", fallbackVal)
	}
}

// DisplayResults maintains backward compatibility with legacy logging calls.
func DisplayResults(
	logger utils.Logger,
	fileCount int64,
	totalTokens int64,
	duration time.Duration,
	quiet bool,
) {
	if !quiet && logger != nil {
		logger.Info("Found and processed %d files.", fileCount)
		logger.Info("Total tokens: %d.", totalTokens)
		logger.Info("Scan complete in %v.", duration.Round(time.Millisecond))
	}
}

// DisplaySkippedItems formats and displays skipped items.
func DisplaySkippedItems(
	logger utils.Logger,
	skippedItems []walker.SkippedItem,
	output io.Writer,
	quiet bool,
) {
	if quiet {
		return
	}

	useColor := isTerminalWriter(output) && os.Getenv("NO_COLOR") == ""
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#f9e2af"))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6c7086"))
	pathStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#cdd6f4"))

	if useColor {
		fmt.Fprintf(output, "\n%s %s\n", headerStyle.Render("--- Skipped Items"), dimStyle.Render(fmt.Sprintf("(%d) ---", len(skippedItems))))
	} else {
		fmt.Fprintf(output, "\n--- Skipped Items (%d) ---\n", len(skippedItems))
	}

	if len(skippedItems) > 0 {
		sort.Slice(skippedItems, func(i, j int) bool {
			return skippedItems[i].Path < skippedItems[j].Path
		})
		for _, item := range skippedItems {
			typeStr := "FILE"
			if item.IsDir {
				typeStr = "DIR "
			}
			if useColor {
				fmt.Fprintf(output, "  %s  %-50s %s\n",
					dimStyle.Render(typeStr),
					pathStyle.Render(item.Path),
					dimStyle.Render(fmt.Sprintf("[%s]", item.Reason)),
				)
			} else {
				fmt.Fprintf(output, "  %s  %-50s [%s]\n",
					typeStr,
					item.Path,
					item.Reason,
				)
			}
		}
	} else {
		fmt.Fprintln(output, "  No items were skipped.")
	}

	if useColor {
		fmt.Fprintf(output, "%s\n", dimStyle.Render("--- End Skipped Items ---"))
	} else {
		fmt.Fprintln(output, "--- End Skipped Items ---")
	}
}

func isTerminalWriter(w io.Writer) bool {
	if f, ok := w.(*os.File); ok {
		return isatty.IsTerminal(f.Fd()) || isatty.IsCygwinTerminal(f.Fd())
	}
	return false
}

func formatTokenNumber(n int64) string {
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
