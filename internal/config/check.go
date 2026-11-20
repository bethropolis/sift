package config

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// ValidationSeverity indicates whether an issue is an error or a warning.
type ValidationSeverity string

const (
	SeverityError   ValidationSeverity = "error"
	SeverityWarning ValidationSeverity = "warning"
)

// ValidationIssue represents a problem or warning found in a configuration file.
type ValidationIssue struct {
	Severity ValidationSeverity
	Field    string
	Message  string
}

func (i ValidationIssue) String() string {
	if i.Field != "" {
		return fmt.Sprintf("[%s] %s: %s", i.Severity, i.Field, i.Message)
	}
	return fmt.Sprintf("[%s] %s", i.Severity, i.Message)
}

// ModesSummary contains a brief overview of configured mode globs.
type ModesSummary struct {
	FullCount int
	SigsCount int
	SkipCount int
}

// ConfigSummary contains high-level information about a parsed valid configuration.
type ConfigSummary struct {
	Path              string
	DefaultProfile    string
	Profiles          []string
	Targets           []string
	Modes             *ModesSummary
	Budget            int
	FallbackMode      string
	FallbackThreshold *float64
	Style             string
	Output            string
	IgnoreCount       int
}

// CheckResult holds the complete result of checking a configuration file.
type CheckResult struct {
	Path    string
	Valid   bool
	Issues  []ValidationIssue
	Summary ConfigSummary
}

// Errors returns all issues with SeverityError.
func (r *CheckResult) Errors() []ValidationIssue {
	var errs []ValidationIssue
	for _, issue := range r.Issues {
		if issue.Severity == SeverityError {
			errs = append(errs, issue)
		}
	}
	return errs
}

// Warnings returns all issues with SeverityWarning.
func (r *CheckResult) Warnings() []ValidationIssue {
	var warns []ValidationIssue
	for _, issue := range r.Issues {
		if issue.Severity == SeverityWarning {
			warns = append(warns, issue)
		}
	}
	return warns
}

// ResolveConfigFileTarget determines the exact configuration file path given
// an optional path argument and root directory.
func ResolveConfigFileTarget(path string, rootDir string) (string, error) {
	if path != "" {
		fi, err := os.Stat(path)
		if err != nil {
			return "", fmt.Errorf("config path %q: %w", path, err)
		}
		if fi.IsDir() {
			candidate := filepath.Join(path, ".sift.toml")
			if _, err := os.Stat(candidate); err != nil {
				return "", fmt.Errorf("no .sift.toml found in directory %q", path)
			}
			return candidate, nil
		}
		return path, nil
	}

	// Default to rootDir or cwd .sift.toml
	base := rootDir
	if base == "" {
		base = "."
	}
	candidate := filepath.Join(base, ".sift.toml")
	if _, err := os.Stat(candidate); err != nil {
		if abs, absErr := filepath.Abs(base); absErr == nil {
			candidate2 := filepath.Join(abs, ".sift.toml")
			if _, err2 := os.Stat(candidate2); err2 == nil {
				return candidate2, nil
			}
		}
		return "", fmt.Errorf("no .sift.toml found in %q", base)
	}
	return candidate, nil
}

// ValidateConfigFile inspects a .sift.toml file at path (or relative to rootDir)
// and returns comprehensive diagnostics and summary information.
func ValidateConfigFile(path string, rootDir string) (*CheckResult, error) {
	targetPath, err := ResolveConfigFileTarget(path, rootDir)
	if err != nil {
		return nil, err
	}

	result := &CheckResult{
		Path: targetPath,
	}

	// 1. Read file bytes with safety cap
	data, err := readConfigFileCapped(targetPath)
	if err != nil {
		result.Issues = append(result.Issues, ValidationIssue{
			Severity: SeverityError,
			Message:  fmt.Sprintf("read config: %v", err),
		})
		result.Valid = false
		return result, nil
	}

	if len(bytes.TrimSpace(data)) == 0 {
		result.Issues = append(result.Issues, ValidationIssue{
			Severity: SeverityWarning,
			Message:  "configuration file is empty",
		})
		result.Valid = true
		return result, nil
	}

	// 2. Check for TOML table order issue: top-level keys declared after table headers
	checkTableOrdering(data, result)

	// 3. TOML syntax and structure unmarshaling
	var cf configFile
	if err := toml.Unmarshal(data, &cf); err != nil {
		result.Issues = append(result.Issues, ValidationIssue{
			Severity: SeverityError,
			Message:  fmt.Sprintf("syntax error: %v", err),
		})
		result.Valid = false
		return result, nil
	}

	// 4. Normalization (token budget units, fallback threshold ranges)
	if err := cf.normalize(); err != nil {
		result.Issues = append(result.Issues, ValidationIssue{
			Severity: SeverityError,
			Message:  fmt.Sprintf("invalid value: %v", err),
		})
		result.Valid = false
		return result, nil
	}

	configDir := filepath.Dir(targetPath)
	effectiveRoot := rootDir
	if effectiveRoot == "" {
		effectiveRoot = configDir
	}

	// 5. Semantic Validation
	validateSemantic(&cf, effectiveRoot, result)

	// 6. Security escapes check against the parsed file, not a reload of
	// <effectiveRoot>/.sift.toml.
	if effectiveRoot != "" {
		for _, escape := range checkParsedConfig(cf, effectiveRoot) {
			result.Issues = append(result.Issues, ValidationIssue{
				Severity: SeverityError,
				Field:    "security",
				Message:  escape,
			})
		}
	}

	result.Valid = len(result.Errors()) == 0

	// 7. Populate configuration summary
	buildSummary(&cf, targetPath, result)

	return result, nil
}

