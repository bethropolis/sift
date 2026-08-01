// Package setup provides initialization and configuration functions
package setup

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/bethropolis/sift/internal/ignore"
	"github.com/bethropolis/sift/internal/utils"
	"github.com/bethropolis/sift/internal/walker"
)

// InfoLogger wraps the Info method for status updates
type InfoLogger func(format string, args ...interface{})

// WalkerConfig holds all parameters needed to configure a directory walker
type WalkerConfig struct {
	RootDir       string
	Concurrent    bool
	MaxWorkers    int
	MaxFileSizeMB int64
	Extensions    string
	IgnoreHidden  bool
	IgnoreGit     bool
	CustomIgnore  string
	IncludeBinary bool
	ShowProgress  bool
	Ctx           context.Context
	Quiet         bool
	Logger        utils.Logger
}

// ConfigureWalker sets up an ignore matcher and walker options based on the config
func ConfigureWalker(cfg WalkerConfig, infoLog InfoLogger) (
	*ignore.IgnoreMatcher,
	[]walker.Option,
	error,
) {
	// --- Parse custom ignore patterns ---
	var customPatterns []string
	if cfg.CustomIgnore != "" {
		customPatterns = strings.Split(cfg.CustomIgnore, ",")
		for i, pattern := range customPatterns {
			customPatterns[i] = strings.TrimSpace(pattern) // Trim whitespace
		}
		infoLog("Using custom ignore patterns: %v", customPatterns)
	}

	// --- Parse file extensions ---
	var fileExtensions map[string]struct{}
	if cfg.Extensions != "" {
		fileExtensions = make(map[string]struct{})
		extList := strings.Split(cfg.Extensions, ",")
		var addedExts []string
		for _, ext := range extList {
			cleanExt := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(ext, ".")))
			if cleanExt != "" {
				fileExtensions[cleanExt] = struct{}{}
				addedExts = append(addedExts, "."+cleanExt)
			}
		}
		infoLog("Filtering enabled. Only including extensions: %s", strings.Join(addedExts, ", "))
	} else {
		infoLog("No extension filtering (including all file types).")
	}

	// Print effective settings
	if cfg.IgnoreHidden {
		infoLog("Ignoring hidden files/directories (starting with '.').")
	} else {
		infoLog("Including hidden files/directories.")
	}

	// --- Initialize ignore matcher ---
	ignoreOptions := []ignore.Option{
		ignore.WithLogger(cfg.Logger),
		ignore.WithHiddenIgnore(cfg.IgnoreHidden),
		ignore.WithGitIgnore(cfg.IgnoreGit),
	}
	if len(customPatterns) > 0 {
		ignoreOptions = append(ignoreOptions, ignore.WithCustomRules(customPatterns))
	}

	matcher, err := ignore.New(cfg.RootDir, ignoreOptions...)
	if err != nil {
		return nil, nil, fmt.Errorf("error initializing ignore rules: %w", err)
	}

	// --- Set up walk options ---
	var walkOptions []walker.Option

	// Add the options correctly
	walkOptions = append(walkOptions,
		walker.WithLogger(cfg.Logger),
		walker.WithConcurrency(cfg.Concurrent),
		walker.WithMaxWorkers(cfg.MaxWorkers),
	)

	// Add progress option if enabled
	if cfg.ShowProgress {
		cfg.Logger.Debug("Progress display enabled")

		// Create a progress handler
		walkOptions = append(walkOptions, walker.WithProgress(func(stats walker.ProgressStats) {
			// Only print to stderr to avoid interfering with regular output
			if !cfg.Quiet {
				var statusLine string

				if stats.CurrentFilePath != "" {
					// Truncate the path if it's too long
					path := stats.CurrentFilePath
					if len(path) > 40 {
						path = "..." + path[len(path)-37:]
					}

					statusLine = fmt.Sprintf("\rProcessing: %-40s | Files: %d/%d | Dirs: %d",
						path,
						stats.ProcessedFiles,
						stats.TotalFiles,
						stats.TotalDirs)
				} else {
					statusLine = fmt.Sprintf("\rScanning... | Files: %d/%d | Dirs: %d",
						stats.ProcessedFiles,
						stats.TotalFiles,
						stats.TotalDirs)
				}

				// Print with carriage return to overwrite previous line
				fmt.Fprint(os.Stderr, statusLine)
			}
		}))
	}

	// Add extension filtering if specified
	if len(fileExtensions) > 0 {
		var extList []string
		for ext := range fileExtensions {
			extList = append(extList, ext)
		}
		walkOptions = append(walkOptions, walker.WithExtensions(extList))
	}

	// Add binary inclusion option if requested
	if cfg.IncludeBinary {
		walkOptions = append(walkOptions, walker.WithIncludeBinary(true))
	}

	// Convert MB to bytes for MaxFileSize if specified
	if cfg.MaxFileSizeMB > 0 {
		maxSizeBytes := cfg.MaxFileSizeMB * 1024 * 1024
		walkOptions = append(walkOptions, walker.WithMaxFileSize(maxSizeBytes))
		infoLog("Ignoring files larger than %d MB.", cfg.MaxFileSizeMB)
	}

	// Add walk context option if timeout is specified
	if cfg.Ctx != nil {
		walkOptions = append(walkOptions, walker.WithContext(cfg.Ctx))
	}

	return matcher, walkOptions, nil
}
