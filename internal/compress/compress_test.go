package compress

import (
	"strings"
	"sync"
	"testing"
)

func TestLanguageForPath(t *testing.T) {
	c := New()
	tests := []struct {
		path string
		want Language
		ok   bool
	}{
		{"main.go", Go, true},
		{"lib.rs", Rust, true},
		{"app.js", JavaScript, true},
		{"App.jsx", JavaScript, true},
		{"m.mjs", JavaScript, true},
		{"c.cjs", JavaScript, true},
		{"App.ts", TypeScript, true},
		{"App.tsx", TSX, true},
		{"util.py", Python, true},
		{"web.php", PHP, true},
		{"README.md", 0, false},
		{"noext", 0, false},
	}
	for _, tt := range tests {
		got, ok := c.LanguageForPath(tt.path)
		if ok != tt.ok || got != tt.want {
			t.Errorf("LanguageForPath(%q) = %v, %v; want %v, %v", tt.path, got, ok, tt.want, tt.ok)
		}
	}
}

func TestCompressGo(t *testing.T) {
	c := New()
	src := `package main

// add returns the sum of a and b.
func add(a, b int) int {
	sum := a + b
	_ = sum
	return a + b
}

type user struct {
	name string
}

// Greeter greets people.
type Greeter interface {
	Greet(name string) string
}

func (u *user) Greet(name string) string {
	return "hi " + name
}

func unexported() {
	// body
}
`
	out, _ := c.Compress([]byte(src), Go)
	lines := strings.Split(strings.TrimSpace(out), "\n")

	want := []string{
		"// add returns the sum of a and b.",
		"func add(a, b int) int",
		"type user struct",
		"// Greeter greets people.",
		"type Greeter interface",
		"func (u *user) Greet(name string) string",
		"func unexported()",
	}
	if len(lines) != len(want) {
		t.Fatalf("got %d lines, want %d:\n%s", len(lines), len(want), out)
	}
	for i, w := range want {
		if got := strings.TrimRight(lines[i], " \t"); got != w {
			t.Errorf("line %d = %q, want %q", i, got, w)
		}
	}
	if strings.Contains(out, "return a + b") || strings.Contains(out, "sum :=") {
		t.Errorf("body leaked into signatures:\n%s", out)
	}
}

func TestCompressJavaScriptDefaultParams(t *testing.T) {
	c := New()
	src := `// config builds a config object.
export function config(opts = { verbose: true, tags: ["a", "b"] }) {
	const v = opts.verbose;
	return v;
}

class Widget {
	// render draws the widget.
	render(width = 10, height = 20) {
		return width * height;
	}
}
`
	out, _ := c.Compress([]byte(src), JavaScript)
	lines := strings.Split(strings.TrimSpace(out), "\n")

	want := []string{
		"// config builds a config object.",
		"function config(opts = { verbose: true, tags: [\"a\", \"b\"] })",
		"class Widget",
		"// render draws the widget.",
		"render(width = 10, height = 20)",
	}
	for i, w := range want {
		if i >= len(lines) {
			t.Fatalf("missing line %d (%q); got:\n%s", i, w, out)
		}
		if got := strings.TrimRight(lines[i], " \t"); got != w {
			t.Errorf("line %d = %q, want %q", i, got, w)
		}
	}
	if strings.Contains(out, "return width") || strings.Contains(out, "const v") {
		t.Errorf("body leaked into signatures:\n%s", out)
	}
}

func TestCompressPython(t *testing.T) {
	c := New()
	src := `import os

def build(host: str, port: int = 8080, tags: list[str] = []) -> dict[str, str]:
    """Build a connection map."""
    return {"host": host}


class Server:
    def start(self) -> None:
        pass
`
	out, _ := c.Compress([]byte(src), Python)
	lines := strings.Split(strings.TrimSpace(out), "\n")

	want := []string{
		"def build(host: str, port: int = 8080, tags: list[str] = []) -> dict[str, str]:",
		"class Server:",
		"def start(self) -> None:",
	}
	for i, w := range want {
		if i >= len(lines) {
			t.Fatalf("missing line %d (%q); got:\n%s", i, w, out)
		}
		if got := strings.TrimRight(lines[i], " \t"); got != w {
			t.Errorf("line %d = %q, want %q", i, got, w)
		}
	}
	if strings.Contains(out, "return") {
		t.Errorf("body leaked into Python signatures:\n%s", out)
	}
}

func TestCompressFallback(t *testing.T) {
	c := New()
	src := "this is not parseable as code, just prose text without declarations"
	out, didCompress := c.Compress([]byte(src), Go)
	if out != src || didCompress {
		t.Errorf("fallback should return source unchanged with compressed=false, got %q, %v", out, didCompress)
	}
}

func TestCompressReportsCompressed(t *testing.T) {
	c := New()
	src := "package p\n\n// F does things.\nfunc F() int {\n\treturn 1\n}\n"
	out, didCompress := c.Compress([]byte(src), Go)
	if !didCompress || !strings.Contains(out, "func F() int") {
		t.Errorf("expected compressed output, got %q, %v", out, didCompress)
	}
}

func TestCompressBlankLineDetachesComment(t *testing.T) {
	c := New()
	src := `package p

// note about something unrelated

func f() int {
	return 1
}
`
	out, _ := c.Compress([]byte(src), Go)
	if strings.Contains(out, "note about something") {
		t.Errorf("blank-line comment leaked into signatures:\n%s", out)
	}
	if !strings.Contains(out, "func f() int") {
		t.Errorf("missing func signature:\n%s", out)
	}
}

// TestCompressConcurrent exercises the shared parser pool across goroutines;
// run with -race to catch parser reuse hazards.
func TestCompressConcurrent(t *testing.T) {
	c := New()
	src := []byte("package p\n\n// F does things.\nfunc F() int {\n\treturn 1\n}\n")

	const workers = 8
	const perWorker = 20
	expected, didCompress := c.Compress(src, Go)
	if !didCompress || !strings.Contains(expected, "func F() int") {
		t.Fatalf("baseline compress failed: %q, %v", expected, didCompress)
	}

	var wg sync.WaitGroup
	errs := make(chan string, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < perWorker; j++ {
				out, ok := c.Compress(src, Go)
				if !ok {
					errs <- "didCompress = false"
					return
				}
				if out != expected {
					errs <- "concurrent output differs from baseline"
					return
				}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}
