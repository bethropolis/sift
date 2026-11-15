package skills

import (
	"strings"
	"testing"
)

func TestReadSkill(t *testing.T) {
	data, err := ReadSkill("sift-project-dotfile")
	if err != nil {
		t.Fatalf("ReadSkill failed: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "name: sift-project-dotfile") {
		t.Errorf("skill content missing frontmatter name, got: %s", content[:min(len(content), 100)])
	}
	if !strings.Contains(content, "# Sift Project Dotfile Authoring") {
		t.Errorf("skill content missing expected title")
	}

	// Case insensitivity
	if _, err := ReadSkill("SIFT-PROJECT-DOTFILE"); err != nil {
		t.Errorf("ReadSkill should be case-insensitive: %v", err)
	}

	// Unknown skill
	if _, err := ReadSkill("nonexistent"); err == nil {
		t.Errorf("expected error for nonexistent skill")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
