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

func TestRenderSkipsUnsupportedAndOversizedContent(t *testing.T) {
	plain := "func main() {}"
	if got := Render("notes.txt", []byte(plain), Options{Enabled: true}); got != plain {
		t.Errorf("unsupported file changed: %q", got)
	}
	if got := Render("main.go", []byte(plain), Options{Enabled: true, MaxBytes: 2}); got != plain {
		t.Errorf("oversized content changed: %q", got)
	}
}
