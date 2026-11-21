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

func TestExportModesMixedFullDirectory(t *testing.T) {
	assignments := map[string]string{
		"internal/app/a.go": "full",
		"internal/app/b.go": "signatures",
		"internal/app/c.go": "sigs",
	}
	got := ExportModes(assignments)
	if len(got.Full) != 0 {
		t.Errorf("full = %v, want none (default)", got.Full)
	}
	if len(got.Signatures) != 2 || got.Signatures[0] != "internal/app/b.go" || got.Signatures[1] != "internal/app/c.go" {
		t.Fatalf("signatures = %v, want exact files (no dir collapse)", got.Signatures)
	}
	for _, pattern := range got.Signatures {
		if strings.Contains(pattern, "**") {
			t.Fatalf("mixed directory collapsed to %q", pattern)
		}
	}

	matcher := NewModesMatcher(t.TempDir(), got)
	if decision := matcher.Match("internal/app/a.go"); decision != ModeDecisionDefault {
		t.Errorf("a.go = %v, want default full", decision)
	}
	for _, path := range []string{"internal/app/b.go", "internal/app/c.go"} {
		if decision := matcher.Match(path); decision != ModeDecisionSignatures {
			t.Errorf("%s = %v, want signatures", path, decision)
		}
	}
}

func TestExportModesCollapsesUniformNonDefaultDirectory(t *testing.T) {
	got := ExportModes(map[string]string{
		"pkg/x.go": "signatures",
		"pkg/y.go": "sigs",
	})
	if len(got.Signatures) != 1 || got.Signatures[0] != "pkg/**" {
		t.Fatalf("signatures = %v, want [pkg/**]", got.Signatures)
	}
	matcher := NewModesMatcher(t.TempDir(), got)
	for _, path := range []string{"pkg/x.go", "pkg/y.go"} {
		if decision := matcher.Match(path); decision != ModeDecisionSignatures {
			t.Errorf("%s = %v, want signatures", path, decision)
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
