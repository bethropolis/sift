package secrets

import (
	"regexp"
	"strings"
	"testing"

	"github.com/bethropolis/sift/internal/format"
)

const pemBlock = `-----BEGIN RSA PRIVATE KEY-----
MIIEowIBAAKCAQEA7OcKZeTq1QZ9V1Vr9B4Xl3Wm5Q8cYxYzZ0aAbCdEfGhIjKlMnO
pQrStUvWxYzQ1B2C3D4E5F6G7H8I9J0K1L2M3N4O5P6Q7R8S9T0U1V2W3X4Y5Z6A7B8C9D0
E1F2G3H4I5J6K7L8M9N0O1P2Q3R4S5T6U7V8W9X0Y1Z2A3B4C5D6E7F8G9H0I1J2K3L4M5
-----END RSA PRIVATE KEY-----
`

func TestRedactContent(t *testing.T) {
	s := New()
	content := []byte(`const aws = "AKIAIOSFODNN7EXAMPLE"
const token = "ghp_AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
jwt=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c
private key below
` + pemBlock + `nothing here`)
	redacted, detections := s.RedactContent(content)

	if len(detections) == 0 {
		t.Fatal("expected detections, got none")
	}

	// Original secrets must not remain.
	for _, d := range detections {
		if strings.Contains(string(redacted), d.Match) {
			t.Errorf("secret still present after redaction: %q", d.Match)
		}
	}

	// Plain text must survive.
	if !strings.Contains(string(redacted), "nothing here") {
		t.Error("non-secret text was removed")
	}

	// Placeholders present.
	if !strings.Contains(string(redacted), "[REDACTED_SECRET:") {
		t.Error("missing redaction placeholder")
	}
}

func TestRedactContentClean(t *testing.T) {
	s := New()
	content := []byte("package main\n\nfunc main() { println(\"hi\") }\n")
	redacted, detections := s.RedactContent(content)
	if len(detections) != 0 {
		t.Errorf("unexpected detections: %+v", detections)
	}
	if string(redacted) != string(content) {
		t.Errorf("clean content changed: %q", redacted)
	}
}

func TestRedactContentEmpty(t *testing.T) {
	s := New()
	redacted, detections := s.RedactContent(nil)
	if len(redacted) != 0 {
		t.Errorf("empty content changed: %q", redacted)
	}
	if len(detections) != 0 {
		t.Errorf("unexpected detections for empty content: %+v", detections)
	}
}

// TestRedactContentKeywordGate proves the keyword fast-path skips a rule's
// regex unless one of its keywords appears in the content.
func TestRedactContentKeywordGate(t *testing.T) {
	kwRule := Rule{
		Name:     "kw-rule",
		Keywords: []string{"xyz"},
		Regex:    regexp.MustCompile(`secret[0-9]+`),
	}
	plainRule := Rule{
		Name:  "plain-rule",
		Regex: regexp.MustCompile(`token_[a-z]+`),
	}
	s := &Scanner{rules: []Rule{kwRule, plainRule}}

	t.Run("keyword absent skips regex", func(t *testing.T) {
		redacted, detections := s.RedactContent([]byte("nothing secret9 here"))
		if len(detections) != 0 {
			t.Errorf("expected no detections, got %+v", detections)
		}
		if string(redacted) != "nothing secret9 here" {
			t.Errorf("content mutated despite skipped regex: %q", redacted)
		}
	})

	t.Run("keyword present runs regex", func(t *testing.T) {
		redacted, detections := s.RedactContent([]byte("xyz secret123 here"))
		if len(detections) != 1 {
			t.Fatalf("expected 1 detection, got %+v", detections)
		}
		if strings.Contains(string(redacted), "secret123") {
			t.Errorf("secret leaked: %q", redacted)
		}
	})

	t.Run("keyword rule missing in second rule", func(t *testing.T) {
		redacted, detections := s.RedactContent([]byte("value token_abc done"))
		if len(detections) != 1 {
			t.Fatalf("expected 1 detection, got %+v", detections)
		}
		if strings.Contains(string(redacted), "token_abc") {
			t.Errorf("secret leaked: %q", redacted)
		}
	})
}

// TestRedactContentCaseVariants guards the keyword fast-path against case
// mismatches: keywords are matched case-insensitively so secrets whose keyword
// appears in a different case than the regex requires are still caught.
func TestRedactContentCaseVariants(t *testing.T) {
	s := New()
	tests := []struct {
		name    string
		content string
	}{
		{name: "stripe uppercase", content: `key=SK_live_4eC39HqLyjWDarjtT1zdp7dc`},
		{name: "1password uppercase", content: `key=A3-BXU2QB-KXREP4KQN3B5-MZQW5-NWQ5Z-KMZQ5`},
		{name: "github org token", content: `token=ghu_ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789ab`},
		{name: "github server token", content: `token=ghs_ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789ab`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			redacted, detections := s.RedactContent([]byte(tt.content))
			if len(detections) == 0 {
				t.Fatalf("expected detection, got none")
			}
			for _, d := range detections {
				if strings.Contains(string(redacted), d.Match) {
					t.Errorf("secret %q leaked through redaction: %q", d.Match, redacted)
				}
			}
		})
	}
}

// TestRedactSelectedFiles exercises concurrent scanning: every entry is
// sanitized, SecretCount is set per file, and non-secret content is untouched.
func TestRedactSelectedFiles(t *testing.T) {
	s := New()
	files := []format.FileEntry{
		{Path: "a.go", Content: []byte("const aws = \"AKIAIOSFODNN7EXAMPLE\"\n")},
		{Path: "b.go", Content: []byte("package main\n")},
		{Path: "c.go", Content: []byte("token=ghp_AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA\n")},
		{Path: "d.go", Content: []byte("func f() {}\n")},
	}

	out := s.RedactSelectedFiles(files)

	if len(out) != len(files) {
		t.Fatalf("got %d files, want %d", len(out), len(files))
	}
	for i, f := range out {
		if f.Path != files[i].Path {
			t.Errorf("path changed: got %q, want %q", f.Path, files[i].Path)
		}
	}

	if strings.Contains(string(out[0].Content), "AKIAIOSFODNN7EXAMPLE") {
		t.Error("aws secret leaked in a.go")
	}
	if out[0].SecretCount == 0 {
		t.Error("a.go SecretCount should be > 0")
	}
	if strings.Contains(string(out[2].Content), "ghp_") {
		t.Error("github secret leaked in c.go")
	}
	if out[2].SecretCount == 0 {
		t.Error("c.go SecretCount should be > 0")
	}
	if string(out[1].Content) != string(files[1].Content) {
		t.Error("clean b.go content changed")
	}
	if out[1].SecretCount != 0 {
		t.Errorf("b.go SecretCount = %d, want 0", out[1].SecretCount)
	}
	if string(out[3].Content) != string(files[3].Content) {
		t.Error("clean d.go content changed")
	}
}

func TestRedactSelectedFilesEmpty(t *testing.T) {
	s := New()
	if out := s.RedactSelectedFiles(nil); len(out) != 0 {
		t.Errorf("expected empty result, got %d files", len(out))
	}
}
