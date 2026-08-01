// Package ignore provides file/directory pattern matching for exclusion
package ignore

import (
	"github.com/bethropolis/sift/internal/utils"
	gitignore "github.com/denormal/go-gitignore"
)

// IgnoreMatcher determines whether a file or directory should be ignored
type IgnoreMatcher struct {
	// The core gitignore object handling repository rules
	repoIgnore gitignore.GitIgnore

	// Matcher built from custom patterns (e.g., -ignore flag)
	customIgnore gitignore.GitIgnore

	// Matcher built from the built-in default patterns. It is consulted last,
	// only when neither custom nor repository rules decided the path.
	defaultIgnore gitignore.GitIgnore

	// Configuration flags
	rootDir        string
	ignoreHidden   bool
	ignoreGit      bool
	recursiveMode  bool
	customPatterns []string
	logger         utils.Logger
	disabled       bool
}

// Config holds configuration options for the ignore matcher
type Config struct {
	RootDir       string
	IgnoreHidden  bool
	IgnoreGit     bool
	RecursiveMode bool
	CustomRules   []string
	Logger        utils.Logger
	Disabled      bool
}
