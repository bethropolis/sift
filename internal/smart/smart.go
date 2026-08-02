// Package smart filters files that are unlikely to be useful LLM context:
// generated artifacts, lockfiles, minified bundles, and oversized files that
// .gitignore often misses. It relies on filename heuristics, generated-header
// sniffing, and a per-file token guardrail.
package smart

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/bethropolis/sift/internal/smart/language"
)

// DefaultMaxTokens is the per-file guardrail used when no limit is given.
const DefaultMaxTokens = 15000

// Evaluator checks files against the token guardrail, generated headers, and
// registered language rules.
type Evaluator struct {
	maxTokens int
	rules     []language.Rule
}

// New returns an Evaluator using the given per-file token ceiling. A
// non-positive maxTokens falls back to DefaultMaxTokens.
func New(maxTokens int) *Evaluator {
	if maxTokens <= 0 {
		maxTokens = DefaultMaxTokens
	}
	return &Evaluator{
		maxTokens: maxTokens,
		rules:     language.AllRules(),
	}
}

// ShouldSkipPath reports whether a file should be excluded based on its name
// alone, without reading or tokenizing its content. It lets callers drop
// lockfiles, generated artifacts, and bundles before any I/O or CPU work.
func (e *Evaluator) ShouldSkipPath(path string) (bool, string) {
	filename := strings.ToLower(filepath.Base(path))
	normPath := strings.ToLower(filepath.ToSlash(path))

	for _, rule := range e.rules {
		if rule(normPath, filename, nil) {
			return true, "Matched smart language filter"
		}
	}
	return false, ""
}

// ShouldSkip reports whether a file should be excluded by the smart filter,
// along with a human-readable reason. Name-based rules run here too, but
// callers that already ran ShouldSkipPath can avoid the redundant check by
// only invoking ShouldSkip on surviving candidates.
func (e *Evaluator) ShouldSkip(path string, content []byte, tokens int) (bool, string) {
	// 1. Token threshold guardrail.
	if tokens > e.maxTokens {
		return true, fmt.Sprintf("Exceeds smart token limit (%d tokens)", e.maxTokens)
	}

	// 2. Generated header detection ("DO NOT EDIT", "@generated", etc.).
	if IsGeneratedHeader(content) {
		return true, "Auto-generated file header detected"
	}

	// 3. Language-specific rules.
	if skip, reason := e.ShouldSkipPath(path); skip {
		return true, reason
	}

	return false, ""
}
