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
	if c.Style != "plain" {
		t.Errorf("Style default = %q, want plain", c.Style)
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

	if err := fs.Parse([]string{"--style", "xml", "--verbose", "--budget", "1000"}); err != nil {
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

	globalPath := filepath.Join(globalDir, "dir-dumper", "config.toml")
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

	localPath := filepath.Join(localDir, ".dirdumper.toml")
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
