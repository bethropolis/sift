package config

import "fmt"

// StringList is an array-of-strings TOML field. The canonical TOML form is
// array (e.g. extensions = ["go", "md"]). String form is intentionally not
// handled here — TOML extensions/ignore should always be arrays so the file
// stays readable and shell-escaping is avoided. Comma-separated strings are
// a CLI concern (Config.Extensions is a plain string joined at Apply time).
type StringList []string

// SiftDefaults holds top-level [sift] defaults. Every field is optional;
// zero/nil means "use Config.New() default". Mirrors a subset of Config.
type SiftDefaults struct {
	Style             string     `toml:"style"`
	TokenizeModel     string     `toml:"tokenize_model"`
	Budget            int        `toml:"budget"`
	Mode              string     `toml:"compress_mode"`
	Output            string     `toml:"output"`
	Prompt            string     `toml:"prompt"`
	PromptFile        string     `toml:"prompt_file"`
	SecretScan        *bool      `toml:"secret_scan"`
	ForceSecrets      *bool      `toml:"force_secrets"`
	IgnoreHidden      *bool      `toml:"ignore_hidden"`
	IgnoreGit         *bool      `toml:"ignore_git"`
	Extensions        StringList `toml:"extensions"`
	Ignore            StringList `toml:"ignore"`
	SmartFilter       *bool      `toml:"smart_filter"`
	SmartMaxTokens    int        `toml:"smart_max_tokens"`
	Highlight         *bool      `toml:"highlight"`
	NoHighlight       *bool      `toml:"no_highlight"`
	Theme             string     `toml:"theme"`
	UIThemeFile       string     `toml:"ui_theme_file"`
	HighlightMaxBytes int        `toml:"highlight_max_bytes"`
	WindowTitle       string     `toml:"window_title"`
	NoWindowTitle     *bool      `toml:"no_window_title"`
	CopyOnGenerate    *bool      `toml:"copy_on_generate"`
	Scoring           *Scoring   `toml:"scoring"`
}

// PromptRef holds either inline text or a file path for a prompt.
type PromptRef struct {
	Text string `toml:"text"`
	File string `toml:"file"`
}

// ResolvedPrompt returns the prompt string, reading from file if needed.
func (p PromptRef) ResolvedPrompt() (string, error) {
	if p.Text != "" && p.File != "" {
		return "", fmt.Errorf("prompt: specify either text or file, not both")
	}
	if p.File != "" {
		return "", fmt.Errorf("prompt file resolution requires ResolveConfig (file: %s)", p.File)
	}
	return p.Text, nil
}

// Target is one named dump artifact declared via [[targets]].
type Target struct {
	Name           string     `toml:"name"`
	Profile        string     `toml:"profile"`
	Output         string     `toml:"output"`
	Style          string     `toml:"style"`
	TokenizeModel  string     `toml:"tokenize_model"`
	Budget         int        `toml:"budget"`
	Mode           string     `toml:"compress_mode"`
	Prompt         string     `toml:"prompt"`
	PromptFile     string     `toml:"prompt_file"`
	SecretScan     *bool      `toml:"secret_scan"`
	ForceSecrets   *bool      `toml:"force_secrets"`
	IgnoreHidden   *bool      `toml:"ignore_hidden"`
	IgnoreGit      *bool      `toml:"ignore_git"`
	Extensions     StringList `toml:"extensions"`
	Ignore         StringList `toml:"ignore"`
	SmartFilter    *bool      `toml:"smart_filter"`
	SmartMaxTokens int        `toml:"smart_max_tokens"`
	Scoring        *Scoring   `toml:"scoring"`
}

// Automation holds optional declarative automation hints.
type Automation struct {
	Watch         *bool    `toml:"watch"`
	WatchInterval string   `toml:"watch_interval"`
	GitHook       string   `toml:"git_hook"`
	HookTargets   []string `toml:"hook_targets"`
}

// configFile is the full v2 TOML file shape.
type configFile struct {
	DefaultProfile string               `toml:"default_profile"`
	Sift           *SiftDefaults        `toml:"sift"`
	Profiles       map[string]Profile   `toml:"profiles"`
	Targets        []Target             `toml:"targets"`
	Prompts        map[string]PromptRef `toml:"prompts"`
	Automation     *Automation          `toml:"automation"`
	Scoring        *Scoring             `toml:"scoring"`
}
