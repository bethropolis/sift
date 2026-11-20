package config

import (
	"strings"
	"testing"
)

func TestExportModesCollapsesUniformDirectories(t *testing.T) {
	assignments := map[string]string{
		"src/app/a.go":       "full",
		"src/app/b.go":       "full",
		"internal/core/c.go": "sigs",
		"internal/core/d.go": "outline",
		"internal/core/e.go": "signatures",
		"docs/readme.md":     "skip",
		"main.go":            "full",
	}
	got := ExportModes(assignments)
	if len(got.Full) != 0 {
		t.Errorf("full = %v, want none (default)", got.Full)
	}
	if len(got.Signatures) != 2 || got.Signatures[0] != "internal/core/c.go" || got.Signatures[1] != "internal/core/e.go" {
		t.Errorf("signatures = %v", got.Signatures)
	}
	if len(got.Outline) != 1 || got.Outline[0] != "internal/core/d.go" {
		t.Errorf("outline = %v", got.Outline)
	}
	if len(got.Skip) != 1 || got.Skip[0] != "docs/readme.md" {
		t.Errorf("skip = %v, want docs/readme.md", got.Skip)
	}

	matcher := NewModesMatcher(t.TempDir(), got)
	for path, raw := range assignments {
		want := ModeDecisionDefault
		switch raw {
		case "sigs", "signatures":
			want = ModeDecisionSignatures
		case "outline":
			want = ModeDecisionOutline
		case "skip":
			want = ModeDecisionSkip
		}
		if decision := matcher.Match(path); decision != want {
			t.Errorf("round-trip %s = %v, want %v", path, decision, want)
		}
	}
}

func TestFormatModesTOML(t *testing.T) {
	modes := ModesConfig{
		Signatures: []string{"ui/**"},
		Skip:       []string{"vendor/**"},
	}
	text := FormatModesTOML(modes)
	want := "[modes]\nsignatures = [\n  \"ui/**\",\n]\nskip = [\n  \"vendor/**\",\n]\n"
	if text != want {
		t.Fatalf("TOML = %q, want %q", text, want)
	}
	if got := FormatModesTOML(ModesConfig{}); got != "" {
		t.Errorf("empty modes should render empty, got %q", got)
	}
	if !strings.Contains(text, "[modes]") {
		t.Error("missing section header")
	}
}
