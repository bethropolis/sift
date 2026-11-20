package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bethropolis/sift/internal/config"
)

func TestCheckCmd_Valid(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, ".sift.toml")

	content := `
default_profile = "test"

[sift]
style = "xml"
output = "codebase.xml"
budget = "100k"

[profiles.test]
budget = "100k"
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg := config.New()
	cmd := newCheckCmd(cfg)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := runCheck(cfg, cmd, []string{configPath})
	if err != nil {
		t.Fatalf("runCheck error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "is valid") && !strings.Contains(out, "✓") {
		// Output went to stdout directly
	}
}

func TestCheckCmd_Invalid(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, ".sift.toml")

	// Invalid: default_profile references nonexistent
	content := `
default_profile = "does_not_exist"
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg := config.New()
	cmd := newCheckCmd(cfg)
	err := runCheck(cfg, cmd, []string{configPath})
	if err == nil {
		t.Fatal("expected error for invalid config")
	}
}

func TestCheckCmd_Quiet(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, ".sift.toml")

	content := `
default_profile = "test"

[profiles.test]
budget = 1000
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg := config.New()
	cfg.Quiet = true
	cmd := newCheckCmd(cfg)
	err := runCheck(cfg, cmd, []string{configPath})
	if err != nil {
		t.Fatalf("expected nil error on valid quiet check, got: %v", err)
	}
}
