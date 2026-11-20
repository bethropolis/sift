//go:build cgo

package signature_test

import (
	"strings"
	"testing"

	_ "github.com/bethropolis/sift/internal/lang/go"
	"github.com/bethropolis/sift/internal/lang/signature"
	"github.com/bethropolis/sift/internal/lang/types"
)

func TestSignatureOutlineOmitsDocComments(t *testing.T) {
	src := `package p

// Config stores settings.
type Config struct {
	Name string
}

// Load reads config.
func Load() Config {
	return Config{}
}
`
	e := signature.New()
	withDocs, ok := e.Signature([]byte(src), types.Go)
	if !ok || !strings.Contains(withDocs, "Config stores settings") {
		t.Fatalf("signatures should keep doc comments:\n%s", withDocs)
	}
	outline, ok := e.SignatureOutline([]byte(src), types.Go)
	if !ok {
		t.Fatal("outline emitted nothing")
	}
	if strings.Contains(outline, "Config stores settings") || strings.Contains(outline, "Load reads config") {
		t.Fatalf("outline kept doc comments:\n%s", outline)
	}
	if !strings.Contains(outline, "type Config struct") || !strings.Contains(outline, "func Load()") {
		t.Fatalf("outline lost declarations:\n%s", outline)
	}
}
