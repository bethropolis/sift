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
	switch strings.ToLower(encoding) {
	case "gpt2", "r50k_base":
		return tokenizer.R50kBase, nil
	case "p50k_base":
		return tokenizer.P50kBase, nil
	case "p50k_edit":
		return tokenizer.P50kEdit, nil
	case "cl100k_base":
		return tokenizer.Cl100kBase, nil
	case "o200k_base":
		return tokenizer.O200kBase, nil
	default:
		return "", fmt.Errorf("tokenize: unsupported encoding %q", encoding)
	}
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

// FitToBudgetWithFallback packs files against budget.
// Files are packed in full fidelity until threshold * budget is reached (or until a file overflows in full mode).
// When fallback is active, subsequent files are packed as signatures using their SigContent and TokensSig.
// Returns the kept files and the total tokens used.
func FitToBudgetWithFallback(files []format.FileEntry, budget int, threshold float64) ([]format.FileEntry, int) {
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
				sigTokens := f.TokensSig
				if sigTokens <= 0 {
					sigTokens = fullTokens
				}
				if sigTokens < fullTokens && used+sigTokens <= budget {
					if len(f.SigContent) > 0 {
						f.Content = f.SigContent
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

		// Fallback active: try to pack as signature
		sigTokens := f.TokensSig
		if sigTokens <= 0 {
			sigTokens = f.Tokens
		}
		if used+sigTokens <= budget {
			if len(f.SigContent) > 0 {
				f.Content = f.SigContent
				f.Tokens = sigTokens
				f.IsCompressed = true
			} else {
				f.Tokens = sigTokens
			}
			kept = append(kept, f)
			used += sigTokens
		}
	}

	return kept, used
}
