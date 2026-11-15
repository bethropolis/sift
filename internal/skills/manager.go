package skills

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	embedded "github.com/bethropolis/sift/skills"
)

// TargetDef defines an AI agent ecosystem and how to locate its skills directory.
type TargetDef struct {
	ID          string   // e.g. "opencode", "antigravity", "claude", "codex", "agents"
	DisplayName string   // e.g. "OpenCode", "Antigravity", "Claude Code"
	Aliases     []string // e.g. ["claude-code", "claudecode"]
	// DetectDirs returns candidate parent directories in priority order.
	// If any exists, the target is considered installed.
	DetectDirs func(home, xdgConfig string) []string
	// SkillDir returns the skills folder given the existing parent directory.
	SkillDir func(detectedParent string) string
}

// BuiltinTargets lists the supported agent platforms.
var BuiltinTargets = []TargetDef{
	{
		ID:          "opencode",
		DisplayName: "OpenCode",
		Aliases:     []string{"open-code"},
		DetectDirs: func(home, xdgConfig string) []string {
			var dirs []string
			if xdgConfig != "" {
				dirs = append(dirs, filepath.Join(xdgConfig, "opencode"))
			}
			if home != "" {
				dirs = append(dirs, filepath.Join(home, ".config", "opencode"))
			}
			return dirs
		},
		SkillDir: func(detectedParent string) string {
			return filepath.Join(detectedParent, "skills")
		},
	},
	{
		ID:          "antigravity",
		DisplayName: "Antigravity",
		Aliases:     []string{"agy", "gemini"},
		DetectDirs: func(home, xdgConfig string) []string {
			var dirs []string
			if home != "" {
				dirs = append(dirs, filepath.Join(home, ".gemini"))
			}
			if xdgConfig != "" {
				dirs = append(dirs, filepath.Join(xdgConfig, "antigravity"))
			}
			if home != "" {
				dirs = append(dirs, filepath.Join(home, ".config", "antigravity"))
			}
			return dirs
		},
		SkillDir: func(detectedParent string) string {
			// If .gemini is detected, skills live in ~/.gemini/skills
			return filepath.Join(detectedParent, "skills")
		},
	},
	{
		ID:          "claude",
		DisplayName: "Claude Code",
		Aliases:     []string{"claude-code", "claudecode"},
		DetectDirs: func(home, xdgConfig string) []string {
			var dirs []string
			if home != "" {
				dirs = append(dirs, filepath.Join(home, ".claude"))
			}
			if xdgConfig != "" {
				dirs = append(dirs, filepath.Join(xdgConfig, "claude"))
			}
			if home != "" {
				dirs = append(dirs, filepath.Join(home, ".config", "claude"))
			}
			return dirs
		},
		SkillDir: func(detectedParent string) string {
			return filepath.Join(detectedParent, "skills")
		},
	},
	{
		ID:          "codex",
		DisplayName: "Codex",
		Aliases:     []string{"openai-codex"},
		DetectDirs: func(home, xdgConfig string) []string {
			var dirs []string
			if home != "" {
				dirs = append(dirs, filepath.Join(home, ".codex"))
			}
			if xdgConfig != "" {
				dirs = append(dirs, filepath.Join(xdgConfig, "codex"))
			}
			if home != "" {
				dirs = append(dirs, filepath.Join(home, ".config", "codex"))
			}
			return dirs
		},
		SkillDir: func(detectedParent string) string {
			return filepath.Join(detectedParent, "skills")
		},
	},
	{
		ID:          "agents",
		DisplayName: "Standard Agents",
		Aliases:     []string{"standard", "agent"},
		DetectDirs: func(home, _ string) []string {
			var dirs []string
			if home != "" {
				dirs = append(dirs, filepath.Join(home, ".agents"), filepath.Join(home, ".agent"))
			}
			return dirs
		},
		SkillDir: func(detectedParent string) string {
			return filepath.Join(detectedParent, "skills")
		},
	},
}

