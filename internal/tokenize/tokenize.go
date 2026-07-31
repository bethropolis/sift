// Package tokenize counts tokens offline and fits documents to a budget.
package tokenize

import (
	"fmt"
	"strings"

	"github.com/tiktoken-go/tokenizer"

	"github.com/bethropolis/dir-dumper/internal/format"
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
	n, err := t.codec.Count(string(text))
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
