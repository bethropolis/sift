package tui

import (
	"testing"

	"github.com/bethropolis/sift/internal/theme"
)

func TestThemeModelAppendsUserThemes(t *testing.T) {
	user := theme.ThemePreset{ID: "custom", Name: "Custom", Border: "#123456", Title: "#abcdef", Muted: "#777777", CursorBg: "#111111", CursorFg: "#ffffff", Selected: "#abcdef", Notice: "#abcdef", ModeFull: "#abcdef", ModeSig: "#abcdef", ModeSkip: "#abcdef"}
	m := newModel(BuildTree([]Item{{Path: "a.go"}}), Options{UITheme: "custom", UserThemes: []theme.ThemePreset{user}})
	if m.themeIndex != len(m.themes)-1 || m.styles.accent != "#abcdef" {
		t.Fatalf("custom theme not applied: index=%d themes=%d accent=%q", m.themeIndex, len(m.themes), m.styles.accent)
	}
}
