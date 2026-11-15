package skills

import (
	"embed"
	"fmt"
	"strings"
)

// FS embeds all bundled agent skills.
//
//go:embed sift-project-dotfile/*
var FS embed.FS

// SkillInfo provides metadata for a bundled skill.
type SkillInfo struct {
	Name        string
	Description string
	Path        string // e.g. "sift-project-dotfile/SKILL.md"
}

// BundledSkills lists every skill embedded in the binary.
var BundledSkills = []SkillInfo{
	{
		Name:        "sift-project-dotfile",
		Description: "Inspect a software repository and create or revise a project-specific .sift.toml that produces useful, safe, budget-aware context dumps for coding agents. Use when asked to configure Sift for a codebase, create a Sift project dotfile, or improve an existing .sift.toml.",
		Path:        "sift-project-dotfile/SKILL.md",
	},
}

// ReadSkill reads the raw content of a bundled skill by name.
func ReadSkill(name string) ([]byte, error) {
	for _, s := range BundledSkills {
		if strings.EqualFold(s.Name, name) {
			return FS.ReadFile(s.Path)
		}
	}
	return nil, fmt.Errorf("skill %q not found", name)
}

// FindSkill returns metadata for a bundled skill by name.
func FindSkill(name string) (SkillInfo, bool) {
	for _, s := range BundledSkills {
		if strings.EqualFold(s.Name, name) {
			return s, true
		}
	}
	return SkillInfo{}, false
}
