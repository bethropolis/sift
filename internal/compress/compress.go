// Package compress extracts signature-only summaries of source files using
// tree-sitter grammars. The result keeps doc comments and declaration
// signatures (function/method/type/class headers) while dropping bodies, so an
// LLM can reason about structure without paying for implementation tokens.
package compress

import (
	"path/filepath"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/golang"
	"github.com/smacker/go-tree-sitter/javascript"
	"github.com/smacker/go-tree-sitter/php"
	"github.com/smacker/go-tree-sitter/python"
	"github.com/smacker/go-tree-sitter/rust"
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
	}
	return "unknown"
}

var extToLang = map[string]Language{
	".go":  Go,
	".rs":  Rust,
	".js":  JavaScript,
	".jsx": JavaScript,
	".mjs": JavaScript,
	".cjs": JavaScript,
	".ts":  TypeScript,
	".tsx": TSX,
	".py":  Python,
	".pyi": Python,
	".php": PHP,
}

var declTypes = map[Language]map[string]bool{
	Go: stringSet(
		"function_declaration",
		"method_declaration",
		"type_declaration",
	),
	Rust: stringSet(
		"function_item",
		"struct_item",
		"enum_item",
		"trait_item",
		"type_item",
	),
	JavaScript: stringSet(
		"function_declaration",
		"generator_function_declaration",
		"method_definition",
		"arrow_function",
		"class_declaration",
	),
	TypeScript: stringSet(
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
		"function_definition",
		"class_definition",
	),
	PHP: stringSet(
		"function_definition",
		"method_declaration",
		"class_declaration",
		"interface_declaration",
		"trait_declaration",
		"enum_declaration",
	),
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
}

// New returns a Compressor with all supported grammars loaded.
func New() *Compressor {
	return &Compressor{
		grammars: map[Language]*sitter.Language{
			Go:         golang.GetLanguage(),
			Rust:       rust.GetLanguage(),
			JavaScript: javascript.GetLanguage(),
			TypeScript: typescript.GetLanguage(),
			TSX:        tsx.GetLanguage(),
			Python:     python.GetLanguage(),
			PHP:        php.GetLanguage(),
		},
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
	parser := sitter.NewParser()
	defer parser.Close()

	grammar, ok := c.grammars[lang]
	if !ok {
		return string(src), false
	}
	parser.SetLanguage(grammar)

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

func (c *Compressor) walkDeclarations(n *sitter.Node, src []byte, lang Language, b *strings.Builder) {
	if n.IsNamed() && declTypes[lang][n.Type()] {
		c.emit(n, src, lang, b)
	}
	for i := 0; i < int(n.ChildCount()); i++ {
		c.walkDeclarations(n.Child(i), src, lang, b)
	}
}

// emit writes doc comments plus a truncated signature for n. Declarations
// without a name are skipped; their children are still visited so class and
// interface members survive.
func (c *Compressor) emit(n *sitter.Node, src []byte, lang Language, b *strings.Builder) {
	if !hasName(n) {
		return
	}
	doc := docComment(src, n)
	sig := signature(src, n, lang)
	if sig == "" {
		return
	}
	if doc != "" {
		b.WriteString(doc)
	}
	b.WriteString(sig)
	b.WriteByte('\n')
}

// hasName reports whether n or one of its direct children carries a name
// field. Type declarations in Go wrap a type_spec, so the name lives one
// level down.
func hasName(n *sitter.Node) bool {
	if n.ChildByFieldName("name") != nil {
		return true
	}
	for i := 0; i < int(n.ChildCount()); i++ {
		if n.Child(i).ChildByFieldName("name") != nil {
			return true
		}
	}
	return false
}

// signature returns the declaration text up to (but excluding) its body,
// trimmed of trailing whitespace.
func signature(src []byte, n *sitter.Node, lang Language) string {
	start := int(n.StartByte())
	end := int(n.EndByte())
	text := src[start:end]

	if body := n.ChildByFieldName("body"); body != nil {
		cut := int(body.StartByte()) - start
		if cut > 0 && cut <= len(text) {
			return strings.TrimRight(string(text[:cut]), " \t\r\n")
		}
	}

	offset := scanBodyBoundary(src, n, lang)
	if offset > 0 {
		return strings.TrimRight(string(text[:offset]), " \t\r\n")
	}
	return strings.TrimRight(string(text), " \t\r\n")
}

// scanBodyBoundary finds the byte offset (relative to the node start) where the
// declaration body begins: the first { at bracket/paren depth zero for brace
// languages, the first : at depth zero for Python. Strings and comments are
// skipped so literals cannot cause a premature cut.
func scanBodyBoundary(src []byte, n *sitter.Node, lang Language) int {
	start := int(n.StartByte())
	end := int(n.EndByte())
	stopColon := lang == Python

	var paren, brack int
	var quote byte
	var lineComment, blockComment bool
	for i := start; i < end; i++ {
		ch := src[i]
		if blockComment {
			if ch == '*' && i+1 < end && src[i+1] == '/' {
				blockComment = false
				i++
			}
			continue
		}
		if lineComment {
			if ch == '\n' {
				lineComment = false
			}
			continue
		}
		if quote != 0 {
			if ch == '\\' {
				i++
				continue
			}
			if ch == quote {
				quote = 0
			}
			continue
		}
		switch ch {
		case '/':
			if i+1 < end && src[i+1] == '/' {
				lineComment = true
				i++
				continue
			}
			if i+1 < end && src[i+1] == '*' {
				blockComment = true
				i++
				continue
			}
		case '#':
			if lang == Python {
				lineComment = true
			}
		case '\'', '"', '`':
			quote = ch
		case '(':
			paren++
		case ')':
			if paren > 0 {
				paren--
			}
		case '[':
			brack++
		case ']':
			if brack > 0 {
				brack--
			}
		case '{':
			if paren == 0 && brack == 0 {
				return i - start
			}
		case ':':
			if stopColon && paren == 0 && brack == 0 && i+1 < end && src[i+1] == '\n' {
				return i - start + 1
			}
		}
	}
	return 0
}

// docComment returns the consecutive comment nodes immediately above the
// declaration, or "" when none are attached. A blank line detaches a comment.
// For wrappers like export_statement the comment attaches to the wrapper, so
// the search climbs parents until a comment is found or a non-wrapper is hit.
func docComment(src []byte, n *sitter.Node) string {
	cur := n
	for cur != nil {
		if parts := attachedComments(src, cur); len(parts) > 0 {
			for i := range parts {
				parts[i] = strings.TrimRight(parts[i], " \t\r")
			}
			return strings.Join(parts, "\n") + "\n"
		}
		parent := cur.Parent()
		if parent == nil || !isWrapper(parent) {
			break
		}
		cur = parent
	}
	return ""
}

var wrapperTypes = stringSet(
	"export_statement",
	"export_clause",
	"export_default",
	"module_item",
	"declaration",
)

func isWrapper(n *sitter.Node) bool {
	return n != nil && wrapperTypes[n.Type()]
}

func attachedComments(src []byte, n *sitter.Node) []string {
	var parts []string
	cur := n
	for {
		prev := cur.PrevNamedSibling()
		if prev == nil || prev.Type() != "comment" {
			break
		}
		if !attached(src, prev, cur) {
			break
		}
		parts = append(parts, prev.Content(src))
		cur = prev
	}
	return parts
}

// attached reports whether prev and next are separated by at most one newline.
func attached(src []byte, prev, next *sitter.Node) bool {
	gap := src[prev.EndByte():next.StartByte()]
	return strings.Count(string(gap), "\n") <= 1
}
