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

// Prompt file helper (used by Profile.Apply and Target.Apply).

func readPromptFile(path string) (string, error) {
	return readPromptFileWithRoot("", path)
}

func readPromptFileWithRoot(rootDir, path string) (string, error) {
	// Project-relative reads are confined to the project root: a symlinked
	// prompt_file in an untrusted repository must not read host files.
	if rootDir != "" && !filepath.IsAbs(filepath.FromSlash(path)) {
		data, err := fsx.ReadFile(rootDir, path)
		if err != nil {
			return "", fmt.Errorf("read prompt file %q: %w", path, err)
		}
		return string(data), nil
	}
	// Absolute paths (trusted global config) and root-less callers keep the
	// historical resolution order.
	if !filepath.IsAbs(path) {
		if _, err := os.Stat(path); err != nil {
			if cwd, _ := os.Getwd(); cwd != "" {
				candidate := filepath.Join(cwd, path)
				if _, err := os.Stat(candidate); err == nil {
					path = candidate
				}
			}
		}
	}
	data, err := fsx.ReadFile("", path)
	if err != nil {
		return "", fmt.Errorf("read prompt file %q: %w", path, err)
	}
	return string(data), nil
}

// Config file reading (v2 shape).

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
	if err := cf.normalize(); err != nil {
		return cf, err
	}
	if cf.Profiles == nil {
		cf.Profiles = map[string]Profile{}
	}
	if cf.Prompts == nil {
		cf.Prompts = map[string]PromptRef{}
	}
	if abs, absErr := filepath.Abs(path); absErr == nil {
		dir := filepath.Dir(abs)
		for i := range cf.Targets {
			cf.Targets[i].sourceDir = dir
		}
	}
	return cf, nil
}

// Target / Automation → Config helpers.

func isStyleChanged(fs *pflag.FlagSet) bool {
	if fs == nil {
		return false
	}
	return fs.Changed("style") || fs.Changed("format") || fs.Changed("json") || fs.Changed("markdown")
}

