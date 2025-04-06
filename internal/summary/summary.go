// Package summary handles display of scan results and statistics
package summary

import (
	"fmt"
	"io"
	"sort"
	"time"

	"github.com/bethropolis/dir-dumper/internal/walker"
)

// Logger defines the minimal logging interface required
type Logger interface {
	Info(format string, args ...interface{})
}

// DisplayResults shows the end results of a scan operation
func DisplayResults(
	logger Logger,
	fileCount int64,
	duration time.Duration,
	quiet bool,
) {
	if !quiet {
		logger.Info("Found and processed %d files.", fileCount)
		logger.Info("Scan complete in %v.", duration.Round(time.Millisecond))
	}
}

// DisplaySkippedItems formats and prints information about skipped items
func DisplaySkippedItems(
	logger Logger,
	skippedItems []walker.SkippedItem,
	output io.Writer,
	quiet bool,
) {
	infoLog := func(format string, args ...interface{}) {
		if !quiet {
			logger.Info(format, args...)
		}
	}

	infoLog("--- Skipped Items (%d) ---", len(skippedItems))
	if len(skippedItems) > 0 {
		// Sort for consistent output
		sort.Slice(skippedItems, func(i, j int) bool {
			return skippedItems[i].Path < skippedItems[j].Path
		})
		for _, item := range skippedItems {
			typeStr := "FILE"
			if item.IsDir {
				typeStr = "DIR " // Add space for alignment
			}
			// Print to stderr
			fmt.Fprintf(output, "Skipped %s: %-.*s [%s]\n",
				typeStr,
				50, // Max width for path column
				item.Path,
				item.Reason,
			)
		}
	} else {
		infoLog("No items were skipped.")
	}
	infoLog("--- End Skipped Items ---")
}
