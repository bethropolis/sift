//go:build cgo

// Package compress extracts signature-only summaries of source files using
// tree-sitter grammars. The result keeps doc comments and declaration
// signatures (function/method/type/class headers) while dropping bodies, so an
// LLM can reason about structure without paying for implementation tokens.
package compress

import (
	"path/filepath"
	"strings"
	"sync"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/cpp"
	"github.com/smacker/go-tree-sitter/csharp"
	"github.com/smacker/go-tree-sitter/golang"
	"github.com/smacker/go-tree-sitter/java"
	"github.com/smacker/go-tree-sitter/javascript"
	"github.com/smacker/go-tree-sitter/kotlin"
	"github.com/smacker/go-tree-sitter/php"
	"github.com/smacker/go-tree-sitter/python"
	"github.com/smacker/go-tree-sitter/ruby"
	"github.com/smacker/go-tree-sitter/rust"
	"github.com/smacker/go-tree-sitter/swift"
	"github.com/smacker/go-tree-sitter/typescript/tsx"
	"github.com/smacker/go-tree-sitter/typescript/typescript"
)

// Language identifies a supported grammar.
type Language int

const (
	// Go is the Go grammar.
	Go Language = iota
	// Rust is the Rust grammar.
	Rust
	// JavaScript is the JavaScript grammar.
	JavaScript
	// TypeScript is the TypeScript grammar.
	TypeScript
	// TSX is the TSX grammar (TypeScript with JSX).
	TSX
	// Python is the Python grammar.
	Python
	// PHP is the PHP grammar.
	PHP
	Java
	Kotlin
	CSharp
	Cpp
	Ruby
	Swift
)

func (l Language) String() string {
	switch l {
	case Go:
		return "go"
	case Rust:
		return "rust"
	case JavaScript:
		return "javascript"
	case TypeScript:
		return "typescript"
	case TSX:
		return "tsx"
	case Python:
		return "python"
	case PHP:
		return "php"
	case Java:
		return "java"
	case Kotlin:
		return "kotlin"
	case CSharp:
		return "csharp"
	case Cpp:
		return "cpp"
	case Ruby:
		return "ruby"
	case Swift:
		return "swift"
	}
	return "unknown"
}

var extToLang = map[string]Language{
	".go":    Go,
	".rs":    Rust,
	".js":    JavaScript,
	".jsx":   JavaScript,
	".mjs":   JavaScript,
	".cjs":   JavaScript,
	".ts":    TypeScript,
	".tsx":   TSX,
	".py":    Python,
	".pyi":   Python,
	".php":   PHP,
	".java":  Java,
	".kt":    Kotlin,
	".kts":   Kotlin,
	".cs":    CSharp,
	".c":     Cpp,
	".h":     Cpp,
	".cc":    Cpp,
	".cpp":   Cpp,
	".cxx":   Cpp,
	".rb":    Ruby,
	".swift": Swift,
}

