//go:build cgo

package compress

import (
	"strings"

	sitter "github.com/smacker/go-tree-sitter"

	"github.com/bethropolis/sift/internal/lang"
)

// walkDeclarations visits the AST in document order, emitting declarations the
// tree can summarize. Nodes handled by emitNode are emitted with their full
// structure (headers, type bodies, placeholders) and their interiors are not
// re-walked, so class members and import specs are never emitted twice.
func (c *Compressor) walkDeclarations(n *sitter.Node, src []byte, spec *lang.SignatureSpec, b *strings.Builder) {
	if n.IsNamed() && spec.Declarations[n.Type()] {
		if c.emitNode(n, src, spec, b) {
			return
		}
	}
	for i := 0; i < int(n.ChildCount()); i++ {
		c.walkDeclarations(n.Child(i), src, spec, b)
	}
}

// emitNode writes a declaration and reports whether it was fully handled (in
// which case the caller must not descend, since the interior was already
// emitted). The branches are:
//   - a variable declaration that initialises a function (JS/TS): header plus
//     a signature, so arrow-function bodies are dropped instead of dumped;
//   - headers/imports/constants: verbatim, they carry no bodies to strip;
//   - type definitions: structure preserved (fields and interface contracts),
//     with class/interface method bodies compressed;
//   - functions and methods: header signature plus a body placeholder.
func (c *Compressor) emitNode(n *sitter.Node, src []byte, spec *lang.SignatureSpec, b *strings.Builder) bool {
	nodeType := n.Type()

	// 1. const/let/var value = () => { ... } becomes a signature.
	if spec.VariableWithFunction != nil && spec.VariableWithFunction(n, src) {
		c.emitVariableFunctionSignature(n, src, spec, b)
		return true
	}

	// 2. Package, imports, and const/var declarations carry no bodies.
	if spec.HeaderOrConst(nodeType) {
		b.WriteString(strings.TrimSpace(n.Content(src)))
		b.WriteString("\n\n")
		return true
	}

	// 3. Type definitions preserve their interior structure.
	if spec.TypeDefinition(nodeType) {
		c.emitTypeWithStructure(n, src, spec, b)
		return true
	}

	// 4. Functions and methods keep their header plus a body placeholder.
	if spec.FunctionOrMethod(nodeType) {
		doc := docComment(src, n)
		sig := signatureFrom(declStart(n), src, n, spec)
		if sig == "" {
			return true
		}
		if doc != "" {
			b.WriteString(doc)
		}
		b.WriteString(sig)
		if spec.PythonLike {
			b.WriteString(" ...\n\n")
		} else {
			b.WriteString(" { /* ... */ }\n\n")
		}
		return true
	}

	return false
}

// declStart returns the byte offset where the declaration's text begins,
// climbing export wrappers so the "export" keyword survives for JS/TS.
func declStart(n *sitter.Node) int {
	start := int(n.StartByte())
	for p := n.Parent(); p != nil && isExportWrapper(p); p = p.Parent() {
		start = int(p.StartByte())
	}
	return start
}

// isExportWrapper reports whether a node prefixes a declaration with the
// "export" keyword in JS/TS grammars.
func isExportWrapper(n *sitter.Node) bool {
	switch n.Type() {
	case "export_statement", "export_clause", "export_default":
		return true
	}
	return false
}

// emitVariableFunctionSignature emits a const/let/var declaration that
// initialises an arrow function or function expression, replacing only the
// function body with a placeholder (e.g. const foo = (a, b) => { /* ... */ }).
func (c *Compressor) emitVariableFunctionSignature(n *sitter.Node, src []byte, spec *lang.SignatureSpec, b *strings.Builder) {
	if doc := docComment(src, n); doc != "" {
		b.WriteString(doc)
	}

	start := declStart(n)
	end := int(n.EndByte())
	for i := 0; i < int(n.ChildCount()); i++ {
		child := n.Child(i)
		if child.Type() == "variable_declarator" {
			if v := child.ChildByFieldName("value"); v != nil &&
				(v.Type() == "arrow_function" || v.Type() == "function_expression") {
				if body := v.ChildByFieldName("body"); body != nil {
					end = int(body.StartByte())
				}
				break
			}
		}
	}

	b.WriteString(strings.TrimRight(string(src[start:end]), " \t\r\n"))
	if spec.PythonLike {
		b.WriteString("\n\n")
	} else {
		b.WriteString(" { /* ... */ }\n\n")
	}
}