// checkTableOrdering warns if bare top-level keys like default_profile appear after a table header.
func checkTableOrdering(data []byte, result *CheckResult) {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	currentTable := ""
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			currentTable = line
			continue
		}

		if currentTable != "" {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				if key == "default_profile" {
					result.Issues = append(result.Issues, ValidationIssue{
						Severity: SeverityError,
						Field:    "default_profile",
						Message: fmt.Sprintf("line %d: 'default_profile' appears inside %s; in TOML, bare top-level keys must appear before any table header",
							lineNum, currentTable),
					})
				}
			}
		}
	}
}

func validateSemantic(cf *configFile, root string, result *CheckResult) {
	validStyles := map[string]bool{
		"xml":      true,
		"markdown": true,
		"md":       true,
		"json":     true,
		"plain":    true,
		"txt":      true,
		"text":     true,
		"raw":      true,
	}

	validModes := map[string]bool{
		"":           true,
		"full":       true,
		"signatures": true,
		"sigs":       true,
		"outline":    true,
	}

	// Check default_profile reference
	if cf.DefaultProfile != "" {
		if cf.Profiles == nil {
			result.Issues = append(result.Issues, ValidationIssue{
				Severity: SeverityError,
				Field:    "default_profile",
				Message:  fmt.Sprintf("references nonexistent profile [profiles.%s]", cf.DefaultProfile),
			})
		} else if _, exists := cf.Profiles[cf.DefaultProfile]; !exists {
			result.Issues = append(result.Issues, ValidationIssue{
				Severity: SeverityError,
				Field:    "default_profile",
				Message:  fmt.Sprintf("references nonexistent profile [profiles.%s]", cf.DefaultProfile),
			})
		}
	}

	// Check [sift] defaults
	if cf.Sift != nil {
		if cf.Sift.Style != "" && !validStyles[strings.ToLower(cf.Sift.Style)] {
			result.Issues = append(result.Issues, ValidationIssue{
				Severity: SeverityError,
				Field:    "[sift].style",
				Message:  fmt.Sprintf("invalid style %q (supported: xml, markdown, json, plain)", cf.Sift.Style),
			})
		}
		if cf.Sift.Mode != "" && !validModes[strings.ToLower(cf.Sift.Mode)] {
			result.Issues = append(result.Issues, ValidationIssue{
				Severity: SeverityError,
				Field:    "[sift].compress_mode",
				Message:  fmt.Sprintf("invalid compress_mode %q (supported: full, signatures, outline)", cf.Sift.Mode),
			})
		}
		if cf.Sift.FallbackMode != "" && cf.Sift.FallbackMode != "signatures" && cf.Sift.FallbackMode != "sigs" {
			result.Issues = append(result.Issues, ValidationIssue{
				Severity: SeverityError,
				Field:    "[sift].fallback_mode",
				Message:  fmt.Sprintf("invalid fallback_mode %q (expected \"signatures\")", cf.Sift.FallbackMode),
			})
		}
		if cf.Sift.FallbackThreshold != nil && (*cf.Sift.FallbackThreshold < 0 || *cf.Sift.FallbackThreshold > 1.0) {
			result.Issues = append(result.Issues, ValidationIssue{
				Severity: SeverityError,
				Field:    "[sift].fallback_threshold",
				Message:  fmt.Sprintf("fallback_threshold must be between 0.0 and 1.0 (got %v)", *cf.Sift.FallbackThreshold),
			})
		}
		if cf.Sift.Prompt != "" && cf.Sift.PromptFile != "" {
			result.Issues = append(result.Issues, ValidationIssue{
				Severity: SeverityError,
				Field:    "[sift]",
				Message:  "specify either prompt or prompt_file, not both",
			})
		}
		if cf.Sift.PromptFile != "" {
			checkFileExists(root, cf.Sift.PromptFile, "[sift].prompt_file", result)
		}
		if cf.Sift.UIThemeFile != "" {
			checkFileExists(root, cf.Sift.UIThemeFile, "[sift].ui_theme_file", result)
		}
	}

	// Check [profiles]
	if len(cf.Profiles) > 0 {
		for pName, p := range cf.Profiles {
			prefix := fmt.Sprintf("profiles.%s", pName)
			if p.Style != "" && !validStyles[strings.ToLower(p.Style)] {
				result.Issues = append(result.Issues, ValidationIssue{
					Severity: SeverityError,
					Field:    prefix + ".style",
					Message:  fmt.Sprintf("invalid style %q", p.Style),
				})
			}
			if p.Mode != "" && !validModes[strings.ToLower(p.Mode)] {
				result.Issues = append(result.Issues, ValidationIssue{
					Severity: SeverityError,
					Field:    prefix + ".compress_mode",
					Message:  fmt.Sprintf("invalid compress_mode %q", p.Mode),
				})
			}
			if p.FallbackMode != "" && p.FallbackMode != "signatures" && p.FallbackMode != "sigs" {
				result.Issues = append(result.Issues, ValidationIssue{
					Severity: SeverityError,
					Field:    prefix + ".fallback_mode",
					Message:  fmt.Sprintf("invalid fallback_mode %q (expected \"signatures\")", p.FallbackMode),
				})
			}
			if p.FallbackThreshold != nil && (*p.FallbackThreshold < 0 || *p.FallbackThreshold > 1.0) {
				result.Issues = append(result.Issues, ValidationIssue{
					Severity: SeverityError,
					Field:    prefix + ".fallback_threshold",
					Message:  fmt.Sprintf("fallback_threshold must be between 0.0 and 1.0 (got %v)", *p.FallbackThreshold),
				})
			}
			if p.Prompt != "" && p.PromptFile != "" {
				result.Issues = append(result.Issues, ValidationIssue{
					Severity: SeverityError,
					Field:    prefix,
					Message:  "specify either prompt or prompt_file, not both",
				})
			}
			if p.PromptFile != "" {
				checkFileExists(root, p.PromptFile, prefix+".prompt_file", result)
			}
			for _, ancestor := range p.Extends {
				if _, ok := cf.Profiles[ancestor]; !ok {
					result.Issues = append(result.Issues, ValidationIssue{
						Severity: SeverityError,
						Field:    prefix + ".extends",
						Message:  fmt.Sprintf("extends nonexistent profile %q", ancestor),
					})
				}
			}
		}

		// Check for circular inheritance
		if _, err := resolveExtends(cf.Profiles); err != nil {
			result.Issues = append(result.Issues, ValidationIssue{
				Severity: SeverityError,
				Field:    "profiles",
				Message:  fmt.Sprintf("profile inheritance: %v", err),
			})
		}
	}

	// Check [modes] mapping
	modes := cf.Modes
	if modes == nil && cf.Sift != nil {
		modes = cf.Sift.Modes
	}
	if modes != nil {
		validateModes(modes, result)
	}

	// Check [[targets]]
	seenTargets := make(map[string]bool)
	for i, t := range cf.Targets {
		prefix := fmt.Sprintf("targets[%d]", i)
		if t.Name != "" {
			prefix = fmt.Sprintf("target %q", t.Name)
			if seenTargets[t.Name] {
				result.Issues = append(result.Issues, ValidationIssue{
					Severity: SeverityError,
					Field:    prefix,
					Message:  fmt.Sprintf("duplicate target name %q", t.Name),
				})
			}
			seenTargets[t.Name] = true
		} else {
			result.Issues = append(result.Issues, ValidationIssue{
				Severity: SeverityError,
				Field:    prefix,
				Message:  "target missing required 'name'",
			})
		}

		if t.Profile != "" {
			if _, ok := cf.Profiles[t.Profile]; !ok {
				result.Issues = append(result.Issues, ValidationIssue{
					Severity: SeverityError,
					Field:    prefix + ".profile",
					Message:  fmt.Sprintf("target references nonexistent profile %q", t.Profile),
				})
			}
		}

		if t.Style != "" && !validStyles[strings.ToLower(t.Style)] {
			result.Issues = append(result.Issues, ValidationIssue{
				Severity: SeverityError,
				Field:    prefix + ".style",
				Message:  fmt.Sprintf("invalid style %q", t.Style),
			})
		}

		if t.Prompt != "" && t.PromptFile != "" {
			result.Issues = append(result.Issues, ValidationIssue{
				Severity: SeverityError,
				Field:    prefix,
				Message:  "specify either prompt or prompt_file, not both",
			})
		}
		if t.PromptFile != "" {
			checkFileExists(root, t.PromptFile, prefix+".prompt_file", result)
		}
		if t.Mode != "" && !validModes[strings.ToLower(t.Mode)] {
			result.Issues = append(result.Issues, ValidationIssue{
				Severity: SeverityError,
				Field:    prefix + ".compress_mode",
				Message:  fmt.Sprintf("invalid compress_mode %q (supported: full, signatures, outline)", t.Mode),
			})
		}
		if t.FallbackMode != "" && t.FallbackMode != "signatures" {
			result.Issues = append(result.Issues, ValidationIssue{
				Severity: SeverityError,
				Field:    prefix + ".fallback_mode",
				Message:  fmt.Sprintf("invalid fallback_mode %q (expected \"signatures\")", t.FallbackMode),
			})
		}
		if t.FallbackThreshold != nil && (*t.FallbackThreshold < 0 || *t.FallbackThreshold > 1.0) {
			result.Issues = append(result.Issues, ValidationIssue{
				Severity: SeverityError,
				Field:    prefix + ".fallback_threshold",
				Message:  fmt.Sprintf("fallback_threshold must be between 0.0 and 1.0 (got %v)", *t.FallbackThreshold),
			})
		}
		if t.Modes != nil {
			validateModes(t.Modes, result)
		}
	}

	// Check [prompts]
	for pName, pRef := range cf.Prompts {
		prefix := fmt.Sprintf("prompts.%s", pName)
		if pRef.Text != "" && pRef.File != "" {
			result.Issues = append(result.Issues, ValidationIssue{
				Severity: SeverityError,
				Field:    prefix,
				Message:  "specify either text or file, not both",
			})
		}
		if pRef.File != "" {
			checkFileExists(root, pRef.File, prefix+".file", result)
		}
	}
}

