package secrets

import (
	"strings"
	"testing"
)

func TestRedact(t *testing.T) {
	s := New()
	content := []byte(`const aws = "AKIAIOSFODNN7EXAMPLE"
const token = "ghp_AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
jwt=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxIn0.signature
private key below
-----BEGIN RSA PRIVATE KEY-----
nothing here`)
	redacted, detections := s.Redact(content)

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

func TestRedactCleanContent(t *testing.T) {
	s := New()
	content := []byte("package main\n\nfunc main() { println(\"hi\") }\n")
	redacted, detections := s.Redact(content)
	if len(detections) != 0 {
		t.Errorf("unexpected detections: %+v", detections)
	}
	if string(redacted) != string(content) {
		t.Errorf("clean content changed: %q", redacted)
	}
}
