// Package ignore provides file/directory pattern matching for exclusion
//
// This package handles advanced file/directory exclusion patterns based on
// multiple criteria including .gitignore rules, hidden files, and custom patterns.
// It uses the functional options pattern for configuration.
package ignore

func NewDefaultMatcher(rootDir string) (*IgnoreMatcher, error) {
	return New(rootDir)
}

func NewFromConfig(cfg Config) (*IgnoreMatcher, error) {
	options := []Option{
		WithHiddenIgnore(cfg.IgnoreHidden),
		WithGitIgnore(cfg.IgnoreGit),
		WithRecursive(cfg.RecursiveMode),
		WithDisabled(cfg.Disabled),
	}

	if cfg.CustomRules != nil && len(cfg.CustomRules) > 0 {
		options = append(options, WithCustomRules(cfg.CustomRules))
	}

	if cfg.Logger != nil {
		options = append(options, WithLogger(cfg.Logger))
	}

	return New(cfg.RootDir, options...)
}

func CreateDisabledMatcher() *IgnoreMatcher {
	matcher, _ := New(".", WithDisabled(true))
	return matcher
}

func IsIgnored(matcher *IgnoreMatcher, path string, isDir bool) bool {
	if matcher == nil {
		return false
	}
	return matcher.ShouldIgnore(path, isDir)
}
