package state

import (
	"os"
	"sync"
	"testing"
)

func TestPresetCRUDAndMissingFile(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	root := t.TempDir()
	if got, err := ListPresets(root); err != nil || len(got) != 0 {
		t.Fatalf("empty presets = %v, %v", got, err)
	}
	saved, err := SavePreset(root, Preset{
		Name: "review", Selections: map[string]string{"main.go": "full", "docs/x.md": "skip"},
		Prompt: "review", Style: "xml", Budget: 1000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if saved.UpdatedAt.IsZero() {
		t.Error("UpdatedAt not set")
	}
	got, err := ListPresets(root)
	if err != nil || len(got) != 1 || got[0].Name != "review" || got[0].Selections["main.go"] != "full" {
		t.Fatalf("ListPresets = %+v, %v", got, err)
	}
	if _, err := SavePreset(root, Preset{Name: "review", Selections: map[string]string{"main.go": "sigs"}}); err != nil {
		t.Fatal(err)
	}
	got, _ = ListPresets(root)
	if len(got) != 1 || got[0].Selections["main.go"] != "sigs" {
		t.Fatalf("upsert failed: %+v", got)
	}
	if err := DeletePreset(root, "review"); err != nil {
		t.Fatal(err)
	}
	if got, _ := ListPresets(root); len(got) != 0 {
		t.Fatalf("delete left %+v", got)
	}
	if err := DeletePreset(root, "missing"); err != nil {
		t.Fatalf("unknown delete: %v", err)
	}
}

func TestPresetConcurrentSaves(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	root := t.TempDir()
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			name := string(rune('a' + n))
			if _, err := SavePreset(root, Preset{Name: name, Selections: map[string]string{"a.go": "full"}}); err != nil {
				t.Errorf("SavePreset(%s): %v", name, err)
			}
		}(i)
	}
	wg.Wait()
	got, err := ListPresets(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 8 {
		t.Fatalf("presets = %d, want 8", len(got))
	}
	if _, err := os.Stat(presetsPathOrFatal(t)); err != nil {
		t.Fatal(err)
	}
}

func presetsPathOrFatal(t *testing.T) string {
	t.Helper()
	p, err := presetsPath()
	if err != nil {
		t.Fatal(err)
	}
	return p
}