var declTypes = map[Language]map[string]bool{
	Go: stringSet(
		"package_clause",
		"import_declaration",
		"const_declaration",
		"var_declaration",
		"function_declaration",
		"method_declaration",
		"type_declaration",
	),
	Rust: stringSet(
		"use_declaration",
		"extern_crate_declaration",
		"const_item",
		"static_item",
		"function_item",
		"struct_item",
		"enum_item",
		"trait_item",
		"type_item",
	),
	JavaScript: stringSet(
		"import_statement",
		"variable_declaration",
		"function_declaration",
		"generator_function_declaration",
		"method_definition",
		"arrow_function",
		"class_declaration",
	),
	TypeScript: stringSet(
		"import_statement",
		"variable_declaration",
		"function_declaration",
		"generator_function_declaration",
		"method_definition",
		"method_signature",
		"property_signature",
		"arrow_function",
		"class_declaration",
		"interface_declaration",
		"type_alias_declaration",
		"enum_declaration",
		"abstract_method_signature",
	),
	TSX: stringSet(
		"import_statement",
		"variable_declaration",
		"function_declaration",
		"generator_function_declaration",
		"method_definition",
		"method_signature",
		"property_signature",
		"arrow_function",
		"class_declaration",
		"interface_declaration",
		"type_alias_declaration",
		"enum_declaration",
		"abstract_method_signature",
	),
	Python: stringSet(
		"import_statement",
		"import_from_statement",
		"function_definition",
		"class_definition",
	),
	PHP: stringSet(
		"namespace_definition",
		"namespace_use_declaration",
		"const_declaration",
		"function_definition",
		"method_declaration",
		"class_declaration",
		"interface_declaration",
		"trait_declaration",
		"enum_declaration",
	),
	Java:   stringSet("package_declaration", "import_declaration", "class_declaration", "interface_declaration", "enum_declaration", "record_declaration", "method_declaration"),
	Kotlin: stringSet("package_header", "import_header", "class_declaration", "object_declaration", "function_declaration", "property_declaration", "type_alias"),
	CSharp: stringSet("using_directive", "namespace_declaration", "class_declaration", "interface_declaration", "struct_declaration", "enum_declaration", "method_declaration", "property_declaration"),
	Cpp:    stringSet("preproc_include", "using_declaration", "function_definition", "class_specifier", "struct_specifier", "enum_specifier"),
	Ruby:   stringSet("class", "module", "method", "singleton_method"),
	Swift:  stringSet("import_declaration", "class_declaration", "struct_declaration", "enum_declaration", "protocol_declaration", "function_declaration", "init_declaration"),
}

func stringSet(items ...string) map[string]bool {
	m := make(map[string]bool, len(items))
	for _, it := range items {
		m[it] = true
	}
	return m
}

// Compressor produces signature summaries for a language.
type Compressor struct {
	grammars map[Language]*sitter.Language
	// parsers holds one reusable parser per language. Tree-sitter parser
	// creation/destruction crosses cgo, which is expensive per file; pooling
	// them avoids that churn across worker goroutines.
	parsers map[Language]*sync.Pool
}

// New returns a Compressor with all supported grammars loaded.
func New() *Compressor {
	grammars := map[Language]*sitter.Language{
		Go:         golang.GetLanguage(),
		Rust:       rust.GetLanguage(),
		JavaScript: javascript.GetLanguage(),
		TypeScript: typescript.GetLanguage(),
		TSX:        tsx.GetLanguage(),
		Python:     python.GetLanguage(),
		PHP:        php.GetLanguage(),
		Java:       java.GetLanguage(),
		Kotlin:     kotlin.GetLanguage(),
		CSharp:     csharp.GetLanguage(),
		Cpp:        cpp.GetLanguage(),
		Ruby:       ruby.GetLanguage(),
		Swift:      swift.GetLanguage(),
	}

	parsers := make(map[Language]*sync.Pool, len(grammars))
	for lang, grammar := range grammars {
		grammar := grammar
		parsers[lang] = &sync.Pool{
			New: func() any {
				p := sitter.NewParser()
				p.SetLanguage(grammar)
				return p
			},
		}
	}

	return &Compressor{
		grammars: grammars,
		parsers:  parsers,
	}
}

// LanguageForPath returns the language matching path's extension, if any.
func (c *Compressor) LanguageForPath(path string) (Language, bool) {
	lang, ok := extToLang[filepath.Ext(path)]
	return lang, ok
}

// Compress returns a signature-only summary of src for lang. The boolean
// result reports whether any declaration was emitted; false means the source
// is returned unchanged (nothing to compress or a parse failure).
func (c *Compressor) Compress(src []byte, lang Language) (string, bool) {
	pool, ok := c.parsers[lang]
	if !ok {
		return string(src), false
	}

	parser := pool.Get().(*sitter.Parser)
	defer pool.Put(parser)

	tree := parser.Parse(nil, src)
	if tree == nil {
		return string(src), false
	}
	root := tree.RootNode()

	var b strings.Builder
	c.walkDeclarations(root, src, lang, &b)
	if b.Len() == 0 {
		return string(src), false
	}
	return strings.TrimRight(b.String(), "\n"), true
}