// TargetStatus describes whether a platform was detected and the status of the skill.
type TargetStatus struct {
	Target            TargetDef
	Detected          bool
	DetectedParent    string
	SkillsDir         string
	SkillFile         string
	Installed         bool
	UpToDate          bool
	InstalledByteSize int64
}

// Manager handles skill detection, installation, and inspection.
type Manager struct {
	HomeDir   string
	XDGConfig string
	Targets   []TargetDef
}

// NewManager creates a Manager configured with the user's environment.
func NewManager() *Manager {
	home, _ := os.UserHomeDir()
	xdg := os.Getenv("XDG_CONFIG_HOME")
	return &Manager{
		HomeDir:   home,
		XDGConfig: xdg,
		Targets:   BuiltinTargets,
	}
}

// ResolveTarget finds a target definition by ID or alias.
func (m *Manager) ResolveTarget(name string) (TargetDef, bool) {
	norm := strings.ToLower(strings.TrimSpace(name))
	for _, t := range m.Targets {
		if strings.ToLower(t.ID) == norm {
			return t, true
		}
		for _, alias := range t.Aliases {
			if strings.ToLower(alias) == norm {
				return t, true
			}
		}
	}
	return TargetDef{}, false
}

// CheckTarget returns the status of a specific target for skillName.
func (m *Manager) CheckTarget(t TargetDef, skillName string) TargetStatus {
	status := TargetStatus{Target: t}
	candidates := t.DetectDirs(m.HomeDir, m.XDGConfig)

	for _, dir := range candidates {
		info, err := os.Stat(dir)
		if err == nil && info.IsDir() {
			status.Detected = true
			status.DetectedParent = dir
			break
		}
	}

	if !status.Detected {
		return status
	}

	skillsDir := t.SkillDir(status.DetectedParent)
	status.SkillsDir = skillsDir
	skillFile := filepath.Join(skillsDir, skillName, "SKILL.md")
	status.SkillFile = skillFile

	info, err := os.Stat(skillFile)
	if err == nil && !info.IsDir() {
		status.Installed = true
		status.InstalledByteSize = info.Size()

		embeddedBytes, err := embedded.ReadSkill(skillName)
		if err == nil {
			diskBytes, err := os.ReadFile(skillFile)
			if err == nil && bytes.Equal(embeddedBytes, diskBytes) {
				status.UpToDate = true
			}
		}
	}

	return status
}

// StatusAll checks all builtin targets for skillName.
func (m *Manager) StatusAll(skillName string) []TargetStatus {
	results := make([]TargetStatus, 0, len(m.Targets))
	for _, t := range m.Targets {
		results = append(results, m.CheckTarget(t, skillName))
	}
	return results
}

// CheckLocal returns the status for installing into the current project workspace.
func (m *Manager) CheckLocal(workDir, skillName string) TargetStatus {
	localTarget := TargetDef{
		ID:          "local",
		DisplayName: "Workspace (.agents)",
	}
	skillsDir := filepath.Join(workDir, ".agents", "skills")
	skillFile := filepath.Join(skillsDir, skillName, "SKILL.md")
	status := TargetStatus{
		Target:         localTarget,
		Detected:       true,
		DetectedParent: workDir,
		SkillsDir:      skillsDir,
		SkillFile:      skillFile,
	}

	info, err := os.Stat(skillFile)
	if err == nil && !info.IsDir() {
		status.Installed = true
		status.InstalledByteSize = info.Size()

		embeddedBytes, err := embedded.ReadSkill(skillName)
		if err == nil {
			diskBytes, err := os.ReadFile(skillFile)
			if err == nil && bytes.Equal(embeddedBytes, diskBytes) {
				status.UpToDate = true
			}
		}
	}

	return status
}

// InstallResult details the outcome of an install operation.
type InstallResult struct {
	TargetID    string
	DisplayName string
	Path        string
	Action      string // "created", "updated", "unchanged", "skipped"
}

