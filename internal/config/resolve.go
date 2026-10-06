package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"github.com/spf13/pflag"
)

// ---------------------------------------------------------------------------
// Prompt file helper (used by Profile.Apply and Target.Apply).
// ---------------------------------------------------------------------------

func readPromptFile(path string) (string, error) {
	return readPromptFileWithRoot("", path)
}

func readPromptFileWithRoot(rootDir, path string) (string, error) {
	if !filepath.IsAbs(path) {
		// Try rootDir-relative first (e.g. `sift dump /repo` with prompt_file = "prompts/x.md").
		if rootDir != "" && rootDir != "." {
			candidate := filepath.Join(rootDir, path)
			if _, err := os.Stat(candidate); err == nil {
				path = candidate
			} else if abs, err := filepath.Abs(rootDir); err == nil {
				candidate2 := filepath.Join(abs, path)
				if _, err := os.Stat(candidate2); err == nil {
					path = candidate2
				}
			}
		}
		// Fall back to cwd-relative (covers tests that chdir and real cwd usage).
		if _, err := os.Stat(path); err != nil {
			if cwd, _ := os.Getwd(); cwd != "" {
				candidate := filepath.Join(cwd, path)
				if _, err := os.Stat(candidate); err == nil {
					path = candidate
				}
			}
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read prompt file %q: %w", path, err)
	}
	return string(data), nil
}

// ---------------------------------------------------------------------------
// Config file reading (v2 shape).
// ---------------------------------------------------------------------------

// readConfigFile loads a full v2 config file from path, returning a zero
// value when the file does not exist.
func readConfigFile(path string) (configFile, error) {
	var cf configFile
	if path == "" {
		return cf, nil
	}
	data, err := readConfigFileCapped(path)
	if err != nil {
		return cf, err
	}
	if len(data) == 0 {
		return cf, nil
	}
	if err := toml.Unmarshal(data, &cf); err != nil {
		return cf, err
	}
	if cf.Profiles == nil {
		cf.Profiles = map[string]Profile{}
	}
	if cf.Prompts == nil {
		cf.Prompts = map[string]PromptRef{}
	}
	return cf, nil
}

// ---------------------------------------------------------------------------
// SiftDefaults / Target / Automation → Config helpers.
// ---------------------------------------------------------------------------

func (s *SiftDefaults) applyTo(c *Config) {
	if s == nil {
		return
	}
	if s.Style != "" {
		c.Style = s.Style
	}
	if s.TokenizeModel != "" {
		c.TokenizeModel = s.TokenizeModel
	}
	if s.Budget != 0 {
		c.Budget = s.Budget
	}
	if s.Mode != "" {
		c.Mode = s.Mode
	}
	if s.Output != "" {
		c.OutputFile = s.Output
	}
	if s.Prompt != "" {
		c.Prompt = s.Prompt
	}
	if s.PromptFile != "" {
		if content, err := readPromptFileWithRoot(c.RootDir, s.PromptFile); err == nil {
			c.Prompt = content
		}
	}
	if s.SecretScan != nil {
		c.SecretScan = *s.SecretScan
	}
	if s.ForceSecrets != nil {
		c.ForceSecrets = *s.ForceSecrets
	}
	if s.IgnoreHidden != nil {
		c.IgnoreHidden = *s.IgnoreHidden
	}
	if s.IgnoreGit != nil {
		c.IgnoreGit = *s.IgnoreGit
	}
	if len(s.Extensions) > 0 {
		c.Extensions = strings.Join([]string(s.Extensions), ",")
	}
	if len(s.Ignore) > 0 {
		c.CustomIgnore = strings.Join([]string(s.Ignore), ",")
	}
	if s.SmartFilter != nil {
		c.SmartFilter = *s.SmartFilter
	}
	if s.SmartMaxTokens != 0 {
		c.SmartMaxTokens = s.SmartMaxTokens
	}
	if s.Highlight != nil {
		c.Highlight = *s.Highlight
	}
	if s.NoHighlight != nil {
		c.NoHighlight = *s.NoHighlight
	}
	if s.Theme != "" {
		c.Theme = s.Theme
	}
	if s.UIThemeFile != "" {
		c.UIThemeFile = s.UIThemeFile
	}
	if s.HighlightMaxBytes != 0 {
		c.HighlightMaxBytes = s.HighlightMaxBytes
	}
	if s.WindowTitle != "" {
		c.WindowTitle = s.WindowTitle
	}
	if s.NoWindowTitle != nil {
		c.NoWindowTitle = *s.NoWindowTitle
	}
	if s.CopyOnGenerate != nil {
		c.CopyOnGenerate = *s.CopyOnGenerate
	}
	if s.Scoring != nil {
		c.Scoring.Merge(*s.Scoring)
	}
}

func (t *Target) applyTo(c *Config, fs *pflag.FlagSet) {
	if t == nil {
		return
	}
	changed := func(name string) bool {
		if fs == nil {
			return false
		}
		return fs.Changed(name)
	}
	if t.Style != "" && !changed("style") {
		c.Style = t.Style
	}
	if t.TokenizeModel != "" && !changed("tokenize-model") {
		c.TokenizeModel = t.TokenizeModel
	}
	if t.Budget != 0 && !changed("budget") {
		c.Budget = t.Budget
	}
	if t.Mode != "" && !changed("mode") {
		c.Mode = t.Mode
	}
	if t.Output != "" && !changed("output") {
		c.OutputFile = t.Output
	}
	if t.Prompt != "" && !changed("prompt") {
		c.Prompt = t.Prompt
	}
	if t.PromptFile != "" && !changed("prompt") {
		if content, err := readPromptFileWithRoot(c.RootDir, t.PromptFile); err == nil {
			c.Prompt = content
		}
	}
	if t.SecretScan != nil && !changed("secrets") {
		c.SecretScan = *t.SecretScan
	}
	if t.ForceSecrets != nil && !changed("force-secrets") {
		c.ForceSecrets = *t.ForceSecrets
	}
	if t.IgnoreHidden != nil && !changed("hidden") {
		c.IgnoreHidden = *t.IgnoreHidden
	}
	if t.IgnoreGit != nil && !changed("git") {
		c.IgnoreGit = *t.IgnoreGit
	}
	if len(t.Extensions) > 0 && !changed("ext") {
		c.Extensions = strings.Join([]string(t.Extensions), ",")
	}
	if len(t.Ignore) > 0 && !changed("ignore") {
		c.CustomIgnore = strings.Join([]string(t.Ignore), ",")
	}
	if t.SmartFilter != nil && !changed("smart") {
		c.SmartFilter = *t.SmartFilter
	}
	if t.SmartMaxTokens != 0 && !changed("smart-max-tokens") {
		c.SmartMaxTokens = t.SmartMaxTokens
	}
	if t.Scoring != nil {
		c.Scoring.Merge(*t.Scoring)
	}
}

// ---------------------------------------------------------------------------
// Extends resolution.
// ---------------------------------------------------------------------------

// resolveExtends expands profile extends chains, detecting cycles.
// Returns a new map where each profile already includes its ancestors.
func resolveExtends(profiles map[string]Profile) (map[string]Profile, error) {
	resolved := make(map[string]Profile, len(profiles))
	for name := range profiles {
		p, err := expandProfile(name, profiles, map[string]bool{})
		if err != nil {
			return nil, err
		}
		resolved[name] = p
	}
	return resolved, nil
}

func expandProfile(name string, all map[string]Profile, visiting map[string]bool) (Profile, error) {
	if visiting[name] {
		return Profile{}, fmt.Errorf("profile %q: extends cycle detected", name)
	}
	p, ok := all[name]
	if !ok {
		return Profile{}, fmt.Errorf("profile %q: not found (extends target)", name)
	}
	if len(p.Extends) == 0 {
		return p, nil
	}
	visiting[name] = true
	defer delete(visiting, name)

	// Start from empty, overlay each ancestor in order, then overlay self
	// (self wins). Extends is not propagated to the result.
	var base Profile
	for _, ancestor := range p.Extends {
		ap, err := expandProfile(ancestor, all, visiting)
		if err != nil {
			return Profile{}, err
		}
		base = overlay(base, ap)
	}
	// Overlay self — but clear Extends so it doesn't re-expand.
	pCopy := p
	pCopy.Extends = nil
	base = overlay(base, pCopy)
	base.Extends = p.Extends // keep for introspection, not used after
	return base, nil
}

// ---------------------------------------------------------------------------
// Public resolution API.
// ---------------------------------------------------------------------------

// ResolveOption configures ResolveConfig.
type ResolveOption func(*resolveOpts)

type resolveOpts struct {
	targetName string
	promptName string
	// localPath pins the local .sift.toml. When localSet, no cwd fallback
	// happens: "" means "no local file". Serve uses this so the server's
	// startup directory can never leak into other projects.
	localPath string
	localSet  bool
}

// WithTarget selects a named [[targets]] entry.
func WithTarget(name string) ResolveOption {
	return func(o *resolveOpts) { o.targetName = name }
}

// WithPromptRef selects a named [prompts.NAME] entry as the prompt.
func WithPromptRef(name string) ResolveOption {
	return func(o *resolveOpts) { o.promptName = name }
}

// WithLocalConfig pins the local .sift.toml path, disabling the cwd
// fallback. Pass "" for no local file. Serve passes the target root's file
// (missing reads as empty) so per-directory configs map only to their own
// directory.
func WithLocalConfig(path string) ResolveOption {
	return func(o *resolveOpts) { o.localPath = path; o.localSet = true }
}

// localConfigPathFor returns the local .sift.toml path, preferring RootDir
// when the caller scans a directory other than cwd (e.g. `sift dump /path`).
func localConfigPathFor(c *Config) string {
	if c.RootDir != "" && c.RootDir != "." {
		p := filepath.Join(c.RootDir, ".sift.toml")
		if _, err := os.Stat(p); err == nil {
			return p
		}
		// Also try absolute-resolved path (RootDir may be relative).
		if abs, err := filepath.Abs(c.RootDir); err == nil {
			p2 := filepath.Join(abs, ".sift.toml")
			if _, err := os.Stat(p2); err == nil {
				return p2
			}
		}
	}
	return localConfigPath()
}

// ResolveConfig builds a resolved Config from global + local config files,
// profile selection, optional target and prompt ref, and the caller's FlagSet
// (whose Changed flags always win). It mirrors the precedence chain:
//
//	Config.New() → global [sift] → local [sift] → profile (+extends) → target → flags
func ResolveConfig(c *Config, fs *pflag.FlagSet, opts ...ResolveOption) error {
	var ro resolveOpts
	for _, o := range opts {
		o(&ro)
	}

	globalCF, err := readConfigFile(globalConfigPath())
	if err != nil {
		return fmt.Errorf("read global config: %w", err)
	}
	localPath := ""
	if ro.localSet {
		localPath = ro.localPath
	} else {
		localPath = localConfigPathFor(c)
	}
	localCF, err := readConfigFile(localPath)
	if err != nil {
		return fmt.Errorf("read .sift.toml: %w", err)
	}

	// Merge global + local into one view for resolution.
	merged := mergeConfigFiles(globalCF, localCF)

	// Expand profile extends chains.
	if len(merged.Profiles) > 0 {
		expanded, err := resolveExtends(merged.Profiles)
		if err != nil {
			return err
		}
		merged.Profiles = expanded
	}

	// Determine which profile to apply.
	profileName := c.Profile
	if profileName == "" {
		// If a target was requested and it names a profile, prefer that.
		if ro.targetName != "" {
			if tgt := findTarget(merged.Targets, ro.targetName); tgt != nil && tgt.Profile != "" {
				profileName = tgt.Profile
			}
		}
		if profileName == "" {
			profileName = merged.DefaultProfile
		}
	}

	// Apply [sift] defaults (local overrides global via mergeConfigFiles).
	// globalCF.Sift is already folded into merged.Sift with local winning.
	if merged.Sift != nil {
		// Sift defaults should NOT override flags, but they DO override New() defaults.
		// So apply before profile/target but after New() — with flag guard.
		applySiftWithFlagGuard(c, merged.Sift, fs)
	}
	// Top-level [scoring] (when not under [sift]) also applies.
	if merged.Scoring != nil {
		c.Scoring.Merge(*merged.Scoring)
	}

	// Apply profile (with extends already expanded), respecting flag guard.
	if profileName != "" {
		if p, ok := merged.Profiles[profileName]; ok {
			// Clear Extends before Apply so it doesn't leak.
			p.Extends = nil
			p.Apply(c, fs)
		} else if c.Profile != "" {
			// Explicit --profile must exist; default_profile may be absent.
			return fmt.Errorf("profile %q not found", profileName)
		}
	}

	// Apply target overrides.
	if ro.targetName != "" {
		tgt := findTarget(merged.Targets, ro.targetName)
		if tgt == nil {
			return fmt.Errorf("target %q not found", ro.targetName)
		}
		tgt.applyTo(c, fs)
	}

	// Apply prompts ref if requested and prompt not already set by target/profile.
	if ro.promptName != "" && (fs == nil || !fs.Changed("prompt")) && c.Prompt == "" {
		if pr, ok := merged.Prompts[ro.promptName]; ok {
			if pr.Text != "" && pr.File != "" {
				return fmt.Errorf("prompts.%s: specify either text or file, not both", ro.promptName)
			}
			if pr.Text != "" {
				c.Prompt = pr.Text
			} else if pr.File != "" {
				content, err := readPromptFileWithRoot(c.RootDir, pr.File)
				if err != nil {
					return err
				}
				c.Prompt = content
			}
		} else {
			return fmt.Errorf("prompt %q not found", ro.promptName)
		}
	}

	// Also handle Prompts referenced via target's prompt field? No — prompts
	// are selected via --prompt-ref or WithPromptRef; targets use inline prompt/prompt_file.

	return nil
}

func applySiftWithFlagGuard(c *Config, s *SiftDefaults, fs *pflag.FlagSet) {
	changed := func(name string) bool {
		if fs == nil {
			return false
		}
		return fs.Changed(name)
	}
	if s.Style != "" && !changed("style") {
		c.Style = s.Style
	}
	if s.TokenizeModel != "" && !changed("tokenize-model") {
		c.TokenizeModel = s.TokenizeModel
	}
	if s.Budget != 0 && !changed("budget") {
		c.Budget = s.Budget
	}
	if s.Mode != "" && !changed("mode") {
		c.Mode = s.Mode
	}
	if s.Output != "" && !changed("output") {
		c.OutputFile = s.Output
	}
	if s.Prompt != "" && !changed("prompt") {
		c.Prompt = s.Prompt
	}
	if s.PromptFile != "" && !changed("prompt") {
		if content, err := readPromptFileWithRoot(c.RootDir, s.PromptFile); err == nil {
			c.Prompt = content
		}
	}
	if s.SecretScan != nil && !changed("secrets") {
		c.SecretScan = *s.SecretScan
	}
	if s.ForceSecrets != nil && !changed("force-secrets") {
		c.ForceSecrets = *s.ForceSecrets
	}
	if s.IgnoreHidden != nil && !changed("hidden") {
		c.IgnoreHidden = *s.IgnoreHidden
	}
	if s.IgnoreGit != nil && !changed("git") {
		c.IgnoreGit = *s.IgnoreGit
	}
	if len(s.Extensions) > 0 && !changed("ext") {
		c.Extensions = strings.Join([]string(s.Extensions), ",")
	}
	if len(s.Ignore) > 0 && !changed("ignore") {
		c.CustomIgnore = strings.Join([]string(s.Ignore), ",")
	}
	if s.SmartFilter != nil && !changed("smart") {
		c.SmartFilter = *s.SmartFilter
	}
	if s.SmartMaxTokens != 0 && !changed("smart-max-tokens") {
		c.SmartMaxTokens = s.SmartMaxTokens
	}
	if s.Highlight != nil && !changed("highlight") {
		c.Highlight = *s.Highlight
	}
	if s.NoHighlight != nil && !changed("no-highlight") {
		c.NoHighlight = *s.NoHighlight
	}
	if s.Theme != "" && !changed("theme") {
		c.Theme = s.Theme
	}
	if s.HighlightMaxBytes != 0 && !changed("highlight-max-bytes") {
		c.HighlightMaxBytes = s.HighlightMaxBytes
	}
	if s.WindowTitle != "" && !changed("window-title") {
		c.WindowTitle = s.WindowTitle
	}
	if s.NoWindowTitle != nil && !changed("no-window-title") {
		c.NoWindowTitle = *s.NoWindowTitle
	}
	if s.CopyOnGenerate != nil && !changed("copy-on-generate") {
		c.CopyOnGenerate = *s.CopyOnGenerate
	}
	if s.Scoring != nil {
		c.Scoring.Merge(*s.Scoring)
	}
}

func findTarget(targets []Target, name string) *Target {
	for i := range targets {
		if targets[i].Name == name {
			return &targets[i]
		}
	}
	return nil
}

func mergeConfigFiles(global, local configFile) configFile {
	out := configFile{
		DefaultProfile: global.DefaultProfile,
		Sift:           global.Sift,
		Profiles:       map[string]Profile{},
		Prompts:        map[string]PromptRef{},
		Automation:     global.Automation,
		Scoring:        global.Scoring,
	}
	// Copy global profiles/prompts/targets.
	for k, v := range global.Profiles {
		out.Profiles[k] = v
	}
	for k, v := range global.Prompts {
		out.Prompts[k] = v
	}
	out.Targets = append(out.Targets, global.Targets...)

	// Local overrides.
	if local.DefaultProfile != "" {
		out.DefaultProfile = local.DefaultProfile
	}
	if local.Sift != nil {
		if out.Sift == nil {
			out.Sift = local.Sift
		} else {
			// Merge: local fields win when set.
			s := *out.Sift
			ls := local.Sift
			if ls.Style != "" {
				s.Style = ls.Style
			}
			if ls.TokenizeModel != "" {
				s.TokenizeModel = ls.TokenizeModel
			}
			if ls.Budget != 0 {
				s.Budget = ls.Budget
			}
			if ls.Mode != "" {
				s.Mode = ls.Mode
			}
			if ls.Output != "" {
				s.Output = ls.Output
			}
			if ls.Prompt != "" {
				s.Prompt = ls.Prompt
			}
			if ls.PromptFile != "" {
				s.PromptFile = ls.PromptFile
			}
			if ls.SecretScan != nil {
				s.SecretScan = ls.SecretScan
			}
			if ls.ForceSecrets != nil {
				s.ForceSecrets = ls.ForceSecrets
			}
			if ls.IgnoreHidden != nil {
				s.IgnoreHidden = ls.IgnoreHidden
			}
			if ls.IgnoreGit != nil {
				s.IgnoreGit = ls.IgnoreGit
			}
			if len(ls.Extensions) > 0 {
				s.Extensions = ls.Extensions
			}
			if len(ls.Ignore) > 0 {
				s.Ignore = ls.Ignore
			}
			if ls.SmartFilter != nil {
				s.SmartFilter = ls.SmartFilter
			}
			if ls.SmartMaxTokens != 0 {
				s.SmartMaxTokens = ls.SmartMaxTokens
			}
			if ls.Highlight != nil {
				s.Highlight = ls.Highlight
			}
			if ls.NoHighlight != nil {
				s.NoHighlight = ls.NoHighlight
			}
			if ls.Theme != "" {
				s.Theme = ls.Theme
			}
			if ls.HighlightMaxBytes != 0 {
				s.HighlightMaxBytes = ls.HighlightMaxBytes
			}
			if ls.WindowTitle != "" {
				s.WindowTitle = ls.WindowTitle
			}
			if ls.NoWindowTitle != nil {
				s.NoWindowTitle = ls.NoWindowTitle
			}
			if ls.Scoring != nil {
				if s.Scoring == nil {
					s.Scoring = ls.Scoring
				} else {
					s.Scoring.Merge(*ls.Scoring)
				}
			}
			out.Sift = &s
		}
	}
	for k, v := range local.Profiles {
		if existing, ok := out.Profiles[k]; ok {
			out.Profiles[k] = overlay(existing, v)
		} else {
			out.Profiles[k] = v
		}
	}
	for k, v := range local.Prompts {
		out.Prompts[k] = v
	}
	out.Targets = append(out.Targets, local.Targets...)
	if local.Automation != nil {
		out.Automation = local.Automation
	}
	if local.Scoring != nil {
		if out.Scoring == nil {
			out.Scoring = local.Scoring
		} else {
			out.Scoring.Merge(*local.Scoring)
		}
	}
	return out
}

// LoadConfigFile returns the merged global+local config file for inspection
// (used by `sift config show`).
func LoadConfigFile() (configFile, error) {
	globalCF, err := readConfigFile(globalConfigPath())
	if err != nil {
		return configFile{}, err
	}
	localCF, err := readConfigFile(localConfigPath())
	if err != nil {
		return configFile{}, err
	}
	merged := mergeConfigFiles(globalCF, localCF)
	if len(merged.Profiles) > 0 {
		expanded, err := resolveExtends(merged.Profiles)
		if err != nil {
			return configFile{}, err
		}
		merged.Profiles = expanded
	}
	return merged, nil
}
