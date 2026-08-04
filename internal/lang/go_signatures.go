//go:build cgo

package lang

import "github.com/smacker/go-tree-sitter/golang"

func init() {
	RegisterSignature(Go, &SignatureSpec{
		Declarations: DeclMap(
			"package_clause", "import_declaration", "const_declaration", "var_declaration",
			"function_declaration", "method_declaration", "type_declaration",
		),
		Grammar: golang.GetLanguage(),
		HeaderOrConst: func(t string) bool {
			return t == "package_clause" || t == "import_declaration" || t == "const_declaration" || t == "var_declaration"
		},
		TypeDefinition:   func(t string) bool { return t == "type_declaration" },
		FunctionOrMethod: func(t string) bool { return t == "function_declaration" || t == "method_declaration" },
	})
}
