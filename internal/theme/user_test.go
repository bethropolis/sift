package theme

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadUserThemesAndInheritance(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "themes.toml")
	data := `
[[theme]]
id = "midnight"
name = "Midnight"
extends = "catppuccin-mocha"
border = "#112233"

[[theme]]
id = "midnight-soft"
name = "Midnight Soft"
extends = "midnight"
title = "#abcdef"
`
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	themes, err := LoadUserThemes(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(themes) != 2 || themes[0].ID != "midnight" || themes[1].ID != "midnight-soft" {
		t.Fatalf("loaded themes = %+v", themes)
	}
	if themes[0].Border != "#112233" || themes[0].Title != ThemePresets[0].Title {
		t.Errorf("inheritance lost fields: %+v", themes[0])
	}
	if themes[1].Border != "#112233" || themes[1].Title != "#abcdef" {
		t.Errorf("chained inheritance lost fields: %+v", themes[1])
	}
}

func TestLoadUserThemesRejectsDuplicateAndUnknownExtends(t *testing.T) {
	dir := t.TempDir()
	for name, data := range map[string]string{
		"duplicate.toml": `[[theme]]
id = "classic"
name = "Duplicate"
`,
		"unknown.toml": `[[theme]]
id = "custom"
name = "Custom"
extends = "missing"
`,
	} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadUserThemes(path); err == nil {
			t.Errorf("%s unexpectedly loaded", name)
		}
	}
}
