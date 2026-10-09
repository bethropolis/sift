package tui

import (
	"testing"

	"github.com/bethropolis/sift/internal/theme"
)

func TestSyncExternalThemeAppliesKnownID(t *testing.T) {
	m := newModel(BuildTree([]Item{{Path: "a.go"}}), Options{UITheme: "tokyo-night"})
	before := m.styles.border
	if !m.syncExternalTheme("dracula") {
		t.Fatal("known id did not apply")
	}
	idx := theme.IndexOf(m.themes, "dracula")
	if m.themeIndex != idx || m.themeCursor != idx {
		t.Fatalf("indexes = (%d, %d), want (%d, %d)", m.themeIndex, m.themeCursor, idx, idx)
	}
	if m.styles.border == before {
		t.Error("styles unchanged after external theme switch")
	}
}

func TestSyncExternalThemeIgnoresUnknownAndSystem(t *testing.T) {
	m := newModel(BuildTree([]Item{{Path: "a.go"}}), Options{UITheme: "tokyo-night"})
	active := m.themeIndex
	for _, id := range []string{"", "system", "no-such-theme", "tokyo-night"} {
		if m.syncExternalTheme(id) {
			t.Errorf("id %q reported a change", id)
		}
		if m.themeIndex != active {
			t.Errorf("id %q moved theme index to %d", id, m.themeIndex)
		}
	}
}

func TestThemePollMsgRearmsAndApplies(t *testing.T) {
	m := newModel(BuildTree([]Item{{Path: "a.go"}}), Options{UITheme: "tokyo-night"})
	updated, cmd := m.Update(themePollMsg{id: "dracula"})
	mm, ok := updated.(model)
	if !ok {
		t.Fatalf("Update returned %T", updated)
	}
	if mm.themeIndex != theme.IndexOf(mm.themes, "dracula") {
		t.Error("poll message did not apply the web theme")
	}
	if cmd == nil {
		t.Error("poll message did not re-arm the ticker")
	}
}
