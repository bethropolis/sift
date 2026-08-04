// Package smart filters files that are unlikely to be useful LLM context:
// generated artifacts, lockfiles, minified bundles, and oversized files that
// .gitignore often misses. It relies on filename heuristics, generated-header
// sniffing, and a per-file token guardrail. The language-specific rules live
// in internal/lang.
package smart

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/bethropolis/sift/internal/lang"
)

// DefaultMaxTokens is the per-file guardrail used when no limit is given.
const DefaultMaxTokens = 15000

// Evaluator checks files against the token guardrail and generated headers,
// delegating language-specific rules to the central lang registry.
type Evaluator struct {
	maxTokens int
}

// New returns an Evaluator using the given per-file token ceiling. A
// non-positive maxTokens falls back to DefaultMaxTokens.
func New(maxTokens int) *Evaluator {
	if maxTokens <= 0 {
		maxTokens = DefaultMaxTokens
	}
	return &Evaluator{maxTokens: maxTokens}
}

// ShouldSkipPath reports whether a file should be excluded based on its name
// alone, without reading or tokenizing its content. It lets callers drop
// lockfiles, generated artifacts, and bundles before any I/O or CPU work.
func (e *Evaluator) ShouldSkipPath(path string) (bool, string) {
	filename := strings.ToLower(filepath.Base(path))
	normPath := strings.ToLower(filepath.ToSlash(path))
	return lang.ShouldSkipSmart(normPath, filename, nil)
}

// ShouldSkipMeta reports whether a file should be excluded using only its
// path and an approximate token count (e.g. size/4). It backs the picker's
// structure-first pass, letting the skeleton skip files that name rules or
// the token guardrail would drop, without reading any content.
func (e *Evaluator) ShouldSkipMeta(path string, approxTokens int) (bool, string) {
	if approxTokens > e.maxTokens {
		return true, fmt.Sprintf("Exceeds smart token limit (%d tokens)", e.maxTokens)
	}
	return e.ShouldSkipPath(path)
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
	return e.ShouldSkipPath(path)
}
