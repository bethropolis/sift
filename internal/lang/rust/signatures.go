//go:build cgo

package langrs

import (
	"github.com/bethropolis/sift/internal/lang/signature"
	"github.com/bethropolis/sift/internal/lang/types"
	"github.com/smacker/go-tree-sitter/rust"
)

func init() {
	signature.Register(types.Rust, &signature.SignatureSpec{
		Declarations: types.DeclMap(
			"use_declaration", "extern_crate_declaration", "const_item", "static_item",
			"function_item", "struct_item", "enum_item", "trait_item", "type_item",
		),
		Grammar: rust.GetLanguage(),
		HeaderOrConst: func(t string) bool {
			return t == "use_declaration" || t == "extern_crate_declaration" || t == "const_item" || t == "static_item"
		},
		TypeDefinition: func(t string) bool {
			return t == "struct_item" || t == "enum_item" || t == "trait_item" || t == "type_item"
		},
		FunctionOrMethod: func(t string) bool { return t == "function_item" },
	})
}