func (t *Target) applyTo(c *Config, fs *pflag.FlagSet) error {
	if t == nil {
		return nil
	}
	changed := func(name string) bool {
		if fs == nil {
			return false
		}
		return fs.Changed(name)
	}
	if t.Style != "" && !isStyleChanged(fs) {
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
	if t.FallbackMode != "" && !changed("fallback-mode") {
		c.FallbackMode = t.FallbackMode
	}
	if t.FallbackThreshold != nil && !changed("fallback-threshold") {
		c.FallbackThreshold = *t.FallbackThreshold
	}
	if t.UIThemeFile != "" && !changed("ui-theme-file") {
		c.UIThemeFile = t.UIThemeFile
	}
	if t.CopyOnGenerate != nil && !changed("copy-on-generate") {
		c.CopyOnGenerate = *t.CopyOnGenerate
	}
	if t.Output != "" && !changed("output") {
		c.OutputFile = t.Output
		c.OutputFileExplicit = true
	}
	if t.Prompt != "" && !changed("prompt") {
		c.Prompt = t.Prompt
	}
	if t.Prompt != "" && t.PromptFile != "" {
		return fmt.Errorf("target %s: specify either prompt or prompt_file, not both", t.Name)
	}
	if t.PromptFile != "" && !changed("prompt") {
		content, err := readPromptFileWithRoot(c.RootDir, t.PromptFile)
		if err != nil {
			return err
		}
		c.Prompt = content
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
	if t.OverrideIgnore && !changed("ignore") {
		c.CustomIgnore = strings.Join([]string(t.Ignore), ",")
	} else if len(t.Ignore) > 0 && !changed("ignore") {
		if c.CustomIgnore == "" {
			c.CustomIgnore = strings.Join([]string(t.Ignore), ",")
		} else {
			base := strings.Split(c.CustomIgnore, ",")
			merged := append(base, []string(t.Ignore)...)
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
	if len(t.Roots) > 0 {
		c.Roots = append([]string(nil), t.Roots...)
	}
	if t.SmartFilter != nil && !changed("smart") {
		c.SmartFilter = *t.SmartFilter
	}
	if t.SmartMaxTokens != 0 && !changed("smart-max-tokens") {
		c.SmartMaxTokens = t.SmartMaxTokens
	}
	if t.IncludeGraph != nil && !changed("include-graph") {
		c.IncludeGraph = *t.IncludeGraph
	}
	if t.CollapseTests != nil && !changed("collapse-tests") {
		c.CollapseTests = *t.CollapseTests
	}
	if t.Scoring != nil {
		c.Scoring.Merge(*t.Scoring)
	}
	if t.Modes != nil {
		c.Modes = *t.Modes
	}
	return nil
}

// Extends resolution.

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

// Public resolution API.

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

// LocalBudget reports the [sift].budget set in the local .sift.toml at
// path. A missing file reads as zero (no budget); a malformed file errors.
// Serve uses this to tell the web client which budget wins without
// re-running the full resolution.
func LocalBudget(path string) (int, error) {
	cf, err := readConfigFile(path)
	if err != nil {
		return 0, err
	}
	if cf.Sift == nil {
		return 0, nil
	}
	return cf.Sift.Budget, nil
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
		// An explicit root has no local config; never fall back to the
		// caller's working directory (that would apply cwd's .sift.toml to a
		// completely different project).
		return ""
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
	} else if c.ConfigFile != "" {
		if _, err := os.Stat(c.ConfigFile); err != nil {
			return fmt.Errorf("config file %q: %w", c.ConfigFile, err)
		}
		localPath = c.ConfigFile
	} else {
		localPath = localConfigPathFor(c)
	}
	localCF, err := readConfigFile(localPath)
	if err != nil {
		return fmt.Errorf("read config file: %w", err)
	}

	// Merge global + local into one view for resolution.
	merged := mergeConfigFiles(globalCF, localCF)

	var target *Target
	if ro.targetName != "" {
		target = findTarget(merged.Targets, ro.targetName)
		if target == nil {
			return fmt.Errorf("target %q not found", ro.targetName)
		}
	}

	// A target-level config is a more specific local layer. Load it before
	// profile expansion so its profiles can be selected by the target.
	var targetConfigDir string
	if target != nil && target.Config != "" {
		targetCF, configDir, err := readTargetConfig(target, c.RootDir)
		if err != nil {
			return err
		}
		targetConfigDir = configDir
		if targetCF.Sift == nil || len(targetCF.Sift.Roots) == 0 {
			clearInheritedRoots(&merged)
		}
		if targetCF.Sift == nil || targetCF.Sift.Output == "" {
			clearInheritedOutput(&merged)
		}
		merged = mergeConfigFiles(merged, targetCF)
	}

	if merged.Automation != nil {
		c.Automation = *merged.Automation
	}
	if merged.History != nil {
		if merged.History.Enabled != nil {
			c.HistoryEnabled = *merged.History.Enabled
		}
		if merged.History.Keep > 0 {
			c.HistoryKeep = merged.History.Keep
		}
	}

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
		if target != nil && target.Profile != "" {
			profileName = target.Profile
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
		if err := applySiftWithFlagGuard(c, merged.Sift, fs); err != nil {
			return err
		}
	}
	// Top-level [scoring] (when not under [sift]) also applies.
	if merged.Scoring != nil {
		c.Scoring.Merge(*merged.Scoring)
	}
	// Top-level [modes] also applies.
	if merged.Modes != nil {
		c.Modes = *merged.Modes
	}

	// Apply profile (with extends already expanded), respecting flag guard.
	if profileName != "" {
		if p, ok := merged.Profiles[profileName]; ok {
			// Clear Extends before Apply so it doesn't leak.
			p.Extends = nil
			if err := p.Apply(c, fs); err != nil {
				return err
			}
		} else if c.Profile != "" {
			// Explicit --profile must exist; default_profile may be absent.
			return fmt.Errorf("profile %q not found", profileName)
		}
	}

	// Apply target overrides.
	outputBase := c.RootDir
	if target != nil {
		if err := target.applyTo(c, fs); err != nil {
			return err
		}
		if target.Config != "" {
			applyTargetConfigRoot(c, targetConfigDir, fs)
		}
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

	if fs != nil && fs.Changed("output") {
		c.OutputFileExplicit = true
	}
	if isStyleChanged(fs) && (fs == nil || !fs.Changed("output")) {
		if strings.HasPrefix(filepath.Base(c.OutputFile), "codebase.") {
			c.OutputFileExplicit = false
		}
	}
	// Normalize aliases after flags/profiles/targets so runtime comparisons
	// (Mode == "signatures", style switches) see canonical names.
	c.Style = CanonicalStyle(c.Style)
	c.Mode = CanonicalMode(c.Mode)
	if c.FallbackMode != "" {
		c.FallbackMode = CanonicalMode(c.FallbackMode)
	}
	c.ResolveOutputFile()
	if target != nil && target.Config != "" {
		anchorTargetOutput(c, target, fs, outputBase)
	}

	return nil
}

func applySiftWithFlagGuard(c *Config, s *SiftDefaults, fs *pflag.FlagSet) error {
	changed := func(name string) bool {
		if fs == nil {
			return false
		}
		return fs.Changed(name)
	}
	if s.Prompt != "" && s.PromptFile != "" {
		return fmt.Errorf("[sift]: specify either prompt or prompt_file, not both")
	}
	if s.Style != "" && !isStyleChanged(fs) {
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
	if s.FallbackMode != "" && !changed("fallback-mode") {
		c.FallbackMode = s.FallbackMode
	}
	if s.FallbackThreshold != nil && !changed("fallback-threshold") {
		c.FallbackThreshold = *s.FallbackThreshold
	}
	if s.Output != "" && !changed("output") {
		c.OutputFile = s.Output
		c.OutputFileExplicit = true
	}
	if s.Prompt != "" && !changed("prompt") {
		c.Prompt = s.Prompt
	}
	if s.PromptFile != "" && !changed("prompt") {
		content, err := readPromptFileWithRoot(c.RootDir, s.PromptFile)
		if err != nil {
			return err
		}
		c.Prompt = content
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
	if len(s.Roots) > 0 {
		c.Roots = append([]string(nil), s.Roots...)
	}
	if s.SmartFilter != nil && !changed("smart") {
		c.SmartFilter = *s.SmartFilter
	}
	if s.SmartMaxTokens != 0 && !changed("smart-max-tokens") {
		c.SmartMaxTokens = s.SmartMaxTokens
	}
	if s.IncludeGraph != nil && !changed("include-graph") {
		c.IncludeGraph = *s.IncludeGraph
	}
	if s.CollapseTests != nil && !changed("collapse-tests") {
		c.CollapseTests = *s.CollapseTests
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
	if s.UIThemeFile != "" && !changed("ui-theme-file") {
		c.UIThemeFile = s.UIThemeFile
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
	if s.Modes != nil {
		c.Modes = *s.Modes
	}
	return nil
}

// readTargetConfig loads a target's referenced config file and resolves its
// relative paths against that file's directory. Targets declared inside the
// referenced file are ignored: only the named target from the declaring config
// defines the artifact.
func readTargetConfig(t *Target, projectRoot string) (configFile, string, error) {
	var empty configFile
	if t == nil || t.Config == "" {
		return empty, "", nil
	}

	path := t.Config
	if !filepath.IsAbs(path) {
		base := t.sourceDir
		if base == "" {
			base = projectRoot
		}
		if base == "" {
			base = "."
		}
		path = filepath.Join(base, path)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return empty, "", fmt.Errorf("target %q config %q: %w", t.Name, t.Config, err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return empty, "", fmt.Errorf("target %q config %q: %w", t.Name, t.Config, err)
	}
	if info.IsDir() {
		return empty, "", fmt.Errorf("target %q config %q is a directory", t.Name, t.Config)
	}

	cf, err := readConfigFile(abs)
	if err != nil {
		return empty, "", fmt.Errorf("target %q config %q: %w", t.Name, t.Config, err)
	}
	cf.Targets = nil
	resolveConfigPaths(&cf, filepath.Dir(abs))
	return cf, filepath.Dir(abs), nil
}

// resolveConfigPaths makes paths declared by a target config absolute relative
// to that config's directory, so they keep working when the target runs from
// the project root.
func resolveConfigPaths(cf *configFile, base string) {
	resolve := func(path string) string {
		if path == "" || path == "-" || filepath.IsAbs(path) {
			return path
		}
		return filepath.Join(base, path)
	}
	resolveList := func(paths StringList) StringList {
		if len(paths) == 0 {
			return paths
		}
		out := make(StringList, len(paths))
		for i, path := range paths {
			out[i] = resolve(path)
		}
		return out
	}

	if cf.Sift != nil {
		s := *cf.Sift
		s.Roots = resolveList(s.Roots)
		s.PromptFile = resolve(s.PromptFile)
		s.Output = resolve(s.Output)
		s.UIThemeFile = resolve(s.UIThemeFile)
		cf.Sift = &s
	}
	for name, p := range cf.Profiles {
		p.Roots = resolveList(p.Roots)
		p.PromptFile = resolve(p.PromptFile)
		p.Output = resolve(p.Output)
		p.UIThemeFile = resolve(p.UIThemeFile)
		cf.Profiles[name] = p
	}
	for name, p := range cf.Prompts {
		p.File = resolve(p.File)
		cf.Prompts[name] = p
	}
}

// clearInheritedRoots removes roots inherited from the outer config when a
// target config does not declare its own. The target then defaults to the
// referenced config's directory.
func clearInheritedRoots(cf *configFile) {
	if cf.Sift == nil {
		return
	}
	s := *cf.Sift
	s.Roots = nil
	cf.Sift = &s
}

// clearInheritedOutput removes an outer output path for the same reason: a
// target config without an output should not inherit the project root's.
func clearInheritedOutput(cf *configFile) {
	if cf.Sift == nil {
		return
	}
	s := *cf.Sift
	s.Output = ""
	cf.Sift = &s
}

// applyTargetConfigRoot scopes the target to its config file's directory. A
// single explicit root is promoted to RootDir, matching the normal dump path.
func applyTargetConfigRoot(c *Config, configDir string, fs *pflag.FlagSet) {
	if configDir == "" {
		return
	}
	if len(c.Roots) == 0 {
		if fs == nil || !fs.Changed("dir") {
			c.RootDir = configDir
		}
		return
	}
	if len(c.Roots) == 1 {
		c.RootDir = c.Roots[0]
		c.Roots = nil
	}
}

// anchorTargetOutput puts a target's relative output at the project root
// instead of the target config's module directory.
func anchorTargetOutput(c *Config, t *Target, fs *pflag.FlagSet, base string) {
	if t.Output == "" || c.OutputFile == "" || c.OutputFile == "-" || filepath.IsAbs(c.OutputFile) {
		return
	}
	if fs != nil && fs.Changed("output") {
		return
	}
	if base == "" {
		base = "."
	}
	if abs, err := filepath.Abs(base); err == nil {
		base = abs
	}
	c.OutputFile = filepath.Join(base, c.OutputFile)
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
		History:        global.History,
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
			if ls.FallbackMode != "" {
				s.FallbackMode = ls.FallbackMode
			}
			if ls.FallbackThreshold != nil {
				s.FallbackThreshold = ls.FallbackThreshold
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
			if len(ls.Roots) > 0 {
				s.Roots = ls.Roots
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
			if ls.IncludeGraph != nil {
				s.IncludeGraph = ls.IncludeGraph
			}
			if ls.CollapseTests != nil {
				s.CollapseTests = ls.CollapseTests
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
			if ls.UIThemeFile != "" {
				s.UIThemeFile = ls.UIThemeFile
			}
			if ls.CopyOnGenerate != nil {
				s.CopyOnGenerate = ls.CopyOnGenerate
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
			if ls.Modes != nil {
				s.Modes = ls.Modes
			}
			out.Sift = &s
		}
	}
	if local.Modes != nil {
		out.Modes = local.Modes
		// A local top-level [modes] is as specific as [sift.modes]. Clear an
		// inherited global [sift.modes] so it cannot override the local file.
		if out.Sift != nil {
			s := *out.Sift
			s.Modes = nil
			out.Sift = &s
		}
	} else if global.Modes != nil && (local.Sift == nil || local.Sift.Modes == nil) {
		// A local [sift.modes] is more specific than the global top-level
		// [modes]. Inherit the global table only when the local file has no
		// modes in either form.
		out.Modes = global.Modes
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
	if local.History != nil {
		if out.History == nil {
			out.History = local.History
		} else {
			merged := *out.History
			if local.History.Enabled != nil {
				merged.Enabled = local.History.Enabled
			}
			if local.History.Keep > 0 {
				merged.Keep = local.History.Keep
			}
			out.History = &merged
		}
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

// TargetNames returns the [[targets]] names visible to c, in file order with
// local targets after global ones. It reads the same files ResolveConfig uses
// without mutating c, so batch commands can enumerate targets before applying
// any one of them.
func TargetNames(c *Config) ([]string, error) {
	globalCF, err := readConfigFile(globalConfigPath())
	if err != nil {
		return nil, fmt.Errorf("read global config: %w", err)
	}
	localPath := ""
	if c.ConfigFile != "" {
		if _, err := os.Stat(c.ConfigFile); err != nil {
			return nil, fmt.Errorf("config file %q: %w", c.ConfigFile, err)
		}
		localPath = c.ConfigFile
	} else {
		localPath = localConfigPathFor(c)
	}
	localCF, err := readConfigFile(localPath)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}
	merged := mergeConfigFiles(globalCF, localCF)
	seen := map[string]bool{}
	names := make([]string, 0, len(merged.Targets))
	for _, t := range merged.Targets {
		if t.Name == "" || seen[t.Name] {
			continue
		}
		seen[t.Name] = true
		names = append(names, t.Name)
	}
	return names, nil
}
