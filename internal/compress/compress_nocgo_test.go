//go:build !cgo

package compress

import (
	"testing"

	"github.com/bethropolis/sift/internal/lang"
)

// TestNoCgoLanguageForPath guards against the regression where the no-cgo
// compressor dereferenced the nil Language returned for unregistered
// extensions (panicking the released static binary). Unknown paths must
// report ok=false without panicking.
func TestNoCgoLanguageForPath(t *testing.T) {
	c := New()
	tests := []struct {
		path string
		want lang.ID
		ok   bool
	}{
		{"main.go", lang.Go, true},
		{"lib.rs", lang.Rust, true},
		{"app.js", lang.JavaScript, true},
		{"README.md", "", false},
		{"noext", "", false},
		{"data.json", "", false},
	}
	for _, tt := range tests {
		got, ok := c.LanguageForPath(tt.path)
		if ok != tt.ok || got != tt.want {
			t.Errorf("LanguageForPath(%q) = %v, %v; want %v, %v", tt.path, got, ok, tt.want, tt.ok)
		}
	}
}

// TestNoCgoCompressKeepsSource verifies the no-cgo build never claims to have
// compressed a file; callers retain the full representation instead.
func TestNoCgoCompressKeepsSource(t *testing.T) {
	c := New()
	out, didCompress := c.Compress([]byte("package p\nfunc F() int { return 1 }\n"), lang.Go)
	if didCompress || out != "" {
		t.Errorf("no-cgo Compress = %q, %v; want empty, false", out, didCompress)
	}
}
