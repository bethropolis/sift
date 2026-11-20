package app

import (
	"bytes"
	"strings"

	"github.com/bethropolis/sift/internal/config"
	"github.com/bethropolis/sift/internal/format"
	"github.com/bethropolis/sift/internal/scan"
	"github.com/bethropolis/sift/internal/tokenize"
)

// fitToBudget selects the largest ranked prefix whose rendered document fits
// the configured budget. It accounts for the prompt, the directory tree, the
// document wrapper, and per-file framing, then verifies the exact rendered
// token count and shrinks if the estimate was optimistic.
func (a *App) fitToBudget(files []format.FileEntry, prompt string) []format.FileEntry {
	if a.cfg.Budget <= 0 || len(files) == 0 {
		return files
	}
	tz, err := tokenize.New(a.cfg.TokenizeModel)
	if err != nil {
		a.log.Warn("Unable to initialize tokenizer for budget accounting: %v", err)
		kept, _ := tokenize.FitToBudget(files, a.cfg.Budget)
		return kept
	}
	style := format.ParseStyle(a.cfg.EffectiveStyle())

	overhead := a.overheadTokens(tz, style, prompt, files)
	remaining := a.cfg.Budget - overhead
	if remaining <= 0 {
		a.log.Warn("Token budget %d is consumed by the prompt, tree, and framing before any file.", a.cfg.Budget)
		return nil
	}

	fit := func(budget int) []format.FileEntry {
		costed := make([]format.FileEntry, len(files))
		copy(costed, files)
		for i := range costed {
			extra := fileFramingTokens(style, costed[i].Path, costed[i].Tokens)
			costed[i].Tokens += extra
			costed[i].TokensFull += extra
			if costed[i].TokensSig > 0 {
				costed[i].TokensSig += extra
			}
		}
		var kept []format.FileEntry
		if a.cfg.FallbackMode == "signatures" {
			kept, _ = tokenize.FitToBudgetWithFallbackLazy(costed, budget, a.cfg.FallbackThreshold, a.signatureProvider())
		} else {
			kept, _ = tokenize.FitToBudget(costed, budget)
		}
		for i := range kept {
			extra := fileFramingTokens(style, kept[i].Path, kept[i].Tokens)
			kept[i].Tokens -= extra
			kept[i].TokensFull -= extra
			if kept[i].TokensSig > 0 {
				kept[i].TokensSig -= extra
			}
		}
		return kept
	}

	kept := fit(remaining)
	// Verify against the actual document and shrink on overshoot. Framing
	// estimates cannot know every renderer detail (JSON base64, escaping).
	for attempt := 0; attempt < 3 && len(kept) > 0; attempt++ {
		var buf bytes.Buffer
		if _, err := a.renderDocumentTo(kept, prompt, &buf, nil); err != nil {
			break
		}
		n, err := tz.Count(buf.Bytes())
		if err != nil {
			break
		}
		if n <= a.cfg.Budget {
			return kept
		}
		over := n - a.cfg.Budget
		remaining -= over + over/4 + 1
		if remaining <= 0 {
			return nil
		}
		kept = fit(remaining)
	}
	return kept
}

// overheadTokens counts the parts of the document that are not file bodies:
// the prompt, the directory tree, and the wrapper.
func (a *App) overheadTokens(tz *tokenize.Tokenizer, style format.Style, prompt string, files []format.FileEntry) int {
	total := 16 // document wrapper and separators
	if prompt != "" {
		if n, err := tz.Count([]byte(prompt)); err == nil {
			total += n
		}
	}
	paths := make([]string, len(files))
	for i, f := range files {
		paths[i] = f.Path
	}
	if tree := format.BuildTree(paths); tree != "" {
		if n, err := tz.Count([]byte(tree)); err == nil {
			total += n
		}
	}
	if len(a.archGraph) > 0 {
		if n, err := tz.Count([]byte(strings.Join(a.archGraph, "\n"))); err == nil {
			total += n + 8 // section framing
		}
	}
	if a.patch != "" {
		if n, err := tz.Count([]byte(a.patch)); err == nil {
			total += n + 10 // diff section framing
		}
	}
	return total
}

// fileFramingTokens estimates the header, fence, or tag overhead added around
// one file by the renderer. tokenCount is the file's own token count, used to
// account for JSON base64 expansion.
func fileFramingTokens(style format.Style, path string, tokenCount int) int {
	pathTokens := (len(path) + 3) / 4
	switch style {
	case format.StyleXML:
		return 16 + pathTokens*2
	case format.StyleMarkdown:
		return 12 + pathTokens*2
	case format.StyleJSON:
		// Content is base64-encoded, which is roughly 15% more tokens.
		return 20 + pathTokens*2 + tokenCount/5
	default:
		return 6 + pathTokens
	}
}

// EstimatedOverhead exposes the non-file token cost for selection callers so
// their optimizer can reserve room for the document around the files.
func (a *App) EstimatedOverhead(prompt string, files []format.FileEntry) int {
	a.prepareArchitecture(files)
	tz, err := tokenize.New(a.cfg.TokenizeModel)
	if err != nil {
		return 0
	}
	return a.overheadTokens(tz, format.ParseStyle(a.cfg.EffectiveStyle()), prompt, files)
}

// EstimatedDocumentOverhead returns the non-file token cost of rendering the
// given files in cfg's style, for callers that run their own selection pass.
func EstimatedDocumentOverhead(cfg *config.Config, prompt string, files []format.FileEntry) int {
	a, err := NewBuffered(bufferedConfig("", cfg))
	if err != nil {
		return 0
	}
	return a.EstimatedOverhead(prompt, files)
}

// signatureProvider lazily compresses a file and counts its signature tokens.
// The shared picker processor (and its content cache) is built on first use,
// so a dump that never falls back never loads tree-sitter.
func (a *App) signatureProvider() tokenize.SignatureProvider {
	var processor *scan.Processor
	return func(f format.FileEntry) ([]byte, int, bool) {
		if processor == nil {
			p, err := a.sharedProcessor(collectPicker)
			if err != nil {
				return nil, 0, false
			}
			processor = p
		}
		entry, err := processor.Process(f.Path, f.Content, scan.ModeSignatures)
		if err != nil || !entry.IsCompressed || len(entry.Content) == 0 {
			return nil, 0, false
		}
		return entry.Content, entry.Tokens, true
	}
}
