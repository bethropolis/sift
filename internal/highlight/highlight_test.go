package highlight

import (
	"strings"
	"testing"
)

func TestRenderHighlightsCommonTokens(t *testing.T) {
	got := Render("main.go", []byte("func main() {\n\treturn 42 // done\n}"), Options{Enabled: true, Theme: ThemeAuto})
	if !strings.Contains(got, "\033[1;34mfunc\033[0m") {
		t.Errorf("keyword was not highlighted: %q", got)
	}
	if !strings.Contains(got, "\033[36m42\033[0m") {
		t.Errorf("number was not highlighted: %q", got)
	}
	if !strings.Contains(got, "\033[2m// done\033[0m") {
		t.Errorf("comment was not highlighted: %q", got)
	}
}

func TestRenderThemeAndDisabledModes(t *testing.T) {
	plain := "func main() {}"
	if got := Render("main.go", []byte(plain), Options{}); got != plain {
		t.Fatalf("disabled output changed: %q", got)
	}
	if got := Render("main.go", []byte(plain), Options{Enabled: true, Theme: ThemeNone}); got != plain {
		t.Fatalf("none theme changed output: %q", got)
	}
	if got := Render("main.go", []byte(plain), Options{Enabled: true, Theme: ThemeDark}); !strings.Contains(got, "\033[1;34m") {
		t.Errorf("dark theme did not emit style: %q", got)
	}
}

func TestANSIBuildsTruecolorSequences(t *testing.T) {
	if got := ANSI("#cba6f7", "1"); got != "\033[1;38;2;203;166;247m" {
		t.Errorf("ANSI bold = %q", got)
	}
	if got := ANSI("6c7086", "2"); got != "\033[2;38;2;108;112;134m" {
		t.Errorf("ANSI faint = %q", got)
	}
	if got := ANSI("#fab387"); got != "\033[38;2;250;179;135m" {
		t.Errorf("ANSI plain = %q", got)
	}
	for _, bad := range []string{"", "#12", "not-a-color", "#zzzzzz"} {
		if got := ANSI(bad); got != "" {
			t.Errorf("ANSI(%q) = %q, want empty", bad, got)
		}
	}
}

func TestCustomPaletteOverridesTheme(t *testing.T) {
	pal := Palette{
		Keyword:  ANSI("#ff79c6", "1"),
		String:   ANSI("#f1fa8c"),
		Number:   ANSI("#bd93f9"),
		Comment:  ANSI("#6272a4", "2"),
		TypeName: ANSI("#8be9fd"),
	}
	got := RenderLine("main.go", "func main() {}", Options{Enabled: true, Theme: ThemeAuto, Palette: &pal})
	if !strings.Contains(got, "\033[1;38;2;255;121;198mfunc\033[0m") {
		t.Errorf("custom palette keyword missing: %q", got)
	}
	if strings.Contains(got, "\033[1;34m") {
		t.Errorf("built-in palette leaked through custom palette: %q", got)
	}
}

func TestRenderSkipsUnsupportedAndOversizedContent(t *testing.T) {
	plain := "func main() {}"
	if got := Render("notes.txt", []byte(plain), Options{Enabled: true}); got != plain {
		t.Errorf("unsupported file changed: %q", got)
	}
	if got := Render("main.go", []byte(plain), Options{Enabled: true, MaxBytes: 2}); got != plain {
		t.Errorf("oversized content changed: %q", got)
	}
}

func TestRenderLineTrailingEscapeDoesNotPanic(t *testing.T) {
	opts := Options{Enabled: true, Theme: ThemeAuto}
	lines := []string{
		`name := "path\`,
		`s := "abc" + '\'`,
		`s := "unterminated`,
		`s := 'single\'`,
		`f("a" + "b")`,
	}
	for _, line := range lines {
		got := RenderLine("main.go", line, opts)
		if stripANSI(got) != line {
			t.Errorf("RenderLine(%q) altered content: %q", line, got)
		}
	}
}

