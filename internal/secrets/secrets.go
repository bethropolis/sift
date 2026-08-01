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
}

// New returns a Scanner backed by the generated gitleaks rules.
func New() *Scanner {
	return &Scanner{rules: CompiledRules}
}

// RedactContent scans and sanitizes content for a single file. The keyword
// fast-path skips a rule's regex unless at least one of its keywords appears
// in the (case-folded) content.
func (s *Scanner) RedactContent(content []byte) ([]byte, []Detection) {
	if len(content) == 0 {
		return content, nil
	}

	contentLower := bytes.ToLower(content)
	var detections []Detection
	redacted := string(content)

	for _, rule := range s.rules {
		if len(rule.Keywords) > 0 {
			hasKeyword := false
			for _, kw := range rule.Keywords {
				if bytes.Contains(contentLower, []byte(strings.ToLower(kw))) {
					hasKeyword = true
					break
				}
			}
			if !hasKeyword {
				continue
			}
		}

		matches := rule.Regex.FindAllStringIndex(redacted, -1)
		if len(matches) == 0 {
			continue
		}

		// Replace from the end so earlier indices stay valid.
		for i := len(matches) - 1; i >= 0; i-- {
			start, end := matches[i][0], matches[i][1]
			match := redacted[start:end]
			detections = append(detections, Detection{RuleName: rule.Name, Match: match})
			redacted = redacted[:start] + "[REDACTED_SECRET: " + rule.Name + "]" + redacted[end:]
		}
	}

	return []byte(redacted), detections
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
