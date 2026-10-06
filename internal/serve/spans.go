package serve

import (
	"github.com/bethropolis/sift/internal/highlight"
)

// highlightSpanOptions is the fixed parse contract for preview spans:
// theme-independent, capped at the preview size. Constant options keep the
// shared SyntaxCache hot across requests for the same content.
func highlightSpanOptions() highlight.Options {
	return highlight.Options{Enabled: true, MaxBytes: previewCap}
}

// highlightKindName maps every TokenKind to its stable wire name. The web
// client groups these into visual classes (documented in serve-protocol.md);
// unknown kinds fall through to plain rendering.
func highlightKindName(k highlight.TokenKind) string {
	switch k {
	case highlight.TokenComment:
		return "comment"
	case highlight.TokenDocComment:
		return "doccomment"
	case highlight.TokenKeyword:
		return "keyword"
	case highlight.TokenString:
		return "string"
	case highlight.TokenStringEscape:
		return "stringescape"
	case highlight.TokenRegex:
		return "regex"
	case highlight.TokenNumber:
		return "number"
	case highlight.TokenBool:
		return "bool"
	case highlight.TokenNull:
		return "null"
	case highlight.TokenType:
		return "type"
	case highlight.TokenFunction:
		return "function"
	case highlight.TokenVariable:
		return "variable"
	case highlight.TokenConstant:
		return "constant"
	case highlight.TokenProperty:
		return "property"
	case highlight.TokenBuiltin:
		return "builtin"
	case highlight.TokenOperator:
		return "operator"
	case highlight.TokenPunctuation:
		return "punctuation"
	case highlight.TokenAttribute:
		return "attribute"
	case highlight.TokenDecorator:
		return "decorator"
	case highlight.TokenTag:
		return "tag"
	case highlight.TokenTagAttribute:
		return "tagattribute"
	case highlight.TokenMarkupHeading:
		return "markupheading"
	case highlight.TokenMarkupLink:
		return "markuplink"
	case highlight.TokenShebang:
		return "shebang"
	case highlight.TokenDiffAdded:
		return "diffadded"
	case highlight.TokenDiffRemoved:
		return "diffremoved"
	case highlight.TokenDiffHunk:
		return "diffhunk"
	default:
		return "plain"
	}
}

// fileSpans parses content into per-line span tuples
// [line, start, end, kind] with byte offsets into each line. Content must be
// the exact bytes sent to the client (post-redaction, post-truncation).
// Empty documents yield nil: the client renders plain text.
func fileSpans(absPath string, content []byte, cache *highlight.SyntaxCache) [][4]any {
	if cache == nil || len(content) == 0 {
		return nil
	}
	doc := cache.Get(absPath, content, highlightSpanOptions())
	var out [][4]any
	for li, line := range doc.Lines {
		for _, sp := range line.Spans {
			if sp.End <= sp.Start || sp.Start < 0 || sp.End > len(line.Text) {
				continue
			}
			name := highlightKindName(sp.Kind)
			if name == "plain" {
				continue
			}
			out = append(out, [4]any{li, sp.Start, sp.End, name})
		}
	}
	return out
}
