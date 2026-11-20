package config

import (
	"path/filepath"
	"strings"

	gitignore "github.com/denormal/go-gitignore"
)

// ModesConfig holds path-based compression and filtering rules.
type ModesConfig struct {
	Full       StringList `toml:"full"`
	Signatures StringList `toml:"signatures"`
	Outline    StringList `toml:"outline"`
	Skip       StringList `toml:"skip"`
}

// ModeDecision represents the compression mode assigned to a specific file.
type ModeDecision int

const (
	ModeDecisionDefault ModeDecision = iota
	ModeDecisionSkip
	ModeDecisionFull
	ModeDecisionOutline
	ModeDecisionSignatures
)

// ModesMatcher matches file paths against glob rules defined in ModesConfig.
type ModesMatcher struct {
	skip       gitignore.GitIgnore
	full       gitignore.GitIgnore
	outline    gitignore.GitIgnore
	signatures gitignore.GitIgnore
}

// NewModesMatcher constructs a matcher from ModesConfig. Returns nil if no rules are defined.
func NewModesMatcher(rootDir string, m ModesConfig) *ModesMatcher {
	if len(m.Skip) == 0 && len(m.Full) == 0 && len(m.Outline) == 0 && len(m.Signatures) == 0 {
		return nil
	}
	mm := &ModesMatcher{}
	if len(m.Skip) > 0 {
		mm.skip = gitignore.New(strings.NewReader(strings.Join([]string(m.Skip), "\n")), rootDir, nil)
	}
	if len(m.Full) > 0 {
		mm.full = gitignore.New(strings.NewReader(strings.Join([]string(m.Full), "\n")), rootDir, nil)
	}
	if len(m.Outline) > 0 {
		mm.outline = gitignore.New(strings.NewReader(strings.Join([]string(m.Outline), "\n")), rootDir, nil)
	}
	if len(m.Signatures) > 0 {
		mm.signatures = gitignore.New(strings.NewReader(strings.Join([]string(m.Signatures), "\n")), rootDir, nil)
	}
	return mm
}

// Match tests a path relative to the root directory and returns the assigned mode decision.
// Precedence order: skip > full > outline > signatures > default.
func (mm *ModesMatcher) Match(relPath string) ModeDecision {
	if mm == nil {
		return ModeDecisionDefault
	}
	rel := filepath.ToSlash(relPath)
	if mm.skip != nil {
		if match := mm.skip.Relative(rel, false); match != nil && match.Ignore() {
			return ModeDecisionSkip
		}
	}
	if mm.full != nil {
		if match := mm.full.Relative(rel, false); match != nil && match.Ignore() {
			return ModeDecisionFull
		}
	}
	if mm.outline != nil {
		if match := mm.outline.Relative(rel, false); match != nil && match.Ignore() {
			return ModeDecisionOutline
		}
	}
	if mm.signatures != nil {
		if match := mm.signatures.Relative(rel, false); match != nil && match.Ignore() {
			return ModeDecisionSignatures
		}
	}
	return ModeDecisionDefault
}
