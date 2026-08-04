//go:build cgo

package lang

import (
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/javascript"
)

func init() {
	RegisterSignature(JavaScript, &SignatureSpec{
		Declarations: DeclMap(
			"import_statement", "variable_declaration", "lexical_declaration",
			"function_declaration", "generator_function_declaration",
			"method_definition", "arrow_function", "class_declaration",
		),
		Grammar: javascript.GetLanguage(),
		HeaderOrConst: func(t string) bool {
			// const/let/var are emitted verbatim UNLESS they initialise a
			// function, which VariableWithFunction intercepts first.
			return t == "import_statement" || t == "variable_declaration" || t == "lexical_declaration"
		},
		TypeDefinition: func(t string) bool { return t == "class_declaration" },
		FunctionOrMethod: func(t string) bool {
			return t == "function_declaration" || t == "generator_function_declaration" ||
				t == "method_definition" || t == "arrow_function"
		},
		VariableWithFunction: functionInVariableDecl,
	})
}

// functionInVariableDecl reports whether a variable/lexical declaration
// initialises an arrow function or function expression.
func functionInVariableDecl(n *sitter.Node, src []byte) bool {
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