func validateModes(m *ModesConfig, result *CheckResult) {
	seen := make(map[string]string)
	checkList := func(list []string, modeName string) {
		for _, pat := range list {
			if prev, exists := seen[pat]; exists && prev != modeName {
				result.Issues = append(result.Issues, ValidationIssue{
					Severity: SeverityWarning,
					Field:    "[modes]",
					Message:  fmt.Sprintf("pattern %q appears in both %s and %s (overlap precedence applies)", pat, prev, modeName),
				})
			}
			seen[pat] = modeName
		}
	}
	checkList(m.Full, "full")
	checkList(m.Outline, "outline")
	checkList(m.Signatures, "signatures")
	checkList(m.Skip, "skip")
}

func checkFileExists(root, relPath, field string, result *CheckResult) {
	if relPath == "" {
		return
	}
	target := relPath
	if !filepath.IsAbs(target) && root != "" {
		target = filepath.Join(root, relPath)
	}
	if _, err := os.Stat(target); err != nil {
		result.Issues = append(result.Issues, ValidationIssue{
			Severity: SeverityWarning,
			Field:    field,
			Message:  fmt.Sprintf("referenced file %q not found on disk", relPath),
		})
	}
}

func buildSummary(cf *configFile, path string, result *CheckResult) {
	s := ConfigSummary{
		Path:           path,
		DefaultProfile: cf.DefaultProfile,
	}

	if cf.Sift != nil {
		s.Style = cf.Sift.Style
		s.Output = cf.Sift.Output
		s.Budget = cf.Sift.Budget
		s.FallbackMode = cf.Sift.FallbackMode
		s.FallbackThreshold = cf.Sift.FallbackThreshold
		s.IgnoreCount = len(cf.Sift.Ignore)
	}

	for pName := range cf.Profiles {
		s.Profiles = append(s.Profiles, pName)
	}
	sort.Strings(s.Profiles)

	for _, t := range cf.Targets {
		s.Targets = append(s.Targets, t.Name)
	}

	modes := cf.Modes
	if modes == nil && cf.Sift != nil {
		modes = cf.Sift.Modes
	}
	if modes != nil && (len(modes.Full) > 0 || len(modes.Signatures) > 0 || len(modes.Skip) > 0) {
		s.Modes = &ModesSummary{
			FullCount: len(modes.Full),
			SigsCount: len(modes.Signatures),
			SkipCount: len(modes.Skip),
		}
	}

	result.Summary = s
}
