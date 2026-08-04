// Package scan owns the per-file processing pipeline: smart filtering, token
// counting, and optional signature compression. It turns a file's bytes into
// an enriched FileEntry for the collection and picker flows, so walkers and
// renderers stay free of tokenizer/compressor details.
package scan

import (
	"errors"
	"fmt"

	"github.com/bethropolis/sift/internal/compress"
	"github.com/bethropolis/sift/internal/format"
	"github.com/bethropolis/sift/internal/logger"
	"github.com/bethropolis/sift/internal/smart"
	"github.com/bethropolis/sift/internal/tokenize"
)

// Mode selects how a file is processed.
type Mode int

const (
	// ModeFull keeps the raw content with a single token count. Used by
	// dump/diff/watch when no signature mode is requested.
	ModeFull Mode = iota
	// ModePicker retains both the full and signature variants plus their
	// token counts, leaving the active view to the caller. The entry's Tokens
	// is left unset (-1); selection reconciles it with the chosen view.
	ModePicker
	// ModeSignatures renders the compressed signature summary into Content,
	// with token counts reflecting the compressed text.
	ModeSignatures
)

// ErrFileSkipped is returned by Process when the smart filter rejects the
// file. The specific reason is wrapped so callers can report or ignore it.
// It distinguishes a deliberate skip from a processing failure.
var ErrFileSkipped = errors.New("file skipped by smart filter")

// Options configures a Processor.
type Options struct {
	TokenizeModel  string
	SmartFilter    bool
	SmartMaxTokens int
	// Compress enables signature compression. It is off for ordinary
	// full-content scans, which never touch the compressor, so the tree-sitter
	// grammars and parser pools are not loaded unnecessarily.
	Compress bool
	Logger   *logger.Logger
}

// Processor transforms a file's content into an enriched FileEntry.
//
// The tokenizer, compressor, and smart evaluator are shared across Process
// calls. The tokenizer's codec is safe for concurrent Count calls (its
// regexp runner comes from a sync.Pool), and the compressor pools its
// tree-sitter parsers, so one Processor may be used by many workers.
type Processor struct {
	tokenizer  *tokenize.Tokenizer
	compressor *compress.Compressor
	evaluator  *smart.Evaluator
	log        *logger.Logger
}

// New returns a Processor for the given options. Tokenizer construction can
// fail, hence the error return.
func New(opts Options) (*Processor, error) {
	tokenizer, err := tokenize.New(opts.TokenizeModel)
	if err != nil {
		return nil, err
	}
	p := &Processor{
		tokenizer: tokenizer,
		log:       opts.Logger,
	}
	if opts.Compress {
		p.compressor = compress.New()
	}
	if opts.SmartFilter {
		p.evaluator = smart.New(opts.SmartMaxTokens)
	}
	return p, nil
}

// Process enriches one file's content according to mode. It returns
// ErrFileSkipped (wrapping the smart-filter reason) when the file is
// rejected by name, content, or token guardrail. Token-counting failures log
// a warning and yield a zero count so a counting error never aborts a scan.
func (p *Processor) Process(path string, content []byte, mode Mode) (format.FileEntry, error) {
	smartTokens := -1
	if p.evaluator != nil {
		// Name and language rules run before any tokenizing so lockfiles and
		// generated bundles never cost BPE work.
		if skip, reason := p.evaluator.ShouldSkipPath(path); skip {
			return format.FileEntry{}, fmt.Errorf("%w: %s", ErrFileSkipped, reason)
		}
		smartTokens = p.countTokens(content, path)
		if skip, reason := p.evaluator.ShouldSkip(path, content, smartTokens); skip {
			return format.FileEntry{}, fmt.Errorf("%w: %s", ErrFileSkipped, reason)
		}
	}

	entry := format.FileEntry{
		Path:    path,
		Content: content,
		Tokens:  -1,
	}

	switch mode {
	case ModePicker:
		// Reuse the smart filter's count as TokensFull to avoid counting
		// twice when the filter is active.
		entry.TokensFull = smartTokens
		if smartTokens < 0 {
			entry.TokensFull = p.countTokens(content, path)
		}
		if p.compressor != nil {
			if lang, ok := p.compressor.LanguageForPath(path); ok {
				if compressed, didCompress := p.compressor.Compress(content, lang); didCompress {
					entry.SigContent = []byte(compressed)
					entry.Language = string(lang)
				}
			}
		}
		if entry.SigContent != nil {
			entry.TokensSig = p.countTokens(entry.SigContent, path)
		} else {
			entry.TokensSig = entry.TokensFull
		}

	case ModeSignatures:
		if p.compressor != nil {
			if lang, ok := p.compressor.LanguageForPath(path); ok {
				if compressed, didCompress := p.compressor.Compress(content, lang); didCompress {
					content = []byte(compressed)
					entry.Content = content
					entry.IsCompressed = true
					entry.Language = string(lang)
				}
			}
		}
		entry.Tokens = p.countTokens(content, path)
		entry.TokensFull = entry.Tokens
		entry.TokensSig = entry.Tokens

	case ModeFull:
		entry.Tokens = p.countTokens(content, path)
		entry.TokensFull = entry.Tokens
		entry.TokensSig = entry.Tokens
	}

	return entry, nil
}

// countTokens counts tokens for content, logging a warning on failure and
// returning 0 so a counting error never aborts the scan.
func (p *Processor) countTokens(content []byte, path string) int {
	tokens, err := p.tokenizer.Count(content)
	if err != nil {
		if p.log != nil {
			p.log.Warn("Failed to count tokens for %s: %v", path, err)
		}
		return 0
	}
	return tokens
}
