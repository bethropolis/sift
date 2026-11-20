package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"github.com/spf13/pflag"

	"github.com/bethropolis/sift/internal/fsx"
)

// Profile is a named set of configuration overrides loaded from TOML.
// Pointer fields distinguish "not set" from an explicit value.
type Profile struct {
	Style             string       `toml:"style"`
	TokenizeModel     string       `toml:"tokenize_model"`
	BudgetRaw         any          `toml:"budget"`
	Budget            int          `toml:"-"`
	Mode              string       `toml:"compress_mode"`
	FallbackMode      string       `toml:"fallback_mode"`
	FallbackThreshold *float64     `toml:"fallback_threshold"`
	Output            string       `toml:"output"`
	Prompt            string       `toml:"prompt"`
	PromptFile        string       `toml:"prompt_file"`
	SecretScan        *bool        `toml:"secret_scan"`
	ForceSecrets      *bool        `toml:"force_secrets"`
	IgnoreHidden      *bool        `toml:"ignore_hidden"`
	IgnoreGit         *bool        `toml:"ignore_git"`
	Extensions        StringList   `toml:"extensions"`
	Ignore            StringList   `toml:"ignore"`
	OverrideIgnore    bool         `toml:"override_ignore"`
	SmartFilter       *bool        `toml:"smart_filter"`
	SmartMaxTokens    int          `toml:"smart_max_tokens"`
	IncludeGraph      *bool        `toml:"include_graph"`
	CollapseTests     *bool        `toml:"collapse_tests"`
	Highlight         *bool        `toml:"highlight"`
	NoHighlight       *bool        `toml:"no_highlight"`
	Theme             string       `toml:"theme"`
	HighlightMaxBytes int          `toml:"highlight_max_bytes"`
	WindowTitle       string       `toml:"window_title"`
	NoWindowTitle     *bool        `toml:"no_window_title"`
	UIThemeFile       string       `toml:"ui_theme_file"`
	CopyOnGenerate    *bool        `toml:"copy_on_generate"`
	Scoring           *Scoring     `toml:"scoring"`
	Modes             *ModesConfig `toml:"modes"`
	Extends           []string     `toml:"extends"`
}

type profilesFile struct {
	DefaultProfile string             `toml:"default_profile"`
	Profiles       map[string]Profile `toml:"profiles"`
}

// globalConfigPath returns the user-level config file path.
func globalConfigPath() string {
	dir, err := fsx.ConfigDir("sift")
	if err != nil {
		return ""
	}
	native := filepath.Join(dir, "config.toml")
	if _, err := os.Stat(native); err == nil {
		return native
	}
	// Backward compatibility: older builds only looked at ~/.config/sift on
	// macOS. Keep using that file when it exists and the native one does not.
	if home, homeErr := os.UserHomeDir(); homeErr == nil {
		legacy := filepath.Join(home, ".config", "sift", "config.toml")
		if legacy != native {
			if _, statErr := os.Stat(legacy); statErr == nil {
				return legacy
			}
		}
	}
	return native
}

// localConfigPath returns the per-repository config file path.
func localConfigPath() string {
	return ".sift.toml"
}

// readProfilesFile loads a profiles file, ignoring a missing file.
func readProfilesFile(path string) (profilesFile, error) {
	var pf profilesFile
	if path == "" {
		return pf, nil
	}
	data, err := readConfigFileCapped(path)
	if err != nil {
		return pf, err
	}
	if len(data) == 0 {
		return pf, nil
	}
	if err := toml.Unmarshal(data, &pf); err != nil {
		return pf, err
	}
	if err := pf.normalize(); err != nil {
		return pf, err
	}
	if pf.Profiles == nil {
		pf.Profiles = map[string]Profile{}
	}
	return pf, nil
}

// DefaultProfileName returns the configured default profile name, if any.
// The local .sift.toml takes precedence over the global config.
func DefaultProfileName() (string, error) {
	global, err := readProfilesFile(globalConfigPath())
	if err != nil {
		return "", err
	}
	local, err := readProfilesFile(localConfigPath())
	if err != nil {
		return "", err
	}
	if local.DefaultProfile != "" {
		return local.DefaultProfile, nil
	}
	return global.DefaultProfile, nil
}

// LoadProfile returns the merged profile with the given name.
// Global config values are applied first, then local overrides them.
func LoadProfile(name string) (Profile, error) {
	var merged Profile

	global, err := readProfilesFile(globalConfigPath())
	if err != nil {
		return merged, err
	}
	if p, ok := global.Profiles[name]; ok {
		merged = overlay(merged, p)
	}

	local, err := readProfilesFile(localConfigPath())
	if err != nil {
		return merged, err
	}
	if p, ok := local.Profiles[name]; ok {
		merged = overlay(merged, p)
	}

	return merged, nil
}

