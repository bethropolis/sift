//go:build cgo

package lang

import (
	"sync"

	sitter "github.com/smacker/go-tree-sitter"
)

// SignatureSpec defines how a language's AST is reduced to signatures. It is
// used only in cgo builds, where tree-sitter's native grammars are available;
// the non-cgo build does not compile this file and compress falls back to
// keeping the original source.
type SignatureSpec struct {
	// Declarations are the top-level AST node types to extract.
	Declarations map[string]bool
	// Grammar is the tree-sitter language grammar.
	Grammar *sitter.Language
	// PythonLike marks brace-less languages that end signatures with a colon
	// (" ...") instead of a brace placeholder.
	PythonLike bool
	// HeaderOrConst returns true for nodes emitted verbatim (imports, package,
	// const/var declarations that do not wrap a function).
	HeaderOrConst func(nodeType string) bool
	// TypeDefinition returns true for struct/interface/class/enum nodes whose
	// interior should be preserved (with class method bodies compressed).
	TypeDefinition func(nodeType string) bool
	// FunctionOrMethod returns true for nodes whose body becomes a placeholder.
	FunctionOrMethod func(nodeType string) bool
	// VariableWithFunction returns true when a variable declaration initializes
	// a function (arrow_function / function_expression), used by JS/TS.
	VariableWithFunction func(node *sitter.Node, src []byte) bool
}

var (
	sigMu    sync.RWMutex
	sigSpecs = map[ID]*SignatureSpec{}
)

// RegisterSignature registers the tree-sitter signature spec for id.
func RegisterSignature(id ID, spec *SignatureSpec) {
	sigMu.Lock()
	defer sigMu.Unlock()
	sigSpecs[id] = spec
}

// LookupSignature returns the signature spec registered for id.
func LookupSignature(id ID) (*SignatureSpec, bool) {
	sigMu.RLock()
	defer sigMu.RUnlock()
	s, ok := sigSpecs[id]
	return s, ok
}

// SignatureIDs returns the ids that registered a signature spec, so cgo callers
// can build a parser pool for each supported grammar.
func SignatureIDs() []ID {
	sigMu.RLock()
	defer sigMu.RUnlock()
	ids := make([]ID, 0, len(sigSpecs))
	for id := range sigSpecs {
		ids = append(ids, id)
	}
	return ids
}
