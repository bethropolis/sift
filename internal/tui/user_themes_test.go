package tui

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

func TestThemeModelAppendsUserThemes(t *testing.T) {
	user := ThemePreset{ID: "custom", Name: "Custom", Border: "#123456", Title: "#abcdef", Muted: "#777777", CursorBg: "#111111", CursorFg: "#ffffff", Selected: "#abcdef", Notice: "#abcdef", ModeFull: "#abcdef", ModeSig: "#abcdef", ModeSkip: "#abcdef"}
	m := newModel(BuildTree([]Item{{Path: "a.go"}}), Options{UITheme: "custom", UserThemes: []ThemePreset{user}})
	if m.themeIndex != len(m.themes)-1 || m.styles.accent != "#abcdef" {
		t.Fatalf("custom theme not applied: index=%d themes=%d accent=%q", m.themeIndex, len(m.themes), m.styles.accent)
	}
}
