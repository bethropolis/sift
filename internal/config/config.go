package config

import (
	"os"
	"runtime"
	"time"

	"github.com/mattn/go-isatty"
	"github.com/spf13/pflag"

	"github.com/bethropolis/sift/internal/smart"
)

// Config holds all application configuration settings.
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
	IgnoreHidden  bool
	IgnoreGit     bool
	CustomIgnore  string
	Extensions    string
	IncludeBinary bool
	// SmartFilter skips generated/lock/minified artifacts and oversized files.
	SmartFilter    bool
	SmartMaxTokens int

	// Output format
	Style          string
	JSONOutput     bool
	MarkdownOutput bool
	// NoNerdFonts forces ASCII glyphs in the interactive picker.
	NoNerdFonts bool

	// Profile selection
	Profile string

	// Token and context settings
	Budget        int
	TokenizeModel string
	Mode          string
	// Prompt is an optional task/directive section prepended to the output.
	Prompt string

	// Safety
	SecretScan   bool
	ForceSecrets bool
	Clipboard    bool

	// Version info
	ShowVersion bool
	Version     string
}

// New returns a Config populated with built-in defaults.
func New() *Config {
	return &Config{
		Version:        "1.0.4",
		IgnoreHidden:   true,
		IgnoreGit:      true,
		SecretScan:     true,
		Style:          "markdown",
		OutputFile:     "codebase.md",
		MaxWorkers:     runtime.NumCPU(),
		SmartMaxTokens: smart.DefaultMaxTokens,
	}
}

// EffectiveStyle returns the resolved output style, honoring the legacy
// --json and --markdown flags.
func (c *Config) EffectiveStyle() string {
	switch {
	case c.JSONOutput:
		return "json"
	case c.MarkdownOutput:
		return "markdown"
	case c.Style != "":
		return c.Style
	default:
		return "plain"
	}
}

// ResolveColors determines whether colored output should be used.
func (c *Config) ResolveColors() {
	c.UseColors = !c.NoColor && isatty.IsTerminal(os.Stderr.Fd()) && c.OutputFile == ""
}

// RegisterFlags binds every config option to fs, using the current field
// values as flag defaults.
func RegisterFlags(c *Config, fs *pflag.FlagSet) {
	fs.StringVar(&c.RootDir, "dir", c.RootDir, "The root directory to scan")
	fs.BoolVar(&c.Verbose, "verbose", c.Verbose, "Enable verbose logging (DEBUG, WARN, ERROR)")
	fs.BoolVar(&c.Quiet, "quiet", c.Quiet, "Suppress INFO messages (only show WARN, ERROR)")
	fs.StringVar(&c.LogLevel, "log-level", c.LogLevel, "Set the logging level (DEBUG, INFO, WARN, ERROR)")
	fs.BoolVar(&c.Concurrent, "concurrent", c.Concurrent, "Enable concurrent file processing")
	fs.IntVar(&c.MaxWorkers, "workers", c.MaxWorkers, "Max number of concurrent workers (defaults to number of CPU cores)")
	fs.Int64Var(&c.MaxFileSizeMB, "max-size", c.MaxFileSizeMB, "Max file size to process in MB (0 = no limit)")
	fs.BoolVar(&c.IgnoreHidden, "hidden", c.IgnoreHidden, "Ignore hidden files/directories (starting with '.')")
	fs.BoolVar(&c.IgnoreGit, "git", c.IgnoreGit, "Ignore .git directories")
	fs.StringVar(&c.CustomIgnore, "ignore", c.CustomIgnore, "Custom ignore patterns (comma-separated, gitignore syntax)")
	fs.StringVar(&c.Extensions, "ext", c.Extensions, "Only include files with these extensions (comma-separated, e.g., 'go,md,txt')")
	fs.BoolVar(&c.IncludeBinary, "binary", c.IncludeBinary, "Include binary files in output (default: skipped)")
	fs.BoolVar(&c.SmartFilter, "smart", c.SmartFilter, "Skip generated, lockfile, minified, and oversized files")
	fs.IntVar(&c.SmartMaxTokens, "smart-max-tokens", c.SmartMaxTokens, "Per-file token ceiling for the smart filter (default: 15000)")
	fs.BoolVar(&c.NoColor, "no-color", c.NoColor, "Disable color output")
	fs.StringVar(&c.OutputFile, "output", c.OutputFile, "Output file (default \"codebase.md\", use \"-\" for stdout)")
	fs.BoolVar(&c.ShowProgress, "progress", c.ShowProgress, "Show progress information")
	fs.DurationVar(&c.Timeout, "timeout", c.Timeout, "Maximum execution time (e.g., '30s', '5m')")
	fs.BoolVar(&c.ShowSkipped, "show-skipped", c.ShowSkipped, "Show a list of skipped files/directories and reasons at the end")
	fs.BoolVar(&c.ShowVersion, "version", c.ShowVersion, "Show version information")
	fs.BoolVar(&c.JSONOutput, "json", c.JSONOutput, "Output results in JSON format")
	fs.BoolVar(&c.MarkdownOutput, "markdown", c.MarkdownOutput, "Output results in Markdown format")

	// Output style, profiles, and LLM context settings
	fs.StringVar(&c.Style, "style", c.Style, "Output style: plain, markdown, json, xml")
	fs.BoolVar(&c.NoNerdFonts, "no-nerd-fonts", c.NoNerdFonts, "Use plain ASCII glyphs in the interactive picker")
	fs.StringVar(&c.Profile, "profile", c.Profile, "Config profile to use (see config.toml)")
	fs.IntVar(&c.Budget, "budget", c.Budget, "Maximum token budget (0 = no limit)")
	fs.StringVar(&c.TokenizeModel, "tokenize-model", c.TokenizeModel, "Tokenizer model encoding (default: cl100k_base)")
	fs.StringVar(&c.Mode, "mode", c.Mode, "Compression mode: full, signatures")
	fs.StringVarP(&c.Prompt, "prompt", "p", c.Prompt, "Task/instruction directives prepended to the output")
	fs.BoolVar(&c.SecretScan, "secrets", c.SecretScan, "Scan output for secrets and redact them")
	fs.BoolVar(&c.ForceSecrets, "force-secrets", c.ForceSecrets, "Include secrets in output instead of redacting")
	fs.BoolVar(&c.Clipboard, "clipboard", c.Clipboard, "Copy the rendered output to the system clipboard")
}