// overlay copies the non-zero fields of src onto dst.
func overlay(dst, src Profile) Profile {
	if src.Style != "" {
		dst.Style = src.Style
	}
	if src.TokenizeModel != "" {
		dst.TokenizeModel = src.TokenizeModel
	}
	if src.Budget != 0 {
		dst.Budget = src.Budget
	}
	if src.Mode != "" {
		dst.Mode = src.Mode
	}
	if src.FallbackMode != "" {
		dst.FallbackMode = src.FallbackMode
	}
	if src.FallbackThreshold != nil {
		dst.FallbackThreshold = src.FallbackThreshold
	}
	if src.Output != "" {
		dst.Output = src.Output
	}
	if src.Prompt != "" {
		dst.Prompt = src.Prompt
	}
	if src.PromptFile != "" {
		dst.PromptFile = src.PromptFile
	}
	if src.SecretScan != nil {
		dst.SecretScan = src.SecretScan
	}
	if src.ForceSecrets != nil {
		dst.ForceSecrets = src.ForceSecrets
	}
	if src.IgnoreHidden != nil {
		dst.IgnoreHidden = src.IgnoreHidden
	}
	if src.IgnoreGit != nil {
		dst.IgnoreGit = src.IgnoreGit
	}
	if len(src.Extensions) > 0 {
		dst.Extensions = src.Extensions
	}
	if len(src.Ignore) > 0 {
		if src.OverrideIgnore {
			dst.Ignore = src.Ignore
		} else {
			dst.Ignore = append(dst.Ignore, src.Ignore...)
		}
	}
	if src.OverrideIgnore {
		dst.OverrideIgnore = true
	}
	if src.SmartFilter != nil {
		dst.SmartFilter = src.SmartFilter
	}
	if src.SmartMaxTokens != 0 {
		dst.SmartMaxTokens = src.SmartMaxTokens
	}
	if src.IncludeGraph != nil {
		dst.IncludeGraph = src.IncludeGraph
	}
	if src.CollapseTests != nil {
		dst.CollapseTests = src.CollapseTests
	}
	if src.Highlight != nil {
		dst.Highlight = src.Highlight
	}
	if src.NoHighlight != nil {
		dst.NoHighlight = src.NoHighlight
	}
	if src.Theme != "" {
		dst.Theme = src.Theme
	}
	if src.UIThemeFile != "" {
		dst.UIThemeFile = src.UIThemeFile
	}
	if src.CopyOnGenerate != nil {
		dst.CopyOnGenerate = src.CopyOnGenerate
	}
	if src.HighlightMaxBytes != 0 {
		dst.HighlightMaxBytes = src.HighlightMaxBytes
	}
	if src.WindowTitle != "" {
		dst.WindowTitle = src.WindowTitle
	}
	if src.NoWindowTitle != nil {
		dst.NoWindowTitle = src.NoWindowTitle
	}
	if src.Scoring != nil {
		merged := *src.Scoring
		if dst.Scoring != nil {
			merged = *dst.Scoring
			merged.Merge(*src.Scoring)
		}
		dst.Scoring = &merged
	}
	if src.Modes != nil {
		if dst.Modes == nil {
			dst.Modes = &ModesConfig{}
		}
		if len(src.Modes.Full) > 0 {
			dst.Modes.Full = src.Modes.Full
		}
		if len(src.Modes.Signatures) > 0 {
			dst.Modes.Signatures = src.Modes.Signatures
		}
		if len(src.Modes.Skip) > 0 {
			dst.Modes.Skip = src.Modes.Skip
		}
	}
	if len(src.Extends) > 0 {
		dst.Extends = src.Extends
	}
	return dst
}

