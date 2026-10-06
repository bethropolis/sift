package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeConfig(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, ".sift.toml"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// TestCheckProjectConfigClean accepts a benign config.
func TestCheckProjectConfigClean(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, `
default_profile = "x"
[sift]
style = "xml"
budget = 60000
[profiles.x]
style = "xml"
`)
	if issues := CheckProjectConfig(dir); len(issues) != 0 {
		t.Fatalf("clean config flagged: %v", issues)
	}
}

// TestCheckProjectConfigEscapes flags absolute and traversal paths in every
// section that reads or writes files.
func TestCheckProjectConfigEscapes(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, `
[sift]
prompt_file = "/etc/passwd"
output = "../../evil.md"
ui_theme_file = "../themes.toml"
[profiles.p]
output = "/tmp/x.md"
[[targets]]
name = "t"
output = "/tmp/y.md"
[prompts.q]
file = "../../secret.txt"
`)
	issues := CheckProjectConfig(dir)
	if len(issues) != 6 {
		t.Fatalf("got %d issues, want 6: %v", len(issues), issues)
	}
}

// TestCheckProjectConfigMissing is silent when there is no config.
func TestCheckProjectConfigMissing(t *testing.T) {
	if issues := CheckProjectConfig(t.TempDir()); len(issues) != 0 {
		t.Fatalf("missing config flagged: %v", issues)
	}
}

// TestConfigSizeCap ensures oversized configs fail instead of allocating.
func TestConfigSizeCap(t *testing.T) {
	dir := t.TempDir()
	big := strings.Repeat("x", maxConfigFileBytes+1)
	writeConfig(t, dir, big)
	if _, err := readConfigFile(filepath.Join(dir, ".sift.toml")); err == nil {
		t.Fatal("oversized config was accepted")
	}
	if issues := CheckProjectConfig(dir); len(issues) == 0 {
		t.Fatal("oversized config produced no issue")
	}
}
