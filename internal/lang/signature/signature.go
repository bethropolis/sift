//go:build cgo

// Package signature owns the tree-sitter signature engine: per-language
// SignatureSpec registration and a pooled parser engine that reduces source to
// declaration signatures. It is cgo-only because tree-sitter's native grammars
// need CGO; the no-cgo build does not compile this package and callers fall
// back to keeping the full source.
package signature

import (
	"sync"

	"github.com/bethropolis/sift/internal/lang/types"
	sitter "github.com/smacker/go-tree-sitter"
)

// SignatureSpec defines how a language's AST is reduced to signatures.
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
	sigMu sync.RWMutex
	specs = map[types.ID]*SignatureSpec{}
)

// Register registers the tree-sitter signature spec for id, replacing any
// previous entry.
func Register(id types.ID, spec *SignatureSpec) {
	sigMu.Lock()
	defer sigMu.Unlock()
	specs[id] = spec
}

// Lookup returns the signature spec registered for id.
func Lookup(id types.ID) (*SignatureSpec, bool) {
	sigMu.RLock()
	defer sigMu.RUnlock()
	s, ok := specs[id]
	return s, ok
}

// FunctionInVariableDecl reports whether a variable/lexical declaration
// initialises an arrow function or function expression. It is the shared
// VariableWithFunction predicate for the JS/TS grammars.
func FunctionInVariableDecl(n *sitter.Node, src []byte) bool {
	if n == nil {
		return false
	}
	for i := 0; i < int(n.ChildCount()); i++ {
		child := n.Child(i)
		if child.Type() == "variable_declarator" {
			if val := child.ChildByFieldName("value"); val != nil &&
				(val.Type() == "arrow_function" || val.Type() == "function_expression") {
				return true
			}
		}
	}
	return false
}

// IDs returns the ids that registered a signature spec, so an engine can build
// a parser pool for each supported grammar.
func IDs() []types.ID {
	sigMu.RLock()
	defer sigMu.RUnlock()
	out := make([]types.ID, 0, len(specs))
	for id := range specs {
		out = append(out, id)
	}
	return out
}
