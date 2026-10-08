//go:build cgo

// Package compress extracts signature-only summaries of source files using
// tree-sitter grammars registered in internal/lang. The result keeps doc
// comments and declaration signatures while dropping bodies, so an LLM can
// reason about structure without paying for implementation tokens. The cgo
// build provides the real engine; the no-cgo build keeps the original source.
package compress

import (
	"github.com/bethropolis/sift/internal/lang"
	"github.com/bethropolis/sift/internal/lang/signature"
)

// Compressor produces signature summaries for registered languages by
// delegating to the pooled tree-sitter engine in internal/lang/signature.
type Compressor struct {
	engine *signature.Engine
}

// every registered language.
func New() *Compressor {
	return &Compressor{engine: signature.New()}
}

// LanguageForPath returns the language for a path that has a registered
// signature spec, if any.
func (c *Compressor) LanguageForPath(path string) (lang.ID, bool) {
	l, ok := lang.ForPath(path)
	if !ok {
		return "", false
	}
	id := l.ID()
	if r, ok := l.(lang.SignatureResolver); ok {
		id = r.SignatureLanguage(path)
	}
	if _, has := signature.Lookup(id); !has {
		return "", false
	}
	return id, true
}

// Compress returns a signature-only summary of src for id. The boolean result
// reports whether any declaration was emitted; false means the source is
// returned unchanged (nothing to compress or a parse failure).
func (c *Compressor) Compress(src []byte, id lang.ID) (string, bool) {
	return c.engine.Signature(src, id)
}
