//go:build cgo

package langother

import (
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/cpp"
	"github.com/smacker/go-tree-sitter/csharp"
	"github.com/smacker/go-tree-sitter/java"
	"github.com/smacker/go-tree-sitter/kotlin"
	"github.com/smacker/go-tree-sitter/php"
	"github.com/smacker/go-tree-sitter/ruby"
	"github.com/smacker/go-tree-sitter/swift"
	zig "github.com/tree-sitter-grammars/tree-sitter-zig/bindings/go"

	"github.com/bethropolis/sift/internal/lang/signature"
	"github.com/bethropolis/sift/internal/lang/types"
)

func init() {
	signature.Register(types.PHP, &signature.SignatureSpec{
		Declarations: types.DeclMap(
			"namespace_definition", "namespace_use_declaration", "const_declaration",
			"function_definition", "method_declaration", "class_declaration",
			"interface_declaration", "trait_declaration", "enum_declaration",
		),
		Grammar: php.GetLanguage(),
		HeaderOrConst: func(t string) bool {
			return t == "namespace_definition" || t == "namespace_use_declaration" || t == "const_declaration"
		},
		TypeDefinition: func(t string) bool {
			return t == "class_declaration" || t == "interface_declaration" || t == "trait_declaration" || t == "enum_declaration"
		},
		FunctionOrMethod: func(t string) bool { return t == "function_definition" || t == "method_declaration" },
	})

	signature.Register(types.Java, &signature.SignatureSpec{
		Declarations: types.DeclMap(
			"package_declaration", "import_declaration", "class_declaration",
			"interface_declaration", "enum_declaration", "record_declaration", "method_declaration",
		),
		Grammar:       java.GetLanguage(),
		HeaderOrConst: func(t string) bool { return t == "package_declaration" || t == "import_declaration" },
		TypeDefinition: func(t string) bool {
			return t == "class_declaration" || t == "interface_declaration" || t == "enum_declaration" || t == "record_declaration"
		},
		FunctionOrMethod: func(t string) bool { return t == "method_declaration" },
	})

	signature.Register(types.Kotlin, &signature.SignatureSpec{
		Declarations: types.DeclMap(
			"package_header", "import_header", "class_declaration", "object_declaration",
			"function_declaration", "property_declaration", "type_alias",
		),
		Grammar:          kotlin.GetLanguage(),
		HeaderOrConst:    func(t string) bool { return t == "package_header" || t == "import_header" },
		TypeDefinition:   func(t string) bool { return t == "class_declaration" || t == "object_declaration" || t == "type_alias" },
		FunctionOrMethod: func(t string) bool { return t == "function_declaration" },
	})

	signature.Register(types.CSharp, &signature.SignatureSpec{
		Declarations: types.DeclMap(
			"using_directive", "namespace_declaration", "class_declaration", "interface_declaration",
			"struct_declaration", "enum_declaration", "method_declaration", "property_declaration",
		),
		Grammar:       csharp.GetLanguage(),
		HeaderOrConst: func(t string) bool { return t == "using_directive" },
		TypeDefinition: func(t string) bool {
			return t == "class_declaration" || t == "interface_declaration" || t == "struct_declaration" || t == "enum_declaration"
		},
		FunctionOrMethod: func(t string) bool { return t == "method_declaration" || t == "property_declaration" },
	})

	signature.Register(types.Cpp, &signature.SignatureSpec{
		Declarations: types.DeclMap(
			"preproc_include", "using_declaration", "function_definition",
			"class_specifier", "struct_specifier", "enum_specifier",
		),
		Grammar:          cpp.GetLanguage(),
		HeaderOrConst:    func(t string) bool { return t == "preproc_include" || t == "using_declaration" },
		TypeDefinition:   func(t string) bool { return t == "class_specifier" || t == "struct_specifier" || t == "enum_specifier" },
		FunctionOrMethod: func(t string) bool { return t == "function_definition" },
	})

	signature.Register(types.Ruby, &signature.SignatureSpec{
		Declarations:  types.DeclMap("class", "module", "method", "singleton_method"),
		Grammar:       ruby.GetLanguage(),
		HeaderOrConst: func(string) bool { return false },
		// Ruby classes are not grouped type definitions; methods are emitted
		// individually (matches the historical behavior).
		TypeDefinition:   func(string) bool { return false },
		FunctionOrMethod: func(t string) bool { return t == "method" || t == "singleton_method" },
	})

	signature.Register(types.Swift, &signature.SignatureSpec{
		Declarations: types.DeclMap(
			"import_declaration", "class_declaration", "struct_declaration",
			"enum_declaration", "protocol_declaration", "function_declaration", "init_declaration",
		),
		Grammar:       swift.GetLanguage(),
		HeaderOrConst: func(t string) bool { return t == "import_declaration" },
		TypeDefinition: func(t string) bool {
			return t == "class_declaration" || t == "struct_declaration" || t == "enum_declaration" || t == "protocol_declaration"
		},
		FunctionOrMethod: func(t string) bool { return t == "function_declaration" || t == "init_declaration" },
	})

	signature.Register(types.Zig, &signature.SignatureSpec{
		Declarations: types.DeclMap(
			"function_declaration", "struct_declaration", "enum_declaration",
			"union_declaration", "error_set_declaration", "test_declaration",
			"using_namespace_declaration",
		),
		Grammar:       sitter.NewLanguage(zig.Language()),
		HeaderOrConst: func(t string) bool { return t == "using_namespace_declaration" },
		TypeDefinition: func(t string) bool {
			return t == "struct_declaration" || t == "enum_declaration" || t == "union_declaration" || t == "error_set_declaration"
		},
		FunctionOrMethod: func(t string) bool { return t == "function_declaration" || t == "test_declaration" },
	})
}
