package highlight

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"
)

// TokenKind identifies the meaning of a source span without embedding color
// or terminal details.
type TokenKind uint8

const (
	TokenPlain TokenKind = iota
	TokenComment
	TokenDocComment
	TokenKeyword
	TokenString
	TokenStringEscape
	TokenRegex
	TokenNumber
	TokenBool
	TokenNull
	TokenType
	TokenFunction
	TokenVariable
	TokenConstant
	TokenProperty
	TokenBuiltin
	TokenOperator
	TokenPunctuation
	TokenAttribute
	TokenDecorator
	TokenTag
	TokenTagAttribute
	TokenMarkupHeading
	TokenMarkupLink
	TokenShebang
	TokenDiffAdded
	TokenDiffRemoved
	TokenDiffHunk
)

// Span is a half-open semantic range within a line. Offsets are byte offsets.
type Span struct {
	Start int
	End   int
	Kind  TokenKind
}

// Line contains original source text and its theme-independent spans.
type Line struct {
	Text  string
	Spans []Span
}

// Document is the cached semantic representation used by the preview.
type Document struct {
	Path     string
	Language string
	Lines    []Line
}

// TokenStyle describes appearance while keeping terminal rendering separate.
type TokenStyle struct {
	Foreground string
	Bold       bool
	Faint      bool
	Italic     bool
	Underline  bool
}

// SyntaxPalette maps semantic token meanings to appearance styles.
type SyntaxPalette map[TokenKind]TokenStyle

// Parse builds a conservative semantic document. Unknown languages receive
// generic lexical classes without attempting language inference.
func Parse(path string, content []byte, options Options) *Document {
	maxBytes := options.MaxBytes
	if maxBytes <= 0 {
		maxBytes = defaultMaxBytes
	}
	doc := &Document{Path: path, Language: languageForPath(path)}
	if !options.Enabled || options.Theme == ThemeNone || len(content) == 0 || len(content) > maxBytes {
		doc.Lines = plainLines(content)
		return doc
	}
	state := scanState{}
	lines := strings.Split(string(content), "\n")
	doc.Lines = make([]Line, len(lines))
	for i, text := range lines {
		doc.Lines[i] = Line{Text: text, Spans: parseLine(doc.Language, text, &state)}
	}
	return doc
}

