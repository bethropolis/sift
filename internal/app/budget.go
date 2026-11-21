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
// token count for JSON (where base64 expansion is hard to estimate) and
// shrinks if necessary. File bodies are never rendered or scanned here.
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

	// Keep the uncosted entries so every token count can be restored exactly
	// after fitting. The fitter works on inflated copies that include per-file
	// framing; recomputing framing from an inflated count is wrong (and JSON
	// is asymmetric because framing depends on the token count).
	original := make(map[string]format.FileEntry, len(files))
	for _, f := range files {
		original[f.Path] = f
	}

	fit := func(budget int) []format.FileEntry {
		costed := make([]format.FileEntry, len(files))
		copy(costed, files)
		for i := range costed {
			if costed[i].PinnedFull {
				// A [modes].full pin must stay full: no signature fallback.
				costed[i].SigContent = nil
				costed[i].TokensSig = 0
			}
			extra := fileFramingTokens(style, costed[i].Path, costed[i].Tokens)
			costed[i].Tokens += extra
			costed[i].TokensFull += extra
			if costed[i].TokensSig > 0 {
				costed[i].TokensSig += extra
			}
		}

		// The provider returns framed signature tokens; record its uncosted
		// count so the result can be restored precisely.
		sigTokens := make(map[string]int)
		var kept []format.FileEntry
		if a.cfg.FallbackMode == "signatures" {
			kept, _ = tokenize.FitToBudgetWithFallbackLazy(costed, budget, a.cfg.FallbackThreshold, a.signatureProvider(style, sigTokens))
		} else {
			kept, _ = tokenize.FitToBudget(costed, budget)
		}

		for i := range kept {
			orig, ok := original[kept[i].Path]
			if !ok {
				continue
			}
			kept[i].TokensFull = orig.TokensFull
			switch {
			case kept[i].IsCompressed && !orig.IsCompressed:
				if n, ok := sigTokens[kept[i].Path]; ok {
					kept[i].Tokens = n
					kept[i].TokensSig = n
				} else if orig.TokensSig > 0 {
					kept[i].Tokens = orig.TokensSig
					kept[i].TokensSig = orig.TokensSig
				} else {
					kept[i].Tokens = orig.Tokens
					kept[i].TokensSig = orig.TokensSig
				}
			default:
				kept[i].Tokens = orig.Tokens
				kept[i].TokensSig = orig.TokensSig
			}
		}
		return kept
	}

	kept := fit(remaining)
	// JSON base64 expansion makes arithmetic framing approximate. Verify once
	// against a render without secret scanning, then shrink only if needed.
	if style == format.StyleJSON {
		for attempt := 0; attempt < 3 && len(kept) > 0; attempt++ {
			n, err := a.countRenderedTokens(tz, kept, prompt)
			if err != nil || n <= a.cfg.Budget {
				break
			}
			over := n - a.cfg.Budget
			remaining -= over + over/4 + 1
			if remaining <= 0 {
				return nil
			}
			kept = fit(remaining)
		}
	}
	return kept
}

// countRenderedTokens renders without secret scanning and counts the exact
// document bytes. It is used only for JSON, where base64 expansion is difficult
// to estimate; file bodies are not scanned or re-tokenized in any other style.
func (a *App) countRenderedTokens(tz *tokenize.Tokenizer, files []format.FileEntry, prompt string) (int, error) {
	var buf bytes.Buffer
	if _, err := a.renderDocumentNoScan(files, prompt, &buf, nil); err != nil {
		return 0, err
	}
	return tz.Count(buf.Bytes())
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

// signatureProvider lazily compresses a file and returns its framed token
// count. The shared picker processor (and its content cache) is built on first
// use, so a dump that never falls back never loads tree-sitter.
func (a *App) signatureProvider(style format.Style, rawTokens map[string]int) tokenize.SignatureProvider {
	var processor *scan.Processor
	return func(f format.FileEntry) ([]byte, int, bool) {
		if f.PinnedFull {
			return nil, 0, false
		}
		if processor == nil {
			p, err := a.sharedProcessor(collectPicker)
			if err != nil {
				return nil, 0, false
			}
			processor = p
		}
		entry, err := processor.Process(f.Path, f.Content, scan.ModeSignatures)
		if err != nil || !entry.IsCompressed || len(entry.Content) == 0 || entry.Tokens <= 0 {
			return nil, 0, false
		}
		rawTokens[f.Path] = entry.Tokens
		return entry.Content, entry.Tokens + fileFramingTokens(style, f.Path, entry.Tokens), true
	}
}
