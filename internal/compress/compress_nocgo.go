//go:build !cgo

// Package compress provides the portable fallback used by static builds.
// Tree-sitter's native grammars are unavailable without CGO, so callers keep
// the original source rather than producing an unsafe partial signature.
package compress

import (
	"github.com/bethropolis/sift/internal/lang"
)

// Compressor is a no-CGO compressor. It intentionally does not claim that a
// raw source file was compressed; scan then retains the full representation.
type Compressor struct{}

func New() *Compressor { return &Compressor{} }

func (c *Compressor) LanguageForPath(path string) (lang.ID, bool) {
	l, ok := lang.ForPath(path)
	if !ok {
		return "", false
	}
	return l.ID(), ok
}

func (c *Compressor) Compress(_ []byte, _ lang.ID) (string, bool) {
	return "", false
}