// Apply overlays the profile onto c, without overriding any flag the user
// explicitly set on the command line.
func (p Profile) Apply(c *Config, fs *pflag.FlagSet) error {
	// A nil FlagSet means "no CLI overrides"; treat every flag as unchanged
	// instead of panicking on fs.Changed.
	if fs == nil {
		fs = pflag.NewFlagSet("profile", pflag.ContinueOnError)
	}
	if p.Prompt != "" && p.PromptFile != "" {
		return fmt.Errorf("profile: specify either prompt or prompt_file, not both")
	}
	if p.Style != "" && !isStyleChanged(fs) {
		c.Style = p.Style
	}
	if p.TokenizeModel != "" && !fs.Changed("tokenize-model") {
		c.TokenizeModel = p.TokenizeModel
	}
	if p.Budget > 0 && !fs.Changed("budget") {
		c.Budget = p.Budget
	}
	if p.Mode != "" && !fs.Changed("mode") {
		c.Mode = p.Mode
	}
	if p.FallbackMode != "" && !fs.Changed("fallback-mode") {
		c.FallbackMode = p.FallbackMode
	}
	if p.FallbackThreshold != nil && !fs.Changed("fallback-threshold") {
		c.FallbackThreshold = *p.FallbackThreshold
	}
	if p.Output != "" && !fs.Changed("output") {
		c.OutputFile = p.Output
		c.OutputFileExplicit = true
	}
	if p.Prompt != "" && !fs.Changed("prompt") {
		c.Prompt = p.Prompt
	}
	if p.PromptFile != "" && !fs.Changed("prompt") {
		content, err := readPromptFileWithRoot(c.RootDir, p.PromptFile)
		if err != nil {
			return err
		}
		c.Prompt = content
	}
	if p.SecretScan != nil && !fs.Changed("secrets") {
		c.SecretScan = *p.SecretScan
	}
	if p.ForceSecrets != nil && !fs.Changed("force-secrets") {
		c.ForceSecrets = *p.ForceSecrets
	}
	if p.IgnoreHidden != nil && !fs.Changed("hidden") {
		c.IgnoreHidden = *p.IgnoreHidden
	}
	if p.IgnoreGit != nil && !fs.Changed("git") {
		c.IgnoreGit = *p.IgnoreGit
	}
	if len(p.Extensions) > 0 && !fs.Changed("ext") {
		c.Extensions = strings.Join([]string(p.Extensions), ",")
	}
	if p.OverrideIgnore && !fs.Changed("ignore") {
		c.CustomIgnore = strings.Join([]string(p.Ignore), ",")
	} else if len(p.Ignore) > 0 && !fs.Changed("ignore") {
		if c.CustomIgnore == "" {
			c.CustomIgnore = strings.Join([]string(p.Ignore), ",")
		} else {
			base := strings.Split(c.CustomIgnore, ",")
			merged := append(base, []string(p.Ignore)...)
			seen := make(map[string]bool, len(merged))
			unique := make([]string, 0, len(merged))
			for _, pat := range merged {
				pat = strings.TrimSpace(pat)
				if pat != "" && !seen[pat] {
					seen[pat] = true
					unique = append(unique, pat)
				}
			}
			c.CustomIgnore = strings.Join(unique, ",")
		}
	}
	if p.SmartFilter != nil && !fs.Changed("smart") {
		c.SmartFilter = *p.SmartFilter
	}
	if p.SmartMaxTokens != 0 && !fs.Changed("smart-max-tokens") {
		c.SmartMaxTokens = p.SmartMaxTokens
	}
	if p.IncludeGraph != nil && !fs.Changed("include-graph") {
		c.IncludeGraph = *p.IncludeGraph
	}
	if p.CollapseTests != nil && !fs.Changed("collapse-tests") {
		c.CollapseTests = *p.CollapseTests
	}
	if p.Highlight != nil && !fs.Changed("highlight") {
		c.Highlight = *p.Highlight
	}
	if p.NoHighlight != nil && !fs.Changed("no-highlight") {
		c.NoHighlight = *p.NoHighlight
	}
	if p.Theme != "" && !fs.Changed("theme") {
		c.Theme = p.Theme
	}
	if p.UIThemeFile != "" && !fs.Changed("ui-theme-file") {
		c.UIThemeFile = p.UIThemeFile
	}
	if p.CopyOnGenerate != nil && !fs.Changed("copy-on-generate") {
		c.CopyOnGenerate = *p.CopyOnGenerate
	}
	if p.HighlightMaxBytes != 0 && !fs.Changed("highlight-max-bytes") {
		c.HighlightMaxBytes = p.HighlightMaxBytes
	}
	if p.WindowTitle != "" && !fs.Changed("window-title") {
		c.WindowTitle = p.WindowTitle
	}
	if p.NoWindowTitle != nil && !fs.Changed("no-window-title") {
		c.NoWindowTitle = *p.NoWindowTitle
	}
	if p.Scoring != nil {
		c.Scoring.Merge(*p.Scoring)
	}
	if p.Modes != nil {
		// A profile's [modes] replaces the current mapping wholesale; tiers are
		// not merged field by field (matching Target and [sift] behavior).
		c.Modes = *p.Modes
	}
	return nil
}
