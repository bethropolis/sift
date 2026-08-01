package compress

import (
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
)

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
