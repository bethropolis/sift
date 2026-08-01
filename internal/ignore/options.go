package ignore

import "github.com/bethropolis/sift/internal/utils"

// Option functions for configuration
type Option func(*IgnoreMatcher)

func WithHiddenIgnore(ignore bool) Option {
	return func(m *IgnoreMatcher) {
		m.ignoreHidden = ignore
	}
}

func WithGitIgnore(ignore bool) Option {
	return func(m *IgnoreMatcher) {
		m.ignoreGit = ignore
	}
}

func WithRecursive(recursive bool) Option {
	return func(m *IgnoreMatcher) {
		m.recursiveMode = recursive
	}
}

func WithCustomRules(patterns []string) Option {
	return func(m *IgnoreMatcher) {
		m.customPatterns = patterns
	}
}

func WithLogger(logger utils.Logger) Option {
	return func(m *IgnoreMatcher) {
		if logger != nil {
			m.logger = logger
		}
	}
}

func WithDisabled(disabled bool) Option {
	return func(m *IgnoreMatcher) {
		m.disabled = disabled
	}
}
