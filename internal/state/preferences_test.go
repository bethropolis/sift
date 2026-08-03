package state

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPreferencesRoundTrip(t *testing.T) {
	dir := t.TempDir()
	old := os.Getenv("XDG_CONFIG_HOME")
	if err := os.Setenv("XDG_CONFIG_HOME", dir); err != nil {
		t.Fatal(err)
	}
	defer os.Setenv("XDG_CONFIG_HOME", old)

	if got, err := LoadPreferences(); err != nil {
		t.Fatal(err)
	} else if got != (Preferences{}) {
		t.Fatalf("missing preferences = %#v, want zero value", got)
	}
	want := Preferences{UITheme: "Tokyo Night"}
	if err := SavePreferences(want); err != nil {
		t.Fatal(err)
	}
	got, err := LoadPreferences()
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("loaded preferences = %#v, want %#v", got, want)
	}
	if _, err := os.Stat(filepath.Join(dir, "sift", "preferences.json")); err != nil {
		t.Fatal(err)
	}
}