// emitTypeWithStructure emits a type definition preserving its interior:
// structs, interfaces, enums, and aliases contain only declarations so their
// full text is kept; classes contain method bodies which are compressed.
func (c *Compressor) emitTypeWithStructure(n *sitter.Node, src []byte, spec *lang.SignatureSpec, b *strings.Builder) {
	if doc := docComment(src, n); doc != "" {
		b.WriteString(doc)
	}

	if isContainerType(n.Type()) {
		if body := n.ChildByFieldName("body"); body != nil {
			c.emitContainer(n, body, src, spec, b)
			return
		}
	}

	// Struct/interface/enum/alias bodies hold declarations only; emit verbatim.
	b.WriteString(strings.TrimSpace(string(src[declStart(n):int(n.EndByte())])))
	b.WriteString("\n\n")
}

// emitContainer writes a class header followed by its members indented two
// spaces, compressing method bodies. Python classes end at the header colon
// (no closing brace).
func (c *Compressor) emitContainer(n, body *sitter.Node, src []byte, spec *lang.SignatureSpec, b *strings.Builder) {
	header := strings.TrimRight(string(src[declStart(n):int(body.StartByte())]), " \t\r\n")
	b.WriteString(header)
	if spec.PythonLike {
		b.WriteString("\n")
	} else {
		b.WriteString(" {\n")
	}

	for i := 0; i < int(body.ChildCount()); i++ {
		child := body.Child(i)
		if !child.IsNamed() {
			continue
		}
		switch {
		case spec.FunctionOrMethod(child.Type()):
			if doc := docComment(src, child); doc != "" {
				b.WriteString("  " + strings.TrimRight(strings.ReplaceAll(strings.TrimSpace(doc), "\n", "\n  "), " ") + "\n")
			}
			sig := signatureFrom(declStart(child), src, child, spec)
			if sig != "" {
				b.WriteString("  " + sig)
				if spec.PythonLike {
					b.WriteString(" ...\n")
				} else {
					b.WriteString(" { /* ... */ }\n")
				}
			}
		case isComment(child.Type()):
			// A comment immediately above a method is its doc and is emitted
			// by that branch; only standalone comments are written here.
			if next := child.NextNamedSibling(); next != nil && spec.FunctionOrMethod(next.Type()) && attached(src, child, next) {
				continue
			}
			b.WriteString("  " + strings.TrimSpace(child.Content(src)) + "\n")
		default:
			// Fields, properties, and nested declarations: verbatim.
			if text := strings.TrimSpace(child.Content(src)); text != "" {
				b.WriteString("  " + text + "\n")
			}
		}
	}

	if spec.PythonLike {
		b.WriteString("\n")
	} else {
		b.WriteString("}\n")
	}
	b.WriteString("\n")
}

// isContainerType reports whether a type node's body can contain method
// bodies that must be compressed (classes and Python class definitions).
func isContainerType(t string) bool {
	return t == "class_definition" || t == "class_declaration"
}

// isComment reports whether a node type is a comment. Grammars differ: Rust
// uses line_comment/block_comment, Go and JS use comment.
func isComment(t string) bool {
	return strings.Contains(t, "comment")
}

// signatureFrom returns the declaration text from start (inclusive) up to (but
// excluding) its body, trimmed of trailing whitespace. start may precede the
// node (an export wrapper); the body boundary is resolved from the node itself.
func signatureFrom(start int, src []byte, n *sitter.Node, spec *lang.SignatureSpec) string {
	end := int(n.EndByte())
	text := src[start:end]

	if body := n.ChildByFieldName("body"); body != nil {
		cut := int(body.StartByte()) - start
		if cut > 0 && cut <= len(text) {
			return strings.TrimRight(string(text[:cut]), " \t\r\n")
		}
	}

	offset := scanBodyBoundary(src, n, spec)
	if offset > 0 {
		return strings.TrimRight(string(text[:offset]), " \t\r\n")
	}
	return strings.TrimRight(string(text), " \t\r\n")
}

// scanBodyBoundary finds the byte offset (relative to the node start) where the
// declaration body begins: the first { at bracket/paren depth zero for brace
// languages, the first : at depth zero for Python. Strings and comments are
// skipped so literals cannot cause a premature cut.
func scanBodyBoundary(src []byte, n *sitter.Node, spec *lang.SignatureSpec) int {
	start := int(n.StartByte())
	end := int(n.EndByte())
	stopColon := spec.PythonLike

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
			if spec.PythonLike {
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
