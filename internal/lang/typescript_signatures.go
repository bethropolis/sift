//go:build cgo

package lang

import (
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/typescript/tsx"
	"github.com/smacker/go-tree-sitter/typescript/typescript"
)

func init() {
	tsSpec := &SignatureSpec{
		Declarations: DeclMap(
			"import_statement", "variable_declaration", "lexical_declaration",
			"function_declaration", "generator_function_declaration", "method_definition",
			"method_signature", "property_signature", "arrow_function",
			"class_declaration", "interface_declaration", "type_alias_declaration",
			"enum_declaration", "abstract_method_signature",
		),
		HeaderOrConst: func(t string) bool {
			return t == "import_statement" || t == "variable_declaration" || t == "lexical_declaration"
		},
		TypeDefinition: func(t string) bool {
			return t == "class_declaration" || t == "interface_declaration" || t == "type_alias_declaration" || t == "enum_declaration"
		},
		FunctionOrMethod: func(t string) bool {
			return t == "function_declaration" || t == "generator_function_declaration" ||
				t == "method_definition" || t == "arrow_function" ||
				t == "method_signature" || t == "abstract_method_signature"
		},
		VariableWithFunction: functionInVariableDecl,
	}
	RegisterSignature(TypeScript, withGrammar(tsSpec, typescript.GetLanguage()))
	RegisterSignature(TSX, withGrammar(tsSpec, tsx.GetLanguage()))
}

func withGrammar(spec *SignatureSpec, grammar *sitter.Language) *SignatureSpec {
	copy := *spec
	copy.Grammar = grammar
	return &copy
}
