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
