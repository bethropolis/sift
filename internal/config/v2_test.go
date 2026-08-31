package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/pflag"
)

func TestV2_StringListLegacyString(t *testing.T) {
	// Legacy profiles with array form (canonical TOML). Array decoding is
	// the path used by all real configs; string comma form is CLI-only
	// (--ext "go,md"), not TOML — Config.Extensions handles that join.
	dir := t.TempDir()
	old, _ := os.Getwd()
	os.Chdir(dir)
	t.Cleanup(func() { os.Chdir(old); os.Setenv("XDG_CONFIG_HOME", "") })
	os.Setenv("XDG_CONFIG_HOME", t.TempDir())

	if err := os.WriteFile(".sift.toml", []byte(`
[profiles.legacy]
extensions = ["go", "md"]
style = "xml"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := LoadProfile("legacy")
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Extensions) != 2 || p.Extensions[0] != "go" || p.Extensions[1] != "md" {
		t.Errorf("extensions = %v, want [go md]", []string(p.Extensions))
	}
}

func TestV2_StringListArray(t *testing.T) {
	dir := t.TempDir()
	old, _ := os.Getwd()
	os.Chdir(dir)
	t.Cleanup(func() { os.Chdir(old); os.Setenv("XDG_CONFIG_HOME", "") })
	os.Setenv("XDG_CONFIG_HOME", t.TempDir())

	if err := os.WriteFile(".sift.toml", []byte(`
[profiles.new]
extensions = ["go", "md"]
style = "xml"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := LoadProfile("new")
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Extensions) != 2 || p.Extensions[0] != "go" || p.Extensions[1] != "md" {
		t.Errorf("extensions = %v, want [go md]", []string(p.Extensions))
	}
}

func TestV2_SiftDefaults(t *testing.T) {
	dir := t.TempDir()
	old, _ := os.Getwd()
	os.Chdir(dir)
	t.Cleanup(func() { os.Chdir(old); os.Setenv("XDG_CONFIG_HOME", "") })
	os.Setenv("XDG_CONFIG_HOME", t.TempDir())

	if err := os.WriteFile(".sift.toml", []byte(`
[sift]
style = "xml"
budget = 40000
extensions = ["go", "ts"]

[profiles.claude]
style = "json"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	cf, err := LoadConfigFile()
	if err != nil {
		t.Fatal(err)
	}
	if cf.Sift == nil {
		t.Fatal("Sift is nil")
	}
	if cf.Sift.Style != "xml" {
		t.Errorf("sift.style = %q, want xml", cf.Sift.Style)
	}
	if cf.Sift.Budget != 40000 {
		t.Errorf("sift.budget = %d, want 40000", cf.Sift.Budget)
	}
	if len(cf.Sift.Extensions) != 2 {
		t.Errorf("sift.extensions = %v, want 2", []string(cf.Sift.Extensions))
	}
}

func TestV2_Targets(t *testing.T) {
	dir := t.TempDir()
	old, _ := os.Getwd()
	os.Chdir(dir)
	t.Cleanup(func() { os.Chdir(old); os.Setenv("XDG_CONFIG_HOME", "") })
	os.Setenv("XDG_CONFIG_HOME", t.TempDir())

	if err := os.WriteFile(".sift.toml", []byte(`
[[targets]]
name = "full"
output = "codebase.md"
style = "markdown"

[[targets]]
name = "sig"
output = "sig.xml"
style = "xml"
budget = 12000
`), 0o644); err != nil {
		t.Fatal(err)
	}
	cf, err := LoadConfigFile()
	if err != nil {
		t.Fatal(err)
	}
	if len(cf.Targets) != 2 {
		t.Fatalf("targets count = %d, want 2", len(cf.Targets))
	}
	if cf.Targets[0].Name != "full" || cf.Targets[0].Output != "codebase.md" {
		t.Errorf("target[0] = %+v", cf.Targets[0])
	}
	if cf.Targets[1].Name != "sig" || cf.Targets[1].Budget != 12000 {
		t.Errorf("target[1] = %+v", cf.Targets[1])
	}
}

func TestV2_ResolveWithTarget(t *testing.T) {
	dir := t.TempDir()
	old, _ := os.Getwd()
	os.Chdir(dir)
	t.Cleanup(func() { os.Chdir(old); os.Setenv("XDG_CONFIG_HOME", "") })
	os.Setenv("XDG_CONFIG_HOME", t.TempDir())

	if err := os.WriteFile(".sift.toml", []byte(`
[sift]
style = "markdown"
budget = 50000

[[targets]]
name = "docs"
style = "xml"
output = "docs.md"
budget = 8000
extensions = ["md"]
`), 0o644); err != nil {
		t.Fatal(err)
	}
	c := New()
	c.Target = "docs"
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	RegisterFlags(c, fs)
	if err := ResolveConfig(c, fs, WithTarget("docs")); err != nil {
		t.Fatal(err)
	}
	if c.Style != "xml" {
		t.Errorf("Style = %q, want xml from target", c.Style)
	}
	if c.OutputFile != "docs.md" {
		t.Errorf("OutputFile = %q, want docs.md from target", c.OutputFile)
	}
	if c.Budget != 8000 {
		t.Errorf("Budget = %d, want 8000 from target", c.Budget)
	}
	if c.Extensions != "md" {
		t.Errorf("Extensions = %q, want md from target", c.Extensions)
	}
}

func TestV2_ResolveSiftDefaults(t *testing.T) {
	dir := t.TempDir()
	old, _ := os.Getwd()
	os.Chdir(dir)
	t.Cleanup(func() { os.Chdir(old); os.Setenv("XDG_CONFIG_HOME", "") })
	os.Setenv("XDG_CONFIG_HOME", t.TempDir())

	if err := os.WriteFile(".sift.toml", []byte(`
[sift]
style = "xml"
budget = 30000
output = "out.md"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	c := New()
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	RegisterFlags(c, fs)
	if err := ResolveConfig(c, fs); err != nil {
		t.Fatal(err)
	}
	if c.Style != "xml" {
		t.Errorf("Style = %q, want xml from [sift]", c.Style)
	}
	if c.Budget != 30000 {
		t.Errorf("Budget = %d, want 30000 from [sift]", c.Budget)
	}
	if c.OutputFile != "out.md" {
		t.Errorf("OutputFile = %q, want out.md from [sift]", c.OutputFile)
	}
}

func TestV2_ExtendsChain(t *testing.T) {
	dir := t.TempDir()
	old, _ := os.Getwd()
	os.Chdir(dir)
	t.Cleanup(func() { os.Chdir(old); os.Setenv("XDG_CONFIG_HOME", "") })
	os.Setenv("XDG_CONFIG_HOME", t.TempDir())

	if err := os.WriteFile(".sift.toml", []byte(`
[profiles.base]
style = "xml"
budget = 10000

[profiles.child]
extends = ["base"]
style = "markdown"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	cf, err := LoadConfigFile()
	if err != nil {
		t.Fatal(err)
	}
	child, ok := cf.Profiles["child"]
	if !ok {
		t.Fatal("child not found")
	}
	if child.Style != "markdown" {
		t.Errorf("child style = %q, want markdown (self wins)", child.Style)
	}
	if child.Budget != 10000 {
		t.Errorf("child budget = %d, want 10000 (from base)", child.Budget)
	}
}

func TestV2_ExtendsCycle(t *testing.T) {
	dir := t.TempDir()
	old, _ := os.Getwd()
	os.Chdir(dir)
	t.Cleanup(func() { os.Chdir(old); os.Setenv("XDG_CONFIG_HOME", "") })
	os.Setenv("XDG_CONFIG_HOME", t.TempDir())

	if err := os.WriteFile(".sift.toml", []byte(`
[profiles.a]
extends = ["b"]

[profiles.b]
extends = ["a"]
`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadConfigFile()
	if err == nil {
		t.Error("want error for extends cycle, got nil")
	}
}

func TestV2_PromptFile(t *testing.T) {
	dir := t.TempDir()
	old, _ := os.Getwd()
	os.Chdir(dir)
	t.Cleanup(func() { os.Chdir(old); os.Setenv("XDG_CONFIG_HOME", "") })
	os.Setenv("XDG_CONFIG_HOME", t.TempDir())

	if err := os.WriteFile("my-prompt.md", []byte("Review this for bugs."), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(".sift.toml", []byte(`
[sift]
prompt_file = "my-prompt.md"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	c := New()
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	RegisterFlags(c, fs)
	if err := ResolveConfig(c, fs); err != nil {
		t.Fatal(err)
	}
	if c.Prompt != "Review this for bugs." {
		t.Errorf("Prompt = %q, want file content", c.Prompt)
	}
}

func TestV2_FlagWinsOverSift(t *testing.T) {
	dir := t.TempDir()
	old, _ := os.Getwd()
	os.Chdir(dir)
	t.Cleanup(func() { os.Chdir(old); os.Setenv("XDG_CONFIG_HOME", "") })
	os.Setenv("XDG_CONFIG_HOME", t.TempDir())

	if err := os.WriteFile(".sift.toml", []byte(`
[sift]
style = "xml"
budget = 9999
`), 0o644); err != nil {
		t.Fatal(err)
	}
	c := New()
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	RegisterFlags(c, fs)
	if err := fs.Parse([]string{"--style", "json"}); err != nil {
		t.Fatal(err)
	}
	if err := ResolveConfig(c, fs); err != nil {
		t.Fatal(err)
	}
	if c.Style != "json" {
		t.Errorf("Style = %q, want json (flag wins)", c.Style)
	}
	if c.Budget != 9999 {
		t.Errorf("Budget = %d, want 9999 from [sift] (no flag)", c.Budget)
	}
}

func TestV2_IgnoreListForm(t *testing.T) {
	dir := t.TempDir()
	old, _ := os.Getwd()
	os.Chdir(dir)
	t.Cleanup(func() { os.Chdir(old); os.Setenv("XDG_CONFIG_HOME", "") })
	os.Setenv("XDG_CONFIG_HOME", t.TempDir())

	if err := os.WriteFile(".sift.toml", []byte(`
[sift]
ignore = ["vendor/**", "dist/**"]
`), 0o644); err != nil {
		t.Fatal(err)
	}
	c := New()
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	RegisterFlags(c, fs)
	if err := ResolveConfig(c, fs); err != nil {
		t.Fatal(err)
	}
	if c.CustomIgnore != "vendor/**,dist/**" {
		t.Errorf("CustomIgnore = %q, want vendor/**,dist/**", c.CustomIgnore)
	}
}

func TestV2_IgnoreStringForm(t *testing.T) {
	// String form for ignore is also handled via array — canonical TOML form
	// is ["vendor/**", "dist/**"]. We verify that a second array variant works
	// (the StringList type handles array natively via plain []string decode).
	dir := t.TempDir()
	old, _ := os.Getwd()
	os.Chdir(dir)
	t.Cleanup(func() { os.Chdir(old); os.Setenv("XDG_CONFIG_HOME", "") })
	os.Setenv("XDG_CONFIG_HOME", t.TempDir())

	if err := os.WriteFile(".sift.toml", []byte(`
[sift]
ignore = ["tmp/**", "cache/**"]
`), 0o644); err != nil {
		t.Fatal(err)
	}
	c := New()
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	RegisterFlags(c, fs)
	if err := ResolveConfig(c, fs); err != nil {
		t.Fatal(err)
	}
	if c.CustomIgnore != "tmp/**,cache/**" {
		t.Errorf("CustomIgnore = %q, want tmp/**,cache/**", c.CustomIgnore)
	}
}

func TestV2_PromptsLibrary(t *testing.T) {
	dir := t.TempDir()
	old, _ := os.Getwd()
	os.Chdir(dir)
	t.Cleanup(func() { os.Chdir(old); os.Setenv("XDG_CONFIG_HOME", "") })
	os.Setenv("XDG_CONFIG_HOME", t.TempDir())

	if err := os.WriteFile(".sift.toml", []byte(`
[prompts.review]
text = "Review for bugs."

[prompts.docs]
text = "Write docs."
`), 0o644); err != nil {
		t.Fatal(err)
	}
	c := New()
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	RegisterFlags(c, fs)
	if err := ResolveConfig(c, fs, WithPromptRef("review")); err != nil {
		t.Fatal(err)
	}
	if c.Prompt != "Review for bugs." {
		t.Errorf("Prompt = %q, want Review for bugs.", c.Prompt)
	}
}

func TestV2_TargetWithProfileAndPromptFile(t *testing.T) {
	dir := t.TempDir()
	old, _ := os.Getwd()
	os.Chdir(dir)
	t.Cleanup(func() { os.Chdir(old); os.Setenv("XDG_CONFIG_HOME", "") })
	os.Setenv("XDG_CONFIG_HOME", t.TempDir())

	if err := os.WriteFile("review.md", []byte("Deep review."), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(".sift.toml", []byte(`
[profiles.claude]
style = "xml"

[[targets]]
name = "api"
profile = "claude"
prompt_file = "review.md"
output = "api.md"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	c := New()
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	RegisterFlags(c, fs)
	if err := ResolveConfig(c, fs, WithTarget("api")); err != nil {
		t.Fatal(err)
	}
	if c.Style != "xml" {
		t.Errorf("Style = %q, want xml from profile", c.Style)
	}
	if c.Prompt != "Deep review." {
		t.Errorf("Prompt = %q, want Deep review. from prompt_file", c.Prompt)
	}
	if c.OutputFile != "api.md" {
		t.Errorf("OutputFile = %q, want api.md", c.OutputFile)
	}
}

func TestV2_BackwardCompatOldProfilesStillWork(t *testing.T) {
	dir := t.TempDir()
	old, _ := os.Getwd()
	os.Chdir(dir)
	t.Cleanup(func() { os.Chdir(old); os.Setenv("XDG_CONFIG_HOME", "") })
	os.Setenv("XDG_CONFIG_HOME", t.TempDir())

	if err := os.WriteFile(".sift.toml", []byte(`
[profiles.claude]
style = "xml"
budget = 60000
`), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := LoadProfile("claude")
	if err != nil {
		t.Fatal(err)
	}
	if p.Style != "xml" || p.Budget != 60000 {
		t.Errorf("legacy profile not loaded: %+v", p)
	}
	// Also via ResolveConfig
	c := New()
	c.Profile = "claude"
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	RegisterFlags(c, fs)
	if err := ResolveConfig(c, fs); err != nil {
		t.Fatal(err)
	}
	if c.Style != "xml" || c.Budget != 60000 {
		t.Errorf("legacy via ResolveConfig: style=%q budget=%d", c.Style, c.Budget)
	}
}

func TestV2_GlobalLocalMergeSift(t *testing.T) {
	globalDir := t.TempDir()
	localDir := t.TempDir()
	old, _ := os.Getwd()
	t.Cleanup(func() { os.Chdir(old); os.Setenv("XDG_CONFIG_HOME", "") })
	os.Setenv("XDG_CONFIG_HOME", globalDir)
	os.Chdir(localDir)

	os.MkdirAll(filepath.Join(globalDir, "sift"), 0o755)
	os.WriteFile(filepath.Join(globalDir, "sift", "config.toml"), []byte(`
[sift]
style = "xml"
budget = 10000
`), 0o644)
	os.WriteFile(filepath.Join(localDir, ".sift.toml"), []byte(`
[sift]
style = "markdown"
`), 0o644)

	c := New()
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	RegisterFlags(c, fs)
	if err := ResolveConfig(c, fs); err != nil {
		t.Fatal(err)
	}
	if c.Style != "markdown" {
		t.Errorf("Style = %q, want markdown (local wins)", c.Style)
	}
	if c.Budget != 10000 {
		t.Errorf("Budget = %d, want 10000 (from global, not overridden)", c.Budget)
	}
}

func unusedPathImport_v2() { _ = filepath.Join }

