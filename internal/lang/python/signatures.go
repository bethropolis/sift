//go:build cgo

package langpy

import (
	"github.com/bethropolis/sift/internal/lang/signature"
	"github.com/bethropolis/sift/internal/lang/types"
	"github.com/smacker/go-tree-sitter/python"
)

func init() {
	signature.Register(types.Python, &signature.SignatureSpec{
		Declarations: types.DeclMap("import_statement", "import_from_statement", "function_definition", "class_definition"),
		Grammar:      python.GetLanguage(),
		PythonLike:   true,
		HeaderOrConst: func(t string) bool {
			return t == "import_statement" || t == "import_from_statement"
		},
		TypeDefinition:   func(t string) bool { return t == "class_definition" },
		FunctionOrMethod: func(t string) bool { return t == "function_definition" },
	})
}