// Install installs skillName into target. If force is true, it overwrites existing files.
func (m *Manager) Install(target TargetDef, skillName string, force bool) (*InstallResult, error) {
	status := m.CheckTarget(target, skillName)
	if !status.Detected {
		return nil, fmt.Errorf("agent %s is not installed (parent directory not found)", target.DisplayName)
	}

	return m.installToFile(target.ID, target.DisplayName, status.SkillFile, skillName, status.UpToDate, force)
}

// InstallLocal installs skillName into the current project (.agents/skills/<skillName>/SKILL.md).
func (m *Manager) InstallLocal(workDir, skillName string, force bool) (*InstallResult, error) {
	status := m.CheckLocal(workDir, skillName)
	return m.installToFile("local", "Workspace (.agents)", status.SkillFile, skillName, status.UpToDate, force)
}

func (m *Manager) installToFile(targetID, displayName, skillFile, skillName string, upToDate, force bool) (*InstallResult, error) {
	if upToDate && !force {
		return &InstallResult{
			TargetID:    targetID,
			DisplayName: displayName,
			Path:        skillFile,
			Action:      "unchanged",
		}, nil
	}

	content, err := embedded.ReadSkill(skillName)
	if err != nil {
		return nil, fmt.Errorf("read embedded skill %q: %w", skillName, err)
	}

	parentDir := filepath.Dir(skillFile)
	if err := os.MkdirAll(parentDir, 0o755); err != nil {
		return nil, fmt.Errorf("create directory %q: %w", parentDir, err)
	}

	action := "created"
	if _, err := os.Stat(skillFile); err == nil {
		action = "updated"
	}

	if err := os.WriteFile(skillFile, content, 0o644); err != nil {
		return nil, fmt.Errorf("write skill file %q: %w", skillFile, err)
	}

	return &InstallResult{
		TargetID:    targetID,
		DisplayName: displayName,
		Path:        skillFile,
		Action:      action,
	}, nil
}

// InstallDetected installs skillName into all detected agent platforms.
func (m *Manager) InstallDetected(skillName string, force bool) ([]InstallResult, error) {
	statuses := m.StatusAll(skillName)
	var results []InstallResult

	for _, st := range statuses {
		if !st.Detected {
			continue
		}
		res, err := m.installToFile(st.Target.ID, st.Target.DisplayName, st.SkillFile, skillName, st.UpToDate, force)
		if err != nil {
			return results, err
		}
		results = append(results, *res)
	}

	return results, nil
}

// Uninstall removes skillName from target.
func (m *Manager) Uninstall(target TargetDef, skillName string) (bool, error) {
	status := m.CheckTarget(target, skillName)
	if !status.Installed {
		return false, nil
	}

	if err := os.Remove(status.SkillFile); err != nil && !os.IsNotExist(err) {
		return false, fmt.Errorf("remove %q: %w", status.SkillFile, err)
	}

	// Try removing the skill folder if empty
	skillDir := filepath.Dir(status.SkillFile)
	_ = os.Remove(skillDir)

	return true, nil
}

// UninstallLocal removes skillName from the local workspace.
func (m *Manager) UninstallLocal(workDir, skillName string) (bool, error) {
	status := m.CheckLocal(workDir, skillName)
	if !status.Installed {
		return false, nil
	}

	if err := os.Remove(status.SkillFile); err != nil && !os.IsNotExist(err) {
		return false, fmt.Errorf("remove %q: %w", status.SkillFile, err)
	}
	skillDir := filepath.Dir(status.SkillFile)
	_ = os.Remove(skillDir)

	return true, nil
}

// UninstallAll removes skillName from every detected target where it is installed.
func (m *Manager) UninstallAll(skillName string) ([]string, error) {
	var uninstalled []string
	for _, t := range m.Targets {
		removed, err := m.Uninstall(t, skillName)
		if err != nil {
			return uninstalled, err
		}
		if removed {
			uninstalled = append(uninstalled, t.DisplayName)
		}
	}
	return uninstalled, nil
}
