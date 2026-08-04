//go:build cgo

// Package compress extracts signature-only summaries of source files using
// tree-sitter grammars registered in internal/lang. The result keeps doc
// comments and declaration signatures while dropping bodies, so an LLM can
// reason about structure without paying for implementation tokens. The cgo
// build provides the real engine; the no-cgo build keeps the original source.
package compress

import (
	"strings"
	"sync"

	sitter "github.com/smacker/go-tree-sitter"

	"github.com/bethropolis/sift/internal/lang"
)

// Compressor produces signature summaries for registered languages.
//
// parsers holds one reusable parser per language. Tree-sitter parser
// creation/destruction crosses cgo, which is expensive per file; pooling them
// avoids that churn across worker goroutines.
type Compressor struct {
	parsers map[lang.ID]*sync.Pool
}

// New returns a Compressor with a parser pool for every registered language.
func New() *Compressor {
	parsers := make(map[lang.ID]*sync.Pool)
	for _, id := range lang.SignatureIDs() {
		spec, ok := lang.LookupSignature(id)
		if !ok || spec.Grammar == nil {
			continue
		}
		grammar := spec.Grammar
		parsers[id] = &sync.Pool{
			New: func() any {
				p := sitter.NewParser()
				p.SetLanguage(grammar)
				return p
			},
		}
	}
	return &Compressor{parsers: parsers}
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
	if _, has := lang.LookupSignature(id); !has {
		return "", false
	}
	return id, true
}

// Compress returns a signature-only summary of src for id. The boolean result
// reports whether any declaration was emitted; false means the source is
// returned unchanged (nothing to compress or a parse failure).
func (c *Compressor) Compress(src []byte, id lang.ID) (string, bool) {
	spec, ok := lang.LookupSignature(id)
	if !ok {
		return string(src), false
	}
	pool, ok := c.parsers[id]
	if !ok {
		return string(src), false
	}

	parser := pool.Get().(*sitter.Parser)
	defer pool.Put(parser)

	tree := parser.Parse(nil, src)
	if tree == nil {
		return string(src), false
	}
	root := tree.RootNode()

	var b strings.Builder
	c.walkDeclarations(root, src, spec, &b)
	if b.Len() == 0 {
		return string(src), false
	}
	return strings.TrimRight(b.String(), "\n"), true
}