func stripANSI(s string) string {
	b := &strings.Builder{}
	for i := 0; i < len(s); i++ {
		if s[i] == '\033' && i+1 < len(s) && s[i+1] == '[' {
			for i < len(s) && s[i] != 'm' {
				i++
			}
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}
func TestParseProducesSemanticSpans(t *testing.T) {
	doc := Parse("main.go", []byte("func greet(name string) bool { return true }"), Options{Enabled: true, Theme: ThemeAuto})
	if doc.Language != "go" {
		t.Fatalf("language = %q", doc.Language)
	}
	want := map[TokenKind]string{
		TokenKeyword:  "func",
		TokenFunction: "greet",
		TokenType:     "string",
		TokenBool:     "true",
	}
	for kind, text := range want {
		found := false
		for _, span := range doc.Lines[0].Spans {
			if span.Kind == kind && doc.Lines[0].Text[span.Start:span.End] == text {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing token kind %d span for %q: %+v", kind, text, doc.Lines[0].Spans)
		}
	}
}

func TestJSONKeysUsePropertySemantics(t *testing.T) {
	doc := Parse("config.json", []byte(`{"name": "sift", "enabled": true}`), Options{Enabled: true, Theme: ThemeAuto})
	for _, span := range doc.Lines[0].Spans {
		text := doc.Lines[0].Text[span.Start:span.End]
		if (text == `"name"` || text == `"enabled"`) && span.Kind != TokenProperty {
			t.Fatalf("JSON key %q classified as %d", text, span.Kind)
		}
	}
}

func TestBlockCommentEndsOnSameLine(t *testing.T) {
	doc := Parse("main.go", []byte("/* comment */ func main() {}"), Options{Enabled: true, Theme: ThemeAuto})
	if len(doc.Lines[0].Spans) == 0 || doc.Lines[0].Spans[0].Kind != TokenComment || doc.Lines[0].Spans[0].End != len("/* comment */") {
		t.Fatalf("same-line comment spans = %+v", doc.Lines[0].Spans)
	}
	if !hasSpan(doc.Lines[0], "func", TokenKeyword) {
		t.Fatalf("same-line comment consumed following code: %+v", doc.Lines[0].Spans)
	}
}

func TestPythonTripleQuotedStringState(t *testing.T) {
	state := scanState{}
	lines := []string{"def f():", "    \"\"\"doc", "    text", "    \"\"\"", "    return True"}
	var docs []Line
	for _, line := range lines {
		docs = append(docs, Line{Text: line, Spans: parseLine("python", line, &state)})
	}
	for _, lineNo := range []int{1, 2, 3} {
		line := docs[lineNo]
		for pos, r := range line.Text {
			if r == ' ' || r == '\t' {
				continue
			}
			covered := false
			for _, span := range line.Spans {
				if span.Kind == TokenString && pos >= span.Start && pos < span.End {
					covered = true
					break
				}
			}
			if !covered {
				t.Fatalf("triple-quoted line %d has unstyled byte %d: spans=%+v text=%q", lineNo, pos, line.Spans, line.Text)
			}
		}
	}
}

func hasSpan(line Line, text string, kind TokenKind) bool {
	for _, span := range line.Spans {
		if span.Kind == kind && line.Text[span.Start:span.End] == text {
			return true
		}
	}
	return false
}

func TestSpansAreOrderedAndInBounds(t *testing.T) {
	doc := Parse("main.go", []byte("/* open\nfunc x() { return \"a\\\\\\\"b\" } // done"), Options{Enabled: true, Theme: ThemeAuto})
	for lineNo, line := range doc.Lines {
		last := 0
		for _, span := range line.Spans {
			if span.Start < last || span.Start < 0 || span.End > len(line.Text) || span.End <= span.Start {
				t.Fatalf("line %d invalid span %+v for %q", lineNo, span, line.Text)
			}
			last = span.End
		}
	}
}

func TestRenderDocumentLineUsesSemanticPalette(t *testing.T) {
	pal := SyntaxPalette{TokenKeyword: {Foreground: "#ff0000", Bold: true}}
	doc := Parse("main.go", []byte("func main() {}"), Options{Enabled: true, Theme: ThemeAuto})
	got := RenderDocumentLine(doc, 0, Options{Enabled: true, Theme: ThemeAuto, Syntax: &pal})
	if !strings.Contains(got, "\033[1;38;2;255;0;0mfunc\033[0m") {
		t.Errorf("semantic render = %q", got)
	}
}

func TestSyntaxCacheReusesDocumentAcrossThemes(t *testing.T) {
	cache := NewSyntaxCache()
	content := []byte("func main() {}")
	first := cache.Get("main.go", content, Options{Enabled: true, Theme: ThemeAuto})
	second := cache.Get("main.go", content, Options{Enabled: true, Theme: ThemeAuto})
	if first != second {
		t.Fatal("cache reparsed unchanged content")
	}
	changed := cache.Get("main.go", []byte("func other() {}"), Options{Enabled: true, Theme: ThemeAuto})
	if changed == first {
		t.Fatal("cache reused content after source change")
	}
}

func TestSyntaxCacheEvictsAtCapacity(t *testing.T) {
	cache := NewSyntaxCache()
	cache.maxEntries = 2
	cache.Get("a.go", []byte("a"), Options{Enabled: true, Theme: ThemeAuto})
	cache.Get("b.go", []byte("b"), Options{Enabled: true, Theme: ThemeAuto})
	cache.Get("c.go", []byte("c"), Options{Enabled: true, Theme: ThemeAuto})
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if len(cache.entries) > 2 {
		t.Fatalf("cache grew beyond capacity: %d", len(cache.entries))
	}
}

func TestColorProfilesDegradeSemanticANSI(t *testing.T) {
	doc := Parse("main.go", []byte("func main() {}"), Options{Enabled: true, Theme: ThemeAuto})
	pal := SyntaxPalette{TokenKeyword: {Foreground: "#ff0000", Bold: true}}
	trueColor := RenderDocumentLine(doc, 0, Options{Enabled: true, Theme: ThemeAuto, Syntax: &pal, Profile: ProfileTrueColor})
	ansi256 := RenderDocumentLine(doc, 0, Options{Enabled: true, Theme: ThemeAuto, Syntax: &pal, Profile: ProfileANSI256})
	plain := RenderDocumentLine(doc, 0, Options{Enabled: true, Theme: ThemeAuto, Syntax: &pal, Profile: ProfileNone})
	if !strings.Contains(trueColor, "38;2;255;0;0") || !strings.Contains(ansi256, "38;5;") {
		t.Fatalf("color profiles did not degrade: true=%q ansi=%q", trueColor, ansi256)
	}
	if plain != "func main() {}" {
		t.Fatalf("none profile changed source: %q", plain)
	}
}

func TestANSI256PaletteIndexesRenderAndFallback(t *testing.T) {
	style := TokenStyle{Foreground: "ansi:129", Bold: true}
	ansi256 := ansiStyleProfile(style, ProfileANSI256)
	if !strings.Contains(ansi256, "38;5;129") {
		t.Fatalf("ANSI256 palette render = %q", ansi256)
	}
	ansi16 := ansiStyleProfile(style, ProfileANSI16)
	if ansi16 != "\033[1;34m" {
		t.Fatalf("ANSI16 palette render = %q, want basic blue fallback", ansi16)
	}
}

func TestParseLargeAndDisabledContentIsPlain(t *testing.T) {
	content := []byte("func main() {}")
	for _, options := range []Options{{}, {Enabled: true, Theme: ThemeNone}, {Enabled: true, Theme: ThemeAuto, MaxBytes: 1}} {
		doc := Parse("main.go", content, options)
		if len(doc.Lines) != 1 || len(doc.Lines[0].Spans) != 0 {
			t.Errorf("options %+v produced semantic spans: %+v", options, doc.Lines)
		}
	}
}

func TestSyntaxCacheUsesConfiguredLRUCapacity(t *testing.T) {
	cache := NewSyntaxCacheWithCapacity(2)
	cache.Get("a.go", []byte("a"), Options{Enabled: true, Theme: ThemeAuto})
	cache.Get("b.go", []byte("b"), Options{Enabled: true, Theme: ThemeAuto})
	cache.Get("a.go", []byte("a"), Options{Enabled: true, Theme: ThemeAuto}) // refresh a
	cache.Get("c.go", []byte("c"), Options{Enabled: true, Theme: ThemeAuto}) // evict b
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if len(cache.entries) != 2 || cache.lru.Len() != 2 {
		t.Fatalf("cache size = entries:%d lru:%d, want 2", len(cache.entries), cache.lru.Len())
	}
	if cache.lru.Back().Value.(*syntaxCacheEntry).path != "a.go" {
		t.Fatalf("least-recent entry = %q, want a.go", cache.lru.Back().Value.(*syntaxCacheEntry).path)
	}
}

func TestSpecialFilesProduceSemanticSpans(t *testing.T) {
	cases := []struct {
		path string
		text string
		kind TokenKind
	}{
		{"Dockerfile", "FROM golang:1.26", TokenKeyword},
		{"Makefile", "build:\n\tgo build ./...", TokenFunction},
		{"Justfile", "build:\n\tgo build ./...", TokenFunction},
		{".env", "PORT=8080", TokenProperty},
		{".gitignore", "vendor/", TokenString},
		{"LICENSE", "MIT License", TokenMarkupHeading},
	}
	for _, tc := range cases {
		doc := Parse(tc.path, []byte(tc.text), Options{Enabled: true, Theme: ThemeAuto})
		found := false
		for _, span := range doc.Lines[0].Spans {
			if span.Kind == tc.kind {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("%s spans = %+v, want kind %v", tc.path, doc.Lines[0].Spans, tc.kind)
		}
	}
}

func TestLegacyRenderHonorsNoColorProfile(t *testing.T) {
	plain := "func main() {}"
	got := Render("main.go", []byte(plain), Options{Enabled: true, Theme: ThemeAuto, Profile: ProfileNone})
	if got != plain {
		t.Fatalf("no-color profile changed output: %q", got)
	}
}
