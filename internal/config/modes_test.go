package config

import (
	"testing"
)

func TestModesMatcher(t *testing.T) {
	cfg := ModesConfig{
		Full: StringList{
			"**/data/**",
			"**/repo/**",
			"docs/**",
		},
		Signatures: StringList{
			"**/ui/**",
			"**/components/**",
		},
		Skip: StringList{
			"**/test/**",
			"**/*_test.go",
		},
	}

	mm := NewModesMatcher("", cfg)
	if mm == nil {
		t.Fatal("expected non-nil ModesMatcher")
	}

	tests := []struct {
		path string
		want ModeDecision
	}{
		{"pkg/data/repo.go", ModeDecisionFull},
		{"src/repo/client.ts", ModeDecisionFull},
		{"docs/api.md", ModeDecisionFull},
		{"src/ui/Button.tsx", ModeDecisionSignatures},
		{"pkg/components/card.go", ModeDecisionSignatures},
		{"pkg/data/test/helper.go", ModeDecisionSkip}, // skip beats full
		{"pkg/data/user_test.go", ModeDecisionSkip},   // skip beats full
		{"other/random.go", ModeDecisionDefault},
	}

	for _, tc := range tests {
		got := mm.Match(tc.path)
		if got != tc.want {
			t.Errorf("Match(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

func TestNilModesMatcher(t *testing.T) {
	var mm *ModesMatcher
	if got := mm.Match("foo.go"); got != ModeDecisionDefault {
		t.Errorf("nil matcher returned %v, want default", got)
	}

	empty := NewModesMatcher("", ModesConfig{})
	if empty != nil {
		t.Errorf("empty config returned %v, want nil", empty)
	}
}
