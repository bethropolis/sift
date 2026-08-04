//go:build cgo

package lang

import "github.com/smacker/go-tree-sitter/python"

func init() {
	RegisterSignature(Python, &SignatureSpec{
		Declarations: DeclMap("import_statement", "import_from_statement", "function_definition", "class_definition"),
		Grammar:      python.GetLanguage(),
		PythonLike:   true,
		HeaderOrConst: func(t string) bool {
			return t == "import_statement" || t == "import_from_statement"
		},
		TypeDefinition:   func(t string) bool { return t == "class_definition" },
		FunctionOrMethod: func(t string) bool { return t == "function_definition" },
	})
}
