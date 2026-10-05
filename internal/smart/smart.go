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
	"sync"

	"github.com/bethropolis/sift/internal/lang"
)

// DefaultMaxTokens is the per-file guardrail used when no limit is given.
const DefaultMaxTokens = 15000

// Evaluator checks files against the token guardrail and generated headers,
// delegating language-specific rules to the central lang registry.
type Evaluator struct {
	maxTokens int

	// pathDecisions memoizes ShouldSkipPath by exact path: the underlying
	// rules are a pure function of the path (name/language only), and the
	// skeleton pass, the walker pre-read filter, and the content processor
	// all consult the same paths within one scan. sync.Map keeps the
	// concurrent walker workers lock-free on the hot path.
	pathDecisions sync.Map // string -> pathDecision
}

// pathDecision is a memoized ShouldSkipPath result.
type pathDecision struct {
	skip   bool
	reason string
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
// Results are memoized per path: every pass that sees the same file (walker
// pre-read filter, content processor, skeleton) shares the first answer.
func (e *Evaluator) ShouldSkipPath(path string) (bool, string) {
	if v, ok := e.pathDecisions.Load(path); ok {
		d := v.(pathDecision)
		return d.skip, d.reason
	}
	filename := strings.ToLower(filepath.Base(path))
	normPath := strings.ToLower(filepath.ToSlash(path))
	skip, reason := lang.ShouldSkipSmart(normPath, filename, nil)
	e.pathDecisions.Store(path, pathDecision{skip: skip, reason: reason})
	return skip, reason
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
	if skip, reason := e.ExceedsTokenLimit(tokens); skip {
		return true, reason
	}

	// 2. Generated header detection ("DO NOT EDIT", "@generated", etc.).
	if IsGeneratedHeader(content) {
		return true, "Auto-generated file header detected"
	}

	// 3. Language-specific rules.
	return e.ShouldSkipPath(path)
}

// ExceedsTokenLimit reports whether tokens breaches the per-file guardrail.
// It lets callers that already ran the header and name checks skip straight
// to the count check without re-running them.
func (e *Evaluator) ExceedsTokenLimit(tokens int) (bool, string) {
	if tokens > e.maxTokens {
		return true, fmt.Sprintf("Exceeds smart token limit (%d tokens)", e.maxTokens)
	}
	return false, ""
}
