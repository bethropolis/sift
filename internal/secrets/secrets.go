// Package secrets detects and redacts credential-like strings before output.
package secrets

import (
	"bytes"
	"regexp"
	"runtime"
	"strings"
	"sync"

	"github.com/bethropolis/sift/internal/format"
)

// Detection describes a single secret found in a file.
type Detection struct {
	RuleName string
	Match    string
}

// Rule is a named regex used to find secrets. When Keywords is non-empty, the
// regex is only evaluated if at least one keyword appears in the content.
type Rule struct {
	Name     string
	Keywords []string
	Regex    *regexp.Regexp
}

// Scanner scans content against the compiled gitleaks rules.
type Scanner struct {
	rules []Rule

	// prepared lazily lowercases every rule keyword exactly once, so the
	// per-file hot path never re-lowercases the same literals for every rule.
	prepared     sync.Once
	keywordBytes [][][]byte // per rule index; nil when the rule has no keywords
}

// New returns a Scanner backed by the generated gitleaks rules.
func New() *Scanner {
	return &Scanner{rules: CompiledRules}
}

// prepare computes the lowercase keyword forms on first use.
func (s *Scanner) prepare() {
	s.prepared.Do(func() {
		s.keywordBytes = make([][][]byte, len(s.rules))
		for i, rule := range s.rules {
			if len(rule.Keywords) == 0 {
				continue
			}
			kws := make([][]byte, len(rule.Keywords))
			for j, kw := range rule.Keywords {
				kws[j] = []byte(strings.ToLower(kw))
			}
			s.keywordBytes[i] = kws
		}
	})
}

// RedactContent scans and sanitizes content for a single file. The keyword
// fast-path skips a rule's regex unless at least one of its keywords appears
// in the (case-folded) content. Content without detections (the common case)
// is returned untouched; the working copy is materialized only when the first
// rule matches.
func (s *Scanner) RedactContent(content []byte) ([]byte, []Detection) {
	if len(content) == 0 {
		return content, nil
	}
	s.prepare()

	contentLower := bytes.ToLower(content)
	var detections []Detection
	// redacted stays nil until the first rule matches. Once set, every later
	// rule scans this evolving copy, exactly like the old always-copy string
	// version, but clean files never pay for it.
	var redacted []byte

	for i, rule := range s.rules {
		if kws := s.keywordBytes[i]; len(kws) > 0 {
			hasKeyword := false
			for _, kw := range kws {
				if bytes.Contains(contentLower, kw) {
					hasKeyword = true
					break
				}
			}
			if !hasKeyword {
				continue
			}
		}

		src := content
		if redacted != nil {
			src = redacted
		}

		// A rule with a filter reports only the span holding the secret, and
		// may reject matches entirely (generic-api-key's entropy/stopwords).
		// Rules without a filter redact the whole match as before.
		spans, candidates := matchRanges(rule, src)
		if len(spans) == 0 {
			continue
		}
		if redacted == nil {
			// Materialize the working copy only now; the byte-identical copy
			// keeps every match index valid.
			redacted = append([]byte(nil), content...)
			src = redacted
		}

		// Replace from the end so earlier indices stay valid.
		for j := len(spans) - 1; j >= 0; j-- {
			start, end := spans[j][0], spans[j][1]
			detections = append(detections, Detection{
				RuleName: rule.Name,
				Match:    string(candidates[j]),
			})
			marker := "[REDACTED_SECRET: " + rule.Name + "]"
			spliced := make([]byte, 0, len(src)-(end-start)+len(marker))
			spliced = append(spliced, src[:start]...)
			spliced = append(spliced, marker...)
			spliced = append(spliced, src[end:]...)
			redacted = spliced
			src = spliced
		}
	}

	if redacted == nil {
		return content, nil
	}
	return redacted, detections
}

// matchRanges returns the byte spans to redact for one rule plus the text of
// each span. Filtered rules (see ruleFilters) redact only their secret capture
// group and drop candidates that fail the filter; unfiltered rules redact the
// whole match.
func matchRanges(rule Rule, src []byte) (spans [][2]int, candidates [][]byte) {
	filter, filtered := ruleFilters[rule.Name]
	if !filtered {
		for _, m := range rule.Regex.FindAllIndex(src, -1) {
			spans = append(spans, [2]int{m[0], m[1]})
			candidates = append(candidates, src[m[0]:m[1]])
		}
		return spans, candidates
	}

	group := filter.SecretGroup
	for _, m := range rule.Regex.FindAllSubmatchIndex(src, -1) {
		if 2*group+1 >= len(m) {
			continue
		}
		start, end := m[2*group], m[2*group+1]
		if start < 0 || end <= start {
			continue
		}
		candidate := src[start:end]
		if !filter.accept(candidate) {
			continue
		}
		spans = append(spans, [2]int{start, end})
		candidates = append(candidates, candidate)
	}
	return spans, candidates
}

// RedactSelectedFiles runs secret scanning concurrently over the given files.
// Each worker sanitizes a disjoint slice of the returned copy, so no entry is
// ever mutated concurrently. SecretCount is set to the number of detections.
func (s *Scanner) RedactSelectedFiles(files []format.FileEntry) []format.FileEntry {
	if len(files) == 0 {
		return files
	}

	workers := runtime.NumCPU()
	if workers > len(files) {
		workers = len(files)
	}

	jobs := make(chan int, len(files))
	for i := range files {
		jobs <- i
	}
	close(jobs)

	var wg sync.WaitGroup
	out := make([]format.FileEntry, len(files))
	copy(out, files)

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range jobs {
				sanitized, detections := s.RedactContent(out[idx].Content)
				out[idx].Content = sanitized
				out[idx].SecretCount = len(detections)
			}
		}()
	}

	wg.Wait()
	return out
}
