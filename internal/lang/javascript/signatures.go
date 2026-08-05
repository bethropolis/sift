//go:build cgo

package langjs

import (
	"github.com/bethropolis/sift/internal/lang/signature"
	"github.com/bethropolis/sift/internal/lang/types"
	"github.com/smacker/go-tree-sitter/javascript"
)

func init() {
	signature.Register(types.JavaScript, &signature.SignatureSpec{
		Declarations: types.DeclMap(
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
		VariableWithFunction: signature.FunctionInVariableDecl,
	})
}
