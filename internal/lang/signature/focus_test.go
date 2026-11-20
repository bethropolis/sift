//go:build cgo

package signature_test

import (
	"strings"
	"testing"

	_ "github.com/bethropolis/sift/internal/lang/go"
	_ "github.com/bethropolis/sift/internal/lang/python"
	_ "github.com/bethropolis/sift/internal/lang/rust"
	"github.com/bethropolis/sift/internal/lang/signature"
	"github.com/bethropolis/sift/internal/lang/types"
	_ "github.com/bethropolis/sift/internal/lang/typescript"
)

func focusOf(t *testing.T, id types.ID, src, symbol string) string {
	t.Helper()
	out, ok, err := signature.New().Focus([]byte(src), id, symbol, 0, 0)
	if err != nil {
		t.Fatalf("Focus(%s, %s): %v", id, symbol, err)
	}
	if !ok {
		t.Fatalf("Focus(%s, %s) emitted nothing", id, symbol)
	}
	return out
}

func TestFocusSymbolsPerLanguage(t *testing.T) {
	cases := []struct {
		name   string
		id     types.ID
		src    string
		symbol string
		want   string
		not    string
		sib    string
	}{
		{
			name: "go", id: types.Go,
			src:    "package p\n\nfunc Keep() int { return 1 }\n\nfunc Other() int { return 2 }\n",
			symbol: "Keep", want: "return 1", not: "return 2", sib: "func Other",
		},
		{
			name: "typescript", id: types.TypeScript,
			src:    "export function keep(): number { return 1; }\nexport function other(): number { return 2; }\n",
			symbol: "keep", want: "return 1", not: "return 2", sib: "other",
		},
		{
			name: "python", id: types.Python,
			src:    "def keep():\n    return 1\n\ndef other():\n    return 2\n",
			symbol: "keep", want: "return 1", not: "return 2", sib: "def other",
		},
		{
			name: "rust", id: types.Rust,
			src:    "fn keep() -> i32 { 1 }\n\nfn other() -> i32 { 2 }\n",
			symbol: "keep", want: "{ 1 }", not: "{ 2 }", sib: "other",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := focusOf(t, tc.id, tc.src, tc.symbol)
			if !strings.Contains(out, tc.want) {
				t.Fatalf("focused output missing body %q:\n%s", tc.want, out)
			}
			if strings.Contains(out, tc.not) {
				t.Fatalf("focused output leaked sibling body %q:\n%s", tc.not, out)
			}
			if !strings.Contains(out, tc.sib) {
				t.Fatalf("focused output missing sibling signature %q:\n%s", tc.sib, out)
			}
		})
	}
}

func TestFocusGoMethod(t *testing.T) {
	src := `package p

type T struct{}

func (t T) Keep() int { return 7 }

func (t T) Other() int { return 8 }
`
	out := focusOf(t, types.Go, src, "T.Keep")
	if !strings.Contains(out, "return 7") {
		t.Fatalf("method body missing:\n%s", out)
	}
	if strings.Contains(out, "return 8") {
		t.Fatalf("sibling method body leaked:\n%s", out)
	}
}

func TestFocusMissingSymbol(t *testing.T) {
	_, _, err := signature.New().Focus([]byte("package p\n\nfunc A() {}\n"), types.Go, "Missing", 0, 0)
	if err == nil {
		t.Fatal("expected an error for a missing symbol")
	}
}
