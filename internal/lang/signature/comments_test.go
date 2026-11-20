//go:build cgo

package signature_test

import (
	"strings"
	"testing"

	_ "github.com/bethropolis/sift/internal/lang/go"
	_ "github.com/bethropolis/sift/internal/lang/python"
	"github.com/bethropolis/sift/internal/lang/signature"
	"github.com/bethropolis/sift/internal/lang/types"
	_ "github.com/bethropolis/sift/internal/lang/typescript"
)

func signatureFor(t *testing.T, id types.ID, src string) string {
	t.Helper()
	engine := signature.New()
	out, ok := engine.Signature([]byte(src), id)
	if !ok {
		t.Fatalf("no signature emitted for %s", id)
	}
	return out
}

func assertOrder(t *testing.T, out string, lines ...string) {
	t.Helper()
	last := -1
	for _, line := range lines {
		idx := strings.Index(out, line)
		if idx < 0 {
			t.Fatalf("missing %q in signature output:\n%s", line, out)
		}
		if idx < last {
			t.Fatalf("%q out of order in signature output:\n%s", line, out)
		}
		last = idx
	}
}

func TestMultiLineGoDocCommentOrder(t *testing.T) {
	src := `package demo

// First line.
// Second line.
// Third line.
func Exported() {
}
`
	out := signatureFor(t, types.Go, src)
	assertOrder(t, out, "// First line.", "// Second line.", "// Third line.", "func Exported()")
}

func TestMultiLineNumberedCommentOrder(t *testing.T) {
	src := `package demo

// Steps:
// 1. first
// 2. second
// 3. third
func Steps() {}
`
	out := signatureFor(t, types.Go, src)
	assertOrder(t, out, "// 1. first", "// 2. second", "// 3. third")
}

func TestMultiLineTypeScriptDocCommentOrder(t *testing.T) {
	src := `// Alpha.
// Beta.
// Gamma.
export function run(): void {
  console.log("x");
}
`
	out := signatureFor(t, types.TypeScript, src)
	assertOrder(t, out, "// Alpha.", "// Beta.", "// Gamma.", "export function run")
}

func TestMultiLinePythonDocCommentOrder(t *testing.T) {
	src := `# one
# two
# three
def run():
    pass
`
	out := signatureFor(t, types.Python, src)
	assertOrder(t, out, "# one", "# two", "# three", "def run")
}

// TestMultiLineSignatureKeepsIndentation guards the parameter list of a
// multi-line declaration against being flattened.
func TestMultiLineSignatureKeepsIndentation(t *testing.T) {
	src := `package demo

func Wide(
	alpha int,
	beta string,
) error {
	return nil
}
`
	out := signatureFor(t, types.Go, src)
	if !strings.Contains(out, "\talpha int,") {
		t.Fatalf("parameter indentation was stripped:\n%s", out)
	}
}
