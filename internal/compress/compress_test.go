package compress

import (
	"strings"
	"sync"
	"testing"

	"github.com/bethropolis/sift/internal/lang"
)

func TestLanguageForPath(t *testing.T) {
	c := New()
	tests := []struct {
		path string
		want lang.ID
		ok   bool
	}{
		{"main.go", lang.Go, true},
		{"lib.rs", lang.Rust, true},
		{"app.js", lang.JavaScript, true},
		{"App.jsx", lang.JavaScript, true},
		{"m.mjs", lang.JavaScript, true},
		{"c.cjs", lang.JavaScript, true},
		{"App.ts", lang.TypeScript, true},
		{"App.tsx", lang.TSX, true},
		{"util.py", lang.Python, true},
		{"web.php", lang.PHP, true},
		{"Main.java", lang.Java, true},
		{"Main.kt", lang.Kotlin, true},
		{"Program.cs", lang.CSharp, true},
		{"main.cpp", lang.Cpp, true},
		{"app.rb", lang.Ruby, true},
		{"App.swift", lang.Swift, true},
		{"README.md", "", false},
		{"noext", "", false},
	}
	for _, tt := range tests {
		got, ok := c.LanguageForPath(tt.path)
		if ok != tt.ok || got != tt.want {
			t.Errorf("LanguageForPath(%q) = %v, %v; want %v, %v", tt.path, got, ok, tt.want, tt.ok)
		}
	}
}

func TestCompressAdditionalLanguages(t *testing.T) {
	tests := []struct {
		name string
		id   lang.ID
		src  string
	}{
		{"java", lang.Java, "package app; public class App { public void run() { int value = 1; } }"},
		{"kotlin", lang.Kotlin, "fun run() { println(\"ok\") }"},
		{"csharp", lang.CSharp, "class App { void Run() { var value = 1; } }"},
		{"cpp", lang.Cpp, "class App { void run() { int value = 1; } };"},
		{"ruby", lang.Ruby, "class App\n  def run\n    value = 1\n  end\nend"},
		{"swift", lang.Swift, "class App { func run() { let value = 1 } }"},
	}
	c := New()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if out, ok := c.Compress([]byte(tt.src), tt.id); !ok || out == tt.src {
				t.Fatalf("compression failed: ok=%v output=%q", ok, out)
			}
		})
	}
}

