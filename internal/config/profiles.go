package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"github.com/spf13/pflag"
)

// Profile is a named set of configuration overrides loaded from TOML.
// Pointer fields distinguish "not set" from an explicit value.
type Profile struct {
	Style             string   `toml:"style"`
	TokenizeModel     string   `toml:"tokenize_model"`
	Budget            int      `toml:"budget"`
	Mode              string   `toml:"compress_mode"`
	Prompt            string   `toml:"prompt"`
	SecretScan        *bool    `toml:"secret_scan"`
	ForceSecrets      *bool    `toml:"force_secrets"`
	IgnoreHidden      *bool    `toml:"ignore_hidden"`
	IgnoreGit         *bool    `toml:"ignore_git"`
	Extensions        []string `toml:"extensions"`
	SmartFilter       *bool    `toml:"smart_filter"`
	SmartMaxTokens    int      `toml:"smart_max_tokens"`
	Highlight         *bool    `toml:"highlight"`
	NoHighlight       *bool    `toml:"no_highlight"`
	Theme             string   `toml:"theme"`
	HighlightMaxBytes int      `toml:"highlight_max_bytes"`
}

type profilesFile struct {
	DefaultProfile string             `toml:"default_profile"`
	Profiles       map[string]Profile `toml:"profiles"`
}

// globalConfigPath returns the user-level config file path.
func globalConfigPath() string {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "sift", "config.toml")
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
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return pf, nil
		}
		return pf, err
	}
	if err := toml.Unmarshal(data, &pf); err != nil {
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
	if src.Prompt != "" {
		dst.Prompt = src.Prompt
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
	if src.SmartFilter != nil {
		dst.SmartFilter = src.SmartFilter
	}
	if src.SmartMaxTokens != 0 {
		dst.SmartMaxTokens = src.SmartMaxTokens
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
	if src.HighlightMaxBytes != 0 {
		dst.HighlightMaxBytes = src.HighlightMaxBytes
	}
	return dst
}

// Apply overlays the profile onto c, without overriding any flag the user
// explicitly set on the command line.
func (p Profile) Apply(c *Config, fs *pflag.FlagSet) {
	if p.Style != "" && !fs.Changed("style") {
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
	if p.Prompt != "" && !fs.Changed("prompt") {
		c.Prompt = p.Prompt
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
		c.Extensions = strings.Join(p.Extensions, ",")
	}
	if p.SmartFilter != nil && !fs.Changed("smart") {
		c.SmartFilter = *p.SmartFilter
	}
	if p.SmartMaxTokens != 0 && !fs.Changed("smart-max-tokens") {
		c.SmartMaxTokens = p.SmartMaxTokens
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
	if p.HighlightMaxBytes != 0 && !fs.Changed("highlight-max-bytes") {
		c.HighlightMaxBytes = p.HighlightMaxBytes
	}
}