func parseLine(language, line string, state *scanState) []Span {
	if language == "markdown" {
		return parseMarkdownLine(line, state)
	}
	if language == "diff" || language == "patch" {
		return parseDiffLine(line)
	}
	if language == "json" || language == "jsonc" {
		return parseJSONLine(line, state, language == "jsonc")
	}
	switch language {
	case "dockerfile":
		return parseDockerfileLine(line)
	case "makefile", "justfile":
		return parseBuildFileLine(line)
	case ".env", "env":
		return parseEnvLine(line)
	case ".gitignore", "gitignore":
		return parseGitignoreLine(line)
	case "license":
		return parseLicenseLine(line)
	case "yaml", "yml":
		return parseYAMLLine(line)
	case "toml":
		return parseTOMLLine(line)
	case "html", "xml", "vue", "svelte", "astro":
		return parseHTMLLine(line)
	case "css", "scss":
		return parseCSSLine(line)
	case "sh", "bash", "zsh":
		return parseShellLine(line)
	case "sql":
		return parseSQLLine(line)
	}

	var spans []Span
	appendSpan := func(start, end int, kind TokenKind) {
		if end <= start || start < 0 || end > len(line) {
			return
		}
		if len(spans) > 0 && start < spans[len(spans)-1].End {
			return
		}
		spans = append(spans, Span{Start: start, End: end, Kind: kind})
	}
	if state.inString {
		end := strings.Index(line, state.stringEnd)
		if end < 0 {
			appendSpan(0, len(line), TokenString)
			return spans
		}
		appendSpan(0, end+len(state.stringEnd), TokenString)
		state.inString = false
		return spans
	}
	commentPrefixes := []string{"//"}
	if language == "go" || language == "c" || language == "cpp" || language == "csharp" || language == "java" || language == "javascript" || language == "typescript" || language == "tsx" || language == "dart" || language == "zig" {
		commentPrefixes = append(commentPrefixes, "/*")
	}
	switch language {
	case "python", "sh", "bash", "zsh", "yaml", "yml", "toml", "ruby", "gitignore", ".env":
		commentPrefixes = append(commentPrefixes, "#")
	case "sql", "lua":
		commentPrefixes = append(commentPrefixes, "--")
	case "html", "xml", "vue", "svelte", "astro":
		commentPrefixes = append(commentPrefixes, "<!--")
	}
	if state.wordLanguage != language || state.wordSets == nil {
		state.wordLanguage = language
		state.wordSets = wordSetsForLanguage(language)
	}
	keywords := state.wordSets.keywords
	builtins := state.wordSets.builtins
	types := state.wordSets.types

	for i := 0; i < len(line); {
		if state.blockEnd != "" {
			end := strings.Index(line[i:], state.blockEnd)
			if end < 0 {
				appendSpan(i, len(line), TokenComment)
				break
			}
			appendSpan(i, i+end+len(state.blockEnd), TokenComment)
			i += end + len(state.blockEnd)
			state.blockEnd = ""
			continue
		}
		matchedComment := false
		for _, prefix := range commentPrefixes {
			if !strings.HasPrefix(line[i:], prefix) {
				continue
			}
			if prefix == "/*" || prefix == "<!--" {
				endToken := "*/"
				if prefix == "<!--" {
					endToken = "-->"
				}
				rest := line[i+len(prefix):]
				if end := strings.Index(rest, endToken); end >= 0 {
					commentEnd := i + len(prefix) + end + len(endToken)
					appendSpan(i, commentEnd, TokenComment)
					i = commentEnd
				} else {
					appendSpan(i, len(line), TokenComment)
					i = len(line)
					state.blockEnd = endToken
				}
			} else {
				appendSpan(i, len(line), TokenComment)
				i = len(line)
			}
			matchedComment = true
			break
		}
		if matchedComment {
			continue
		}
		if language == "python" && (strings.HasPrefix(line[i:], `"""`) || strings.HasPrefix(line[i:], `'''`)) {
			delimiter := line[i : i+3]
			start := i
			i += 3
			if end := strings.Index(line[i:], delimiter); end >= 0 {
				i += end + 3
				appendSpan(start, i, TokenString)
			} else {
				appendSpan(start, len(line), TokenString)
				state.inString = true
				state.stringEnd = delimiter
			}
			continue
		}
		if line[i] == '"' || line[i] == '\'' || line[i] == '`' {
			quote := line[i]
			start := i
			i++
			for i < len(line) {
				if line[i] == '\\' && i+1 < len(line) {
					i += 2
					continue
				}
				if line[i] == quote {
					i++
					break
				}
				i++
			}
			kind := TokenString
			if language == "json" {
				j := i
				for j < len(line) && (line[j] == ' ' || line[j] == '\t') {
					j++
				}
				if j < len(line) && line[j] == ':' {
					kind = TokenProperty
				}
			}
			appendSpan(start, i, kind)
			if i == len(line) && quote == '`' {
				state.inString = true
				state.stringEnd = string(quote)
			}
			continue
		}
		if unicode.IsDigit(rune(line[i])) || (line[i] == '.' && i+1 < len(line) && unicode.IsDigit(rune(line[i+1]))) {
			j := i + 1
			for j < len(line) && (unicode.IsDigit(rune(line[j])) || line[j] == '.' || line[j] == 'x' || line[j] == 'X' || line[j] == 'e' || line[j] == 'E' || line[j] == '-' || line[j] == '+') {
				j++
			}
			appendSpan(i, j, TokenNumber)
			i = j
			continue
		}
		if isIdentStart(line[i]) {
			j := i + 1
			for j < len(line) && isIdentPart(line[j]) {
				j++
			}
			word := line[i:j]
			kind := TokenPlain
			switch {
			case keywords[word]:
				kind = TokenKeyword
			case word == "true" || word == "false":
				kind = TokenBool
			case word == "nil" || word == "null" || word == "None":
				kind = TokenNull
			case builtins[word]:
				kind = TokenBuiltin
			case types[word] || (unicode.IsUpper(rune(word[0])) && language != "json"):
				kind = TokenType
			case j < len(line) && line[j] == '(':
				kind = TokenFunction
			case unicode.IsUpper(rune(word[0])):
				kind = TokenConstant
			}
			if kind != TokenPlain {
				appendSpan(i, j, kind)
			}
			i = j
			continue
		}
		if strings.ContainsRune("+-*/%=<>!&|^~?", rune(line[i])) {
			appendSpan(i, i+1, TokenOperator)
		} else if strings.ContainsRune(":;,.", rune(line[i])) {
			appendSpan(i, i+1, TokenPunctuation)
		}
		i++
	}
	return spans
}

