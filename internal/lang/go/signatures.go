//go:build cgo

package langgo

import (
	"github.com/bethropolis/sift/internal/lang/signature"
	"github.com/bethropolis/sift/internal/lang/types"
	"github.com/smacker/go-tree-sitter/golang"
)

func init() {
	signature.Register(types.Go, &signature.SignatureSpec{
		Declarations: types.DeclMap(
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
