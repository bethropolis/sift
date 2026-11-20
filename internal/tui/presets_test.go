package tui

import "testing"

func TestPresetSaveApplyDelete(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	rootPath := t.TempDir()
	root := BuildTree([]Item{{Path: "a.go"}, {Path: "b.go"}})
	m := newModel(root, Options{ProjectPath: rootPath})
	a := root.findChild("a.go")
	b := root.findChild("b.go")
	a.SelectState = Selected
	a.Mode = ModeSignatures
	b.SelectState = Selected
	b.Mode = ModeFull
	root.recompute()

	m.openPresets()
	if !m.presetsOpen {
		t.Fatal("preset modal did not open")
	}
	m.presetSaving = true
	m.presetInput = "review"
	m.savePreset()
	if m.presetError != "" || len(m.presets) != 1 {
		t.Fatalf("save: %v, presets=%v", m.presetError, m.presets)
	}

	// Clear the tree, then apply the preset and confirm modes return.
	a.SelectState = Unselected
	a.Mode = ModeFull
	b.SelectState = Unselected
	root.recompute()
	m.presetCursor = 0
	m.applyPreset()
	if a.SelectState != Selected || a.Mode != ModeSignatures {
		t.Fatalf("apply did not restore a.go: %+v", a)
	}
	if b.SelectState != Selected || b.Mode != ModeFull {
		t.Fatalf("apply did not restore b.go: %+v", b)
	}

	m.openPresets()
	m.deletePreset()
	if len(m.presets) != 0 {
		t.Fatalf("delete left %v", m.presets)
	}
}
