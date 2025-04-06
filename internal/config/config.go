package config

import (
	"flag"
	"os"
	"runtime" // Add runtime for CPU core count
	"time"

	"github.com/mattn/go-isatty"
)

// Config holds all application configuration settings
type Config struct {
	// Directory settings
	RootDir string

	// Logging settings
	Verbose     bool
	Quiet       bool
	LogLevel    string
	NoColor     bool
	UseColors   bool
	OutputFile  string
	ShowSkipped bool

	// Processing settings
	Concurrent    bool
	MaxWorkers    int
	MaxFileSizeMB int64
	ShowProgress  bool
	Timeout       time.Duration

	// Filtering settings
	IgnoreHidden bool
	IgnoreGit    bool
	CustomIgnore string
	Extensions   string

	// Output format
	JSONOutput     bool
	MarkdownOutput bool

	// Version info
	ShowVersion bool
	Version     string
}

// New creates a new Config with values from command-line flags
func New() *Config {
	c := &Config{
		Version: "1.0.4",
	}

	// Parse command-line flags
	flag.StringVar(&c.RootDir, "dir", ".", "The root directory to scan")
	flag.BoolVar(&c.Verbose, "verbose", false, "Enable verbose logging (DEBUG, WARN, ERROR)")
	flag.BoolVar(&c.Quiet, "quiet", false, "Suppress INFO messages (only show WARN, ERROR)")
	flag.StringVar(&c.LogLevel, "log-level", "INFO", "Set the logging level (DEBUG, INFO, WARN, ERROR)")
	flag.BoolVar(&c.Concurrent, "concurrent", false, "Enable concurrent file processing")
	flag.IntVar(&c.MaxWorkers, "workers", runtime.NumCPU(), "Max number of concurrent workers (defaults to number of CPU cores)")
	flag.Int64Var(&c.MaxFileSizeMB, "max-size", 0, "Max file size to process in MB (0 = no limit)")
	flag.BoolVar(&c.IgnoreHidden, "hidden", true, "Ignore hidden files/directories (starting with '.')")
	flag.BoolVar(&c.IgnoreGit, "git", true, "Ignore .git directories")
	flag.StringVar(&c.CustomIgnore, "ignore", "", "Custom ignore patterns (comma-separated, gitignore syntax)")
	flag.StringVar(&c.Extensions, "ext", "", "Only include files with these extensions (comma-separated, e.g., 'go,md,txt')")
	flag.BoolVar(&c.NoColor, "no-color", false, "Disable color output")
	flag.StringVar(&c.OutputFile, "output", "", "Output to file instead of stdout")
	flag.BoolVar(&c.ShowProgress, "progress", false, "Show progress information")
	flag.DurationVar(&c.Timeout, "timeout", 0, "Maximum execution time (e.g., '30s', '5m')")
	flag.BoolVar(&c.ShowSkipped, "show-skipped", false, "Show a list of skipped files/directories and reasons at the end")
	flag.BoolVar(&c.ShowVersion, "version", false, "Show version information")
	flag.BoolVar(&c.JSONOutput, "json", false, "Output results in JSON format")
	flag.BoolVar(&c.MarkdownOutput, "markdown", false, "Output results in Markdown format")

	flag.Parse()

	// Determine if colors should be used
	c.UseColors = !c.NoColor && isatty.IsTerminal(os.Stderr.Fd()) && c.OutputFile == ""

	return c
}
