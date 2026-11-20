// Package tokenize counts tokens offline and fits documents to a budget.
package tokenize

import (
	"fmt"
	"strings"
	"unsafe"

	"github.com/tiktoken-go/tokenizer"

	"github.com/bethropolis/sift/internal/format"
)

// Tokenizer counts tokens using an offline BPE encoder.
type Tokenizer struct {
	codec tokenizer.Codec
}

// New returns a Tokenizer for the named encoding (e.g. "cl100k_base",
// "o200k_base"). An empty name selects cl100k_base.
func New(encoding string) (*Tokenizer, error) {
	if encoding == "" {
		encoding = "cl100k_base"
	}

	enc, err := parseEncoding(encoding)
	if err != nil {
		return nil, err
	}
	codec, err := tokenizer.Get(enc)
	if err != nil {
		return nil, fmt.Errorf("tokenize: init %s: %w", encoding, err)
	}
	return &Tokenizer{codec: codec}, nil
}

func parseEncoding(encoding string) (tokenizer.Encoding, error) {
	switch strings.ToLower(strings.TrimSpace(encoding)) {
	case "gpt2", "r50k", "r50k_base":
		return tokenizer.R50kBase, nil
	case "p50k_base":
		return tokenizer.P50kBase, nil
	case "p50k_edit":
		return tokenizer.P50kEdit, nil
	// OpenAI modern models and Claude all use the cl100k BPE in practice;
	// non-OpenAI names are an offline approximation, not an API contract.
	case "cl100k", "cl100k_base", "gpt-4", "gpt-4-turbo", "gpt-3.5", "gpt-3.5-turbo",
		"claude", "anthropic":
		return tokenizer.Cl100kBase, nil
	case "o200k", "o200k_base", "gpt-4o", "gpt-4o-mini", "gpt-5", "gemini":
		return tokenizer.O200kBase, nil
	default:
		return "", fmt.Errorf("tokenize: unsupported encoding %q", encoding)
	}
}

// ValidateModel reports whether model/encoding is accepted by the tokenizer,
// so config checks can fail early instead of at first render.
func ValidateModel(model string) error {
	if strings.TrimSpace(model) == "" {
		return nil
	}
	_, err := parseEncoding(model)
	return err
}

// Count returns the number of tokens in text.
func (t *Tokenizer) Count(text []byte) (int, error) {
	if len(text) == 0 {
		return 0, nil
	}
	// Zero-allocation string conversion: tiktoken only reads the input, so the
	// caller's buffer can be reused instead of copying it to the heap.
	str := unsafe.String(unsafe.SliceData(text), len(text))
	n, err := t.codec.Count(str)
	if err != nil {
		return 0, fmt.Errorf("tokenize: count: %w", err)
	}
	return n, nil
}

// FitToBudget greedily keeps files until the next one would exceed budget.
// Files are expected to be pre-sorted by priority; it returns the kept files
// and the total tokens used.
func FitToBudget(files []format.FileEntry, budget int) ([]format.FileEntry, int) {
	if budget <= 0 {
		total := 0
		for _, f := range files {
			total += f.Tokens
		}
		return files, total
	}

	kept := make([]format.FileEntry, 0, len(files))
	used := 0
	for _, f := range files {
		if used+f.Tokens > budget {
			break
		}
		kept = append(kept, f)
		used += f.Tokens
	}
	return kept, used
}

// SignatureProvider lazily produces a signature rendering and its token count
// for a file. ok is false when the file cannot be compressed (unsupported
// language, parse failure, or a no-cgo build).
type SignatureProvider func(format.FileEntry) (content []byte, tokens int, ok bool)

// FitToBudgetWithFallback packs files against budget using signatures already
// present on the entries. See FitToBudgetWithFallbackLazy.
func FitToBudgetWithFallback(files []format.FileEntry, budget int, threshold float64) ([]format.FileEntry, int) {
	return FitToBudgetWithFallbackLazy(files, budget, threshold, nil)
}

// FitToBudgetWithFallbackLazy packs files against budget. Files are packed in
// full fidelity until threshold * budget is reached (or until a file overflows
// in full mode). When fallback activates, provide is called on demand for the
// signature rendering and token count, so tree-sitter and BPE work only runs
// for files that actually fall back.
//
// Returns the kept files and the total tokens used.
func FitToBudgetWithFallbackLazy(files []format.FileEntry, budget int, threshold float64, provide SignatureProvider) ([]format.FileEntry, int) {
	if budget <= 0 {
		total := 0
		for _, f := range files {
			total += f.Tokens
		}
		return files, total
	}

	if threshold <= 0 || threshold > 1.0 {
		if threshold > 1.0 && threshold <= 100.0 {
			threshold = threshold / 100.0
		} else {
			threshold = 1.0
		}
	}

	thresholdBudget := int(float64(budget) * threshold)
	kept := make([]format.FileEntry, 0, len(files))
	used := 0
	fallbackTriggered := false

	for _, f := range files {
		if used >= budget {
			break
		}

		// If already compressed (e.g. forced via [modes] signatures):
		if f.IsCompressed {
			if used+f.Tokens <= budget {
				kept = append(kept, f)
				used += f.Tokens
			}
			continue
		}

		if !fallbackTriggered {
			fullTokens := f.Tokens
			if fullTokens <= 0 && f.TokensFull > 0 {
				fullTokens = f.TokensFull
			}
			if used+fullTokens <= thresholdBudget && used+fullTokens <= budget {
				f.Tokens = fullTokens
				kept = append(kept, f)
				used += fullTokens
				continue
			}
			if used < thresholdBudget {
				// Still below the threshold: this single file is the outlier.
				// Downgrade just this file (or skip it) instead of flipping
				// the global switch and compressing every smaller file behind
				// it.
				sigContent, sigTokens, ok := signatureVariant(f, fullTokens, provide)
				if !ok || sigTokens >= fullTokens {
					continue
				}
				if used+sigTokens <= budget {
					if len(sigContent) > 0 {
						f.Content = sigContent
						f.IsCompressed = true
					}
					f.Tokens = sigTokens
					kept = append(kept, f)
					used += sigTokens
				}
				continue
			}
			fallbackTriggered = true
		}

		// Fallback active: try to pack as signature.
		sigContent, sigTokens, ok := signatureVariant(f, f.Tokens, provide)
		if !ok {
			sigTokens = f.Tokens
			sigContent = nil
		}
		if used+sigTokens <= budget {
			if len(sigContent) > 0 {
				f.Content = sigContent
				f.IsCompressed = true
			}
			f.Tokens = sigTokens
			kept = append(kept, f)
			used += sigTokens
		}
	}

	return kept, used
}

// signatureVariant returns the signature content and token count for f,
// consulting provide only when the entry does not already carry them.
func signatureVariant(f format.FileEntry, fullTokens int, provide SignatureProvider) ([]byte, int, bool) {
	if f.TokensSig > 0 {
		return f.SigContent, f.TokensSig, true
	}
	if provide == nil {
		return nil, fullTokens, false
	}
	content, tokens, ok := provide(f)
	if !ok || tokens <= 0 {
		return nil, fullTokens, false
	}
	return content, tokens, true
}
