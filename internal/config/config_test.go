package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/pflag"
)

func TestNewDefaults(t *testing.T) {
	c := New()
	if !c.IgnoreHidden {
		t.Error("IgnoreHidden default = false, want true")
	}
	if !c.IgnoreGit {
		t.Error("IgnoreGit default = false, want true")
	}
	if !c.SecretScan {
		t.Error("SecretScan default = false, want true")
	}
	if c.Style != "markdown" {
		t.Errorf("Style default = %q, want markdown", c.Style)
	}
	if c.OutputFile != "codebase.md" {
		t.Errorf("OutputFile default = %q, want codebase.md", c.OutputFile)
	}
}

func TestEffectiveStyle(t *testing.T) {
	tests := []struct {
		name string
		c    *Config
		want string
	}{
		{name: "style only", c: &Config{Style: "xml"}, want: "xml"},
		{name: "json overrides style", c: &Config{Style: "xml", JSONOutput: true}, want: "json"},
		{name: "json beats markdown", c: &Config{JSONOutput: true, MarkdownOutput: true}, want: "json"},
		{name: "default plain", c: &Config{}, want: "plain"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.c.EffectiveStyle(); got != tc.want {
				t.Errorf("EffectiveStyle() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestRegisterFlagsBindsValues(t *testing.T) {
	c := New()
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	RegisterFlags(c, fs)

	if err := fs.Parse([]string{"--style", "xml", "--verbose", "--budget", "1000", "--prompt", "review", "--no-nerd-fonts"}); err != nil {
		t.Fatal(err)
	}

	if c.Style != "xml" {
		t.Errorf("Style = %q, want xml", c.Style)
	}
	if !c.Verbose {
		t.Error("Verbose = false, want true")
	}
	if c.Budget != 1000 {
		t.Errorf("Budget = %d, want 1000", c.Budget)
	}
	if c.Prompt != "review" {
		t.Errorf("Prompt = %q, want review", c.Prompt)
	}
	if !c.NoNerdFonts {
		t.Error("NoNerdFonts = false, want true")
	}
}

func TestDefaultOutputFile(t *testing.T) {
	tests := []struct {
		style string
		want  string
	}{
		{"xml", "codebase.xml"},
		{"XML", "codebase.xml"},
		{"json", "codebase.json"},
		{"plain", "codebase.txt"},
		{"text", "codebase.txt"},
		{"markdown", "codebase.md"},
		{"unknown", "codebase.md"},
	}
	for _, tc := range tests {
		if got := DefaultOutputFile(tc.style); got != tc.want {
			t.Errorf("DefaultOutputFile(%q) = %q, want %q", tc.style, got, tc.want)
		}
	}
}

func TestFormatAndOutputAliases(t *testing.T) {
	c := New()
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	RegisterFlags(c, fs)

	if err := fs.Parse([]string{"-f", "xml", "-o", "custom.xml"}); err != nil {
		t.Fatal(err)
	}
	if c.Style != "xml" {
		t.Errorf("Style = %q, want xml from -f", c.Style)
	}
	if c.OutputFile != "custom.xml" {
		t.Errorf("OutputFile = %q, want custom.xml from -o", c.OutputFile)
	}
}

func TestResolveOutputFileFromStyle(t *testing.T) {
	// Style XML without explicit output flag should resolve to codebase.xml
	c := New()
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	RegisterFlags(c, fs)
	if err := fs.Parse([]string{"--style", "xml"}); err != nil {
		t.Fatal(err)
	}
	if err := ResolveConfig(c, fs); err != nil {
		t.Fatal(err)
	}
	if c.OutputFile != "codebase.xml" {
		t.Errorf("OutputFile = %q, want codebase.xml for xml style", c.OutputFile)
	}

	// Format alias
	c2 := New()
	fs2 := pflag.NewFlagSet("test", pflag.ContinueOnError)
	RegisterFlags(c2, fs2)
	if err := fs2.Parse([]string{"--format", "json"}); err != nil {
		t.Fatal(err)
	}
	if err := ResolveConfig(c2, fs2); err != nil {
		t.Fatal(err)
	}
	if c2.OutputFile != "codebase.json" {
		t.Errorf("OutputFile = %q, want codebase.json for --format json", c2.OutputFile)
	}

	// Explicit output flag is preserved
	c3 := New()
	fs3 := pflag.NewFlagSet("test", pflag.ContinueOnError)
	RegisterFlags(c3, fs3)
	if err := fs3.Parse([]string{"--style", "xml", "-o", "mycode.md"}); err != nil {
		t.Fatal(err)
	}
	if err := ResolveConfig(c3, fs3); err != nil {
		t.Fatal(err)
	}
	if c3.OutputFile != "mycode.md" {
		t.Errorf("OutputFile = %q, want mycode.md when explicitly specified", c3.OutputFile)
	}
}

func TestRegisterFlagsSmart(t *testing.T) {
	c := New()
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	RegisterFlags(c, fs)

	if err := fs.Parse([]string{"--smart", "--smart-max-tokens", "3000"}); err != nil {
		t.Fatal(err)
	}

	if !c.SmartFilter {
		t.Error("SmartFilter = false, want true")
	}
	if c.SmartMaxTokens != 3000 {
		t.Errorf("SmartMaxTokens = %d, want 3000", c.SmartMaxTokens)
	}
}

func TestSmartMaxTokensDefault(t *testing.T) {
	c := New()
	if c.SmartMaxTokens != 15000 {
		t.Errorf("SmartMaxTokens default = %d, want 15000", c.SmartMaxTokens)
	}
	if c.SmartFilter {
		t.Error("SmartFilter default = true, want false")
	}
}

func TestApplyProfileSmart(t *testing.T) {
	c := New()
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	RegisterFlags(c, fs)

	smart := true
	p := Profile{
		SmartFilter:    &smart,
		SmartMaxTokens: 9000,
	}
	p.Apply(c, fs)

	if !c.SmartFilter {
		t.Error("SmartFilter = false, want true from profile")
	}
	if c.SmartMaxTokens != 9000 {
		t.Errorf("SmartMaxTokens = %d, want 9000", c.SmartMaxTokens)
	}
}

func TestApplyProfile(t *testing.T) {
	c := New()
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	RegisterFlags(c, fs)

	hidden := false
	sec := false
	p := Profile{
		Style:        "xml",
		Budget:       5000,
		IgnoreHidden: &hidden,
		SecretScan:   &sec,
		Extensions:   []string{"go", "md"},
	}
	p.Apply(c, fs)

	if c.Style != "xml" {
		t.Errorf("Style = %q, want xml", c.Style)
	}
	if c.Budget != 5000 {
		t.Errorf("Budget = %d, want 5000", c.Budget)
	}
	if c.IgnoreHidden {
		t.Error("IgnoreHidden = true, want false from profile")
	}
	if c.SecretScan {
		t.Error("SecretScan = true, want false from profile")
	}
	if c.Extensions != "go,md" {
		t.Errorf("Extensions = %q, want go,md", c.Extensions)
	}
}

func TestApplyProfileFlagWins(t *testing.T) {
	c := New()
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	RegisterFlags(c, fs)

	if err := fs.Parse([]string{"--style", "json"}); err != nil {
		t.Fatal(err)
	}

	style := "xml"
	p := Profile{Style: style}
	p.Apply(c, fs)

	if c.Style != "json" {
		t.Errorf("Style = %q, want json (flag beats profile)", c.Style)
	}
}

func TestLoadProfileMergesLocalOverGlobal(t *testing.T) {
	// Point config file lookups at temp dirs.
	globalDir := t.TempDir()
	localDir := t.TempDir()

	oldXDG := os.Getenv("XDG_CONFIG_HOME")
	oldCWD, _ := os.Getwd()
	t.Cleanup(func() {
		os.Setenv("XDG_CONFIG_HOME", oldXDG)
		os.Chdir(oldCWD)
	})

	os.Setenv("XDG_CONFIG_HOME", globalDir)
	if err := os.Chdir(localDir); err != nil {
		t.Fatal(err)
	}

	globalPath := filepath.Join(globalDir, "sift", "config.toml")
	if err := os.MkdirAll(filepath.Dir(globalPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(globalPath, []byte(`
default_profile = "claude"

[profiles.claude]
style = "xml"
budget = 60000
`), 0o644); err != nil {
		t.Fatal(err)
	}

	localPath := filepath.Join(localDir, ".sift.toml")
	if err := os.WriteFile(localPath, []byte(`
[profiles.claude]
style = "markdown"
`), 0o644); err != nil {
		t.Fatal(err)
	}

	name, err := DefaultProfileName()
	if err != nil {
		t.Fatal(err)
	}
	if name != "claude" {
		t.Errorf("DefaultProfileName() = %q, want claude", name)
	}

	p, err := LoadProfile("claude")
	if err != nil {
		t.Fatal(err)
	}
	if p.Style != "markdown" {
		t.Errorf("Style = %q, want markdown (local overrides global)", p.Style)
	}
	if p.Budget != 60000 {
		t.Errorf("Budget = %d, want 60000 from global", p.Budget)
	}
}
