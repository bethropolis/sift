package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// writeConfig writes content to dir/name and returns the path.
func writeConfigFileAt(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestResolveAppliesUIThemeFileAndCopyOnGenerate(t *testing.T) {
	global := t.TempDir()
	project := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", global)
	local := writeConfigFileAt(t, project, ".sift.toml", "[sift]\nui_theme_file = \"themes.toml\"\ncopy_on_generate = true\n")

	cfg := New()
	cfg.RootDir = project
	if err := ResolveConfig(cfg, nil, WithLocalConfig(local)); err != nil {
		t.Fatal(err)
	}
	if cfg.UIThemeFile != "themes.toml" {
		t.Errorf("UIThemeFile = %q, want themes.toml", cfg.UIThemeFile)
	}
	if !cfg.CopyOnGenerate {
		t.Error("CopyOnGenerate not applied")
	}
}

func TestResolveProfileCopyOnGenerateAndNilFlagSet(t *testing.T) {
	global := t.TempDir()
	project := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", global)
	local := writeConfigFileAt(t, project, ".sift.toml", `default_profile = "claude"

[profiles.claude]
copy_on_generate = true
ui_theme_file = "picker.toml"
`)

	cfg := New()
	cfg.RootDir = project
	if err := ResolveConfig(cfg, nil, WithLocalConfig(local)); err != nil { // nil FlagSet must not panic
		t.Fatal(err)
	}
	if !cfg.CopyOnGenerate {
		t.Error("profile CopyOnGenerate not applied")
	}
	if cfg.UIThemeFile != "picker.toml" {
		t.Errorf("profile UIThemeFile = %q", cfg.UIThemeFile)
	}
}

func TestResolveNormalizesStyleAndModeAliases(t *testing.T) {
	global := t.TempDir()
	project := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", global)
	local := writeConfigFileAt(t, project, ".sift.toml", "[sift]\nstyle = \"md\"\ncompress_mode = \"sigs\"\n")

	cfg := New()
	cfg.RootDir = project
	if err := ResolveConfig(cfg, nil, WithLocalConfig(local)); err != nil {
		t.Fatal(err)
	}
	if cfg.Style != "markdown" {
		t.Errorf("Style = %q, want markdown", cfg.Style)
	}
	if cfg.Mode != "signatures" {
		t.Errorf("Mode = %q, want signatures", cfg.Mode)
	}
}

func TestValidateConfigFileChecksTheGivenFile(t *testing.T) {
	global := t.TempDir()
	project := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", global)
	// A clean root config must not mask an escape declared in the custom file.
	writeConfigFileAt(t, project, ".sift.toml", "[sift]\nstyle = \"markdown\"\n")
	custom := writeConfigFileAt(t, project, "custom.toml", "[sift]\noutput = \"../outside.md\"\n")

	result, err := ValidateConfigFile(custom, project)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, issue := range result.Issues {
		if strings.Contains(issue.Message, "escapes the project root") {
			found = true
		}
	}
	if !found {
		t.Fatalf("escape in custom.toml not reported: %+v", result.Issues)
	}
}

func TestResolveMissingPromptFileErrors(t *testing.T) {
	global := t.TempDir()
	project := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", global)
	local := writeConfigFileAt(t, project, ".sift.toml", "[sift]\nprompt_file = \"missing.md\"\n")

	cfg := New()
	cfg.RootDir = project
	err := ResolveConfig(cfg, nil, WithLocalConfig(local))
	if err == nil || !strings.Contains(err.Error(), "prompt file") {
		t.Fatalf("ResolveConfig error = %v, want missing prompt file", err)
	}
}

func TestLocalConfigDoesNotLeakFromCwd(t *testing.T) {
	global := t.TempDir()
	cwd := t.TempDir()
	project := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", global)
	writeConfigFileAt(t, cwd, ".sift.toml", "[sift]\nstyle = \"xml\"\n")

	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(cwd); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldWD)

	cfg := New()
	cfg.RootDir = project // no .sift.toml here
	if err := ResolveConfig(cfg, nil); err != nil {
		t.Fatal(err)
	}
	if cfg.Style == "xml" {
		t.Error("cwd's .sift.toml leaked into an unrelated project root")
	}
}

func TestLocalTopLevelModesBeatGlobalSiftModes(t *testing.T) {
	global := t.TempDir()
	project := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", global)
	writeConfigFileAt(t, filepath.Join(global, "sift"), "config.toml", "[sift.modes]\nfull = [\"global/**\"]\n")
	local := writeConfigFileAt(t, project, ".sift.toml", "[modes]\nfull = [\"local/**\"]\n")

	cfg := New()
	cfg.RootDir = project
	if err := ResolveConfig(cfg, nil, WithLocalConfig(local)); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual([]string(cfg.Modes.Full), []string{"local/**"}) {
		t.Errorf("Modes.Full = %v, want [local/**]", cfg.Modes.Full)
	}
}

func TestLocalSiftModesBeatGlobalTopLevelModes(t *testing.T) {
	global := t.TempDir()
	project := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", global)
	writeConfigFileAt(t, filepath.Join(global, "sift"), "config.toml", "[modes]\nfull = [\"global/**\"]\n")
	local := writeConfigFileAt(t, project, ".sift.toml", "[sift.modes]\nfull = [\"local/**\"]\n")

	cfg := New()
	cfg.RootDir = project
	if err := ResolveConfig(cfg, nil, WithLocalConfig(local)); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual([]string(cfg.Modes.Full), []string{"local/**"}) {
		t.Errorf("Modes.Full = %v, want [local/**]", cfg.Modes.Full)
	}
}

func TestProfilePromptConflictErrors(t *testing.T) {
	p := Profile{Prompt: "inline", PromptFile: "file.md"}
	if err := p.Apply(New(), nil); err == nil {
		t.Fatal("expected prompt/prompt_file conflict error")
	}
}