func TestCompressGo(t *testing.T) {
	c := New()
	src := `package main

import "fmt"

const greeting = "hi"

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
	out, didCompress := c.Compress([]byte(src), lang.Go)
	if !didCompress {
		t.Fatal("expected compression")
	}

	for _, w := range []string{
		"package main",
		`import "fmt"`,
		`const greeting = "hi"`,
		"// add returns the sum of a and b.",
		"func add(a, b int) int { /* ... */ }",
		"type user struct {\n\tname string\n}",
		"// Greeter greets people.",
		"type Greeter interface {\n\tGreet(name string) string\n}",
		"func (u *user) Greet(name string) string { /* ... */ }",
		"func unexported() { /* ... */ }",
	} {
		if !strings.Contains(out, w) {
			t.Errorf("missing %q in output:\n%s", w, out)
		}
	}
	if strings.Contains(out, "return a + b") || strings.Contains(out, "sum :=") {
		t.Errorf("body leaked into signatures:\n%s", out)
	}
	if strings.Contains(out, `"hi " + name`) {
		t.Errorf("method body leaked into signatures:\n%s", out)
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
	out, didCompress := c.Compress([]byte(src), lang.JavaScript)
	if !didCompress {
		t.Fatal("expected compression")
	}

	for _, w := range []string{
		"// config builds a config object.",
		"export function config(opts = { verbose: true, tags: [\"a\", \"b\"] }) { /* ... */ }",
		"class Widget {",
		"  // render draws the widget.",
		"  render(width = 10, height = 20) { /* ... */ }",
	} {
		if !strings.Contains(out, w) {
			t.Errorf("missing %q in output:\n%s", w, out)
		}
	}
	if strings.Contains(out, "return width") || strings.Contains(out, "const v") {
		t.Errorf("body leaked into signatures:\n%s", out)
	}
}

// TestCompressJavaScriptVariableFunction verifies the JS signature fix:
// const foo = (a, b) => {...} and export default function() {...} become
// signatures rather than missing or verbatim dumps of the function body.
func TestCompressJavaScriptVariableFunction(t *testing.T) {
	c := New()
	src := `// greet greets a name.
const greet = (name) => {
	return "hi " + name;
};

export default function init(opts) {
	return opts;
};
`
	out, didCompress := c.Compress([]byte(src), lang.JavaScript)
	if !didCompress {
		t.Fatal("expected compression")
	}

	for _, w := range []string{
		"// greet greets a name.",
		"const greet = (name) => { /* ... */ }",
		"export default function init(opts) { /* ... */ }",
	} {
		if !strings.Contains(out, w) {
			t.Errorf("missing %q in output:\n%s", w, out)
		}
	}
	if strings.Contains(out, `"hi " + name`) || strings.Contains(out, "return opts") {
		t.Errorf("function body leaked into signatures:\n%s", out)
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
	out, didCompress := c.Compress([]byte(src), lang.Python)
	if !didCompress {
		t.Fatal("expected compression")
	}

	for _, w := range []string{
		"import os",
		"def build(host: str, port: int = 8080, tags: list[str] = []) -> dict[str, str]: ...",
		"class Server:",
		"def start(self) -> None: ...",
	} {
		if !strings.Contains(out, w) {
			t.Errorf("missing %q in output:\n%s", w, out)
		}
	}
	if strings.Contains(out, "return") {
		t.Errorf("body leaked into Python signatures:\n%s", out)
	}
}

func TestCompressRust(t *testing.T) {
	c := New()
	src := `use std::collections::HashMap;

const MAX: usize = 100;
static NAME: &str = "sift";

/// A user record.
struct User {
    name: String,
    age: u32,
}

/// Builds a user.
impl User {
    pub fn new(name: String) -> Self {
        Self { name, age: 0 }
    }
}

fn greet(u: &User) -> String {
    format!("hi {}", u.name)
}
`
	out, didCompress := c.Compress([]byte(src), lang.Rust)
	if !didCompress {
		t.Fatal("expected compression")
	}

	for _, w := range []string{
		"use std::collections::HashMap;",
		"const MAX: usize = 100;",
		`static NAME: &str = "sift";`,
		"/// A user record.",
		"struct User {\n    name: String,\n    age: u32,\n}",
		"pub fn new(name: String) -> Self { /* ... */ }",
		"fn greet(u: &User) -> String { /* ... */ }",
	} {
		if !strings.Contains(out, w) {
			t.Errorf("missing %q in output:\n%s", w, out)
		}
	}
	if strings.Contains(out, "Self { name") || strings.Contains(out, "format!") {
		t.Errorf("body leaked into Rust signatures:\n%s", out)
	}
}

func TestCompressPHP(t *testing.T) {
	c := New()
	src := `<?php

namespace App;

use App\Models\User;

const VERSION = '1.0';

class Service
{
    public function handle(User $u): string
    {
        return $u->name;
    }
}
`
	out, didCompress := c.Compress([]byte(src), lang.PHP)
	if !didCompress {
		t.Fatal("expected compression")
	}

	for _, w := range []string{
		"namespace App;",
		"use App\\Models\\User;",
		"const VERSION = '1.0';",
		"class Service {",
		"public function handle(User $u): string { /* ... */ }",
	} {
		if !strings.Contains(out, w) {
			t.Errorf("missing %q in output:\n%s", w, out)
		}
	}
	if strings.Contains(out, "$u->name") {
		t.Errorf("method body leaked into PHP signatures:\n%s", out)
	}
}

func TestCompressFallback(t *testing.T) {
	c := New()
	src := "this is not parseable as code, just prose text without declarations"
	out, didCompress := c.Compress([]byte(src), lang.Go)
	if out != src || didCompress {
		t.Errorf("fallback should return source unchanged with compressed=false, got %q, %v", out, didCompress)
	}
}

func TestCompressReportsCompressed(t *testing.T) {
	c := New()
	src := "package p\n\n// F does things.\nfunc F() int {\n\treturn 1\n}\n"
	out, didCompress := c.Compress([]byte(src), lang.Go)
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
	out, _ := c.Compress([]byte(src), lang.Go)
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
	expected, didCompress := c.Compress(src, lang.Go)
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
				out, ok := c.Compress(src, lang.Go)
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