// RenderDocumentLine renders one parsed line with the supplied palette.
func RenderDocumentLine(doc *Document, line int, options Options) string {
	if doc == nil || line < 0 || line >= len(doc.Lines) {
		return ""
	}
	entry := doc.Lines[line]
	if options.Syntax == nil || len(*options.Syntax) == 0 {
		return RenderLine(doc.Path, entry.Text, options)
	}
	return renderSpansWithProfile(entry.Text, entry.Spans, *options.Syntax, options.Profile)
}

func plainLines(content []byte) []Line {
	lines := strings.Split(string(content), "\n")
	out := make([]Line, len(lines))
	for i, text := range lines {
		out[i] = Line{Text: text}
	}
	return out
}

type scanState struct {
	blockEnd     string
	stringEnd    string
	inString     bool
	fence        bool
	wordLanguage string
	wordSets     *languageWordSets
}

func languageForPath(path string) string {
	base := strings.ToLower(filepath.Base(path))
	switch base {
	case "dockerfile", "makefile", "justfile", ".gitignore", ".env", "license":
		return base
	}
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), ".")
	switch ext {
	case "markdown":
		return "markdown"
	case "arb":
		return "json"
	case "jsonc":
		return "jsonc"
	case "py", "pyi":
		return "python"
	case "rs":
		return "rust"
	case "js", "jsx", "mjs", "cjs":
		return "javascript"
	case "ts":
		return "typescript"
	case "tsx":
		return "tsx"
	case "yml":
		return "yaml"
	case "bash", "zsh":
		return "sh"
	case "htm":
		return "html"
	case "dart":
		return "dart"
	case "zig":
		return "zig"
	case "cs":
		return "csharp"
	case "h":
		return "c"
	case "hpp", "cc", "cxx":
		return "cpp"
	default:
		return ext
	}
}

func renderSpansWithProfile(text string, spans []Span, palette SyntaxPalette, profile ColorProfile) string {
	if len(spans) == 0 || profile == ProfileNone {
		return text
	}
	var b strings.Builder
	last := 0
	for _, span := range spans {
		if span.Start < last || span.End <= span.Start || span.End > len(text) {
			continue
		}
		b.WriteString(text[last:span.Start])
		style, ok := palette[span.Kind]
		if !ok || style.Foreground == "" {
			b.WriteString(text[span.Start:span.End])
		} else {
			b.WriteString(ansiStyleProfile(style, profile))
			b.WriteString(text[span.Start:span.End])
			b.WriteString(reset)
		}
		last = span.End
	}
	b.WriteString(text[last:])
	return b.String()
}

func ansiStyle(style TokenStyle) string {
	return ansiStyleProfile(style, ProfileTrueColor)
}

func ansiStyleProfile(style TokenStyle, profile ColorProfile) string {
	if profile == ProfileNone || style.Foreground == "" {
		return ""
	}
	mods := make([]string, 0, 4)
	if style.Bold {
		mods = append(mods, "1")
	}
	if style.Faint {
		mods = append(mods, "2")
	}
	if style.Italic {
		mods = append(mods, "3")
	}
	if style.Underline {
		mods = append(mods, "4")
	}
	if strings.HasPrefix(style.Foreground, "ansi:") {
		return ansiIndexedStyle(style.Foreground, mods, profile)
	}
	if profile == ProfileANSI256 || profile == ProfileANSI16 {
		rgb, ok := parseHexRGB(style.Foreground)
		if !ok {
			return ""
		}
		color := ansi256(rgb)
		if profile == ProfileANSI16 {
			color = ansi16(rgb)
		}
		return "\033[" + strings.Join(append(mods, fmt.Sprintf("38;5;%d", color)), ";") + "m"
	}
	return ANSI(style.Foreground, mods...)
}

