package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateConfigFile_Valid(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, ".sift.toml")

	content := `
default_profile = "full"

[sift]
style = "xml"
output = "codebase.xml"
budget = "150k"
compress_mode = "full"
fallback_mode = "signatures"
fallback_threshold = 0.9
ignore = ["vendor/**", "build/**"]

[modes]
full = ["cmd/**", "internal/**"]
signatures = ["web/**"]
skip = ["**/*.test.js"]

[profiles.full]
budget = "200k"

[profiles.tests]
extends = ["full"]
ignore = ["testdata/**"]

[[targets]]
name = "guidance"
profile = "full"
style = "markdown"
output = "guidance.md"
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	result, err := ValidateConfigFile(configPath, tempDir)
	if err != nil {
		t.Fatalf("ValidateConfigFile failed: %v", err)
	}

	if !result.Valid {
		t.Fatalf("expected valid config, got issues: %v", result.Issues)
	}

	if result.Summary.DefaultProfile != "full" {
		t.Errorf("expected default_profile 'full', got %q", result.Summary.DefaultProfile)
	}
	if len(result.Summary.Profiles) != 2 {
		t.Errorf("expected 2 profiles, got %d", len(result.Summary.Profiles))
	}
	if len(result.Summary.Targets) != 1 {
		t.Errorf("expected 1 target, got %d", len(result.Summary.Targets))
	}
	if result.Summary.Modes == nil || result.Summary.Modes.FullCount != 2 {
		t.Errorf("expected modes summary with 2 full globs, got %v", result.Summary.Modes)
	}
}

func TestValidateConfigFile_TableOrdering(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, ".sift.toml")

	// Misplaced default_profile after table header
	content := `
[sift]
style = "xml"

default_profile = "full"

[profiles.full]
budget = 100000
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	result, err := ValidateConfigFile(configPath, tempDir)
	if err != nil {
		t.Fatalf("ValidateConfigFile: %v", err)
	}

	if result.Valid {
		t.Errorf("expected invalid config due to misplaced default_profile")
	}

	foundOrderError := false
	for _, issue := range result.Errors() {
		if issue.Field == "default_profile" {
			foundOrderError = true
			break
		}
	}
	if !foundOrderError {
		t.Errorf("expected default_profile table order error, got: %v", result.Issues)
	}
}

func TestValidateConfigFile_MissingDefaultProfile(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, ".sift.toml")

	content := `
default_profile = "nonexistent"

[profiles.full]
budget = 100000
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	result, err := ValidateConfigFile(configPath, tempDir)
	if err != nil {
		t.Fatalf("ValidateConfigFile: %v", err)
	}

	if result.Valid {
		t.Errorf("expected invalid config for missing default_profile target")
	}
}

func TestValidateConfigFile_CircularExtends(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, ".sift.toml")

	content := `
[profiles.a]
extends = ["b"]

[profiles.b]
extends = ["a"]
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	result, err := ValidateConfigFile(configPath, tempDir)
	if err != nil {
		t.Fatalf("ValidateConfigFile: %v", err)
	}

	if result.Valid {
		t.Errorf("expected circular extends to fail validation")
	}
}

func TestValidateConfigFile_ConflictingPrompt(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, ".sift.toml")

	content := `
[sift]
prompt = "hello"
prompt_file = "prompt.md"
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	result, err := ValidateConfigFile(configPath, tempDir)
	if err != nil {
		t.Fatalf("ValidateConfigFile: %v", err)
	}

	if result.Valid {
		t.Errorf("expected conflicting prompt/prompt_file to fail validation")
	}
}

func TestValidateConfigFile_SecurityEscape(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, ".sift.toml")

	content := `
[sift]
output = "../outside.xml"
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	result, err := ValidateConfigFile(configPath, tempDir)
	if err != nil {
		t.Fatalf("ValidateConfigFile: %v", err)
	}

	if result.Valid {
		t.Errorf("expected path escape to fail validation")
	}
}

func TestValidateConfigFile_Nonexistent(t *testing.T) {
	tempDir := t.TempDir()
	_, err := ValidateConfigFile("", tempDir)
	if err == nil {
		t.Errorf("expected error for nonexistent .sift.toml")
	}
}

func TestValidateConfigFile_OwnSiftToml(t *testing.T) {
	// Root of repo contains .sift.toml
	root := filepath.Join("..", "..")
	siftToml := filepath.Join(root, ".sift.toml")
	if _, err := os.Stat(siftToml); err != nil {
		t.Skip("skipping, .sift.toml not found at repo root")
	}

	res, err := ValidateConfigFile(siftToml, root)
	if err != nil {
		t.Fatalf("ValidateConfigFile failed: %v", err)
	}
	if !res.Valid {
		t.Fatalf(".sift.toml at repo root is reported invalid: %v", res.Issues)
	}
	if res.Summary.DefaultProfile != "default" {
		t.Errorf("expected default_profile 'default', got %q", res.Summary.DefaultProfile)
	}
}