func ansiIndexedStyle(foreground string, mods []string, profile ColorProfile) string {
	value := strings.TrimPrefix(foreground, "ansi:")
	index, err := strconv.Atoi(value)
	if err != nil || index < 0 || index > 255 {
		return ""
	}
	if profile == ProfileANSI16 {
		index = ansi256ToANSI16(index)
		code := 30 + index
		if index >= 8 {
			code = 90 + index - 8
		}
		return "\033[" + strings.Join(append(mods, strconv.Itoa(code)), ";") + "m"
	}
	return "\033[" + strings.Join(append(mods, fmt.Sprintf("38;5;%d", index)), ";") + "m"
}

func ansi256ToANSI16(index int) int {
	if index < 16 {
		return index
	}
	if index >= 232 {
		if index >= 244 {
			return 15
		}
		if index <= 238 {
			return 0
		}
		return 8
	}
	// Approximate the xterm cube by its dominant color channel. This keeps
	// extended terminal palette entries readable on genuinely ANSI16-only
	// terminals without pretending they are exact RGB values.
	channel := (index - 16) / 36
	if channel == 0 {
		if (index-16)%36 >= 18 {
			return 1
		}
		return 9
	}
	if channel == 1 {
		if (index-16)%36 >= 18 {
			return 2
		}
		return 10
	}
	return 4
}

func parseHexRGB(hex string) ([3]int, bool) {
	hex = strings.TrimPrefix(strings.TrimSpace(hex), "#")
	if len(hex) == 3 {
		hex = string([]byte{hex[0], hex[0], hex[1], hex[1], hex[2], hex[2]})
	}
	if len(hex) != 6 {
		return [3]int{}, false
	}
	var rgb [3]int
	for i := range rgb {
		v, err := strconv.ParseUint(hex[2*i:2*i+2], 16, 8)
		if err != nil {
			return [3]int{}, false
		}
		rgb[i] = int(v)
	}
	return rgb, true
}

func ansi256(rgb [3]int) int {
	if rgb[0] == rgb[1] && rgb[1] == rgb[2] {
		if rgb[0] < 8 {
			return 16
		}
		if rgb[0] > 248 {
			return 231
		}
		return 232 + (rgb[0]-8)/10
	}
	return 16 + 36*(rgb[0]*5/255) + 6*(rgb[1]*5/255) + rgb[2]*5/255
}

func ansi16(rgb [3]int) int {
	r, g, b := rgb[0] > 127, rgb[1] > 127, rgb[2] > 127
	if r && g && b {
		return 15
	}
	if r && g {
		return 11
	}
	if g && b {
		return 10
	}
	if r && b {
		return 13
	}
	if r {
		return 9
	}
	if g {
		return 10
	}
	if b {
		return 12
	}
	return 0
}

func parseMarkdownLine(line string, state *scanState) []Span {
	trimmed := strings.TrimLeft(line, " \t")
	indent := len(line) - len(trimmed)
	if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
		state.fence = !state.fence
		return []Span{{Start: indent, End: len(line), Kind: TokenMarkupHeading}}
	}
	if state.fence {
		return nil
	}
	if strings.HasPrefix(trimmed, "#") {
		return []Span{{Start: indent, End: len(line), Kind: TokenMarkupHeading}}
	}
	return nil
}

func parseDiffLine(line string) []Span {
	switch {
	case strings.HasPrefix(line, "@@"):
		return []Span{{Start: 0, End: len(line), Kind: TokenDiffHunk}}
	case strings.HasPrefix(line, "+"):
		return []Span{{Start: 0, End: len(line), Kind: TokenDiffAdded}}
	case strings.HasPrefix(line, "-"):
		return []Span{{Start: 0, End: len(line), Kind: TokenDiffRemoved}}
	default:
		return nil
	}
}
