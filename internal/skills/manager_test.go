package skills

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSkillDetectionAndInstallation(t *testing.T) {
	tempHome := t.TempDir()
	tempXDG := filepath.Join(tempHome, "xdg_config")

	// Create fake OpenCode and Antigravity home dirs
	opencodeDir := filepath.Join(tempXDG, "opencode")
	geminiDir := filepath.Join(tempHome, ".gemini")
	if err := os.MkdirAll(opencodeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(geminiDir, 0o755); err != nil {
		t.Fatal(err)
	}

	mgr := &Manager{
		HomeDir:   tempHome,
		XDGConfig: tempXDG,
		Targets:   BuiltinTargets,
	}

	skillName := "sift-project-dotfile"

	// Initial status: OpenCode and Antigravity should be detected, but not installed
	statuses := mgr.StatusAll(skillName)
	detectedCount := 0
	for _, st := range statuses {
		if st.Detected {
			detectedCount++
			if st.Installed {
				t.Errorf("target %s should not be installed yet", st.Target.ID)
			}
		}
	}
	if detectedCount != 2 {
		t.Fatalf("expected 2 detected targets, got %d", detectedCount)
	}

	// Install to all detected
	results, err := mgr.InstallDetected(skillName, false)
	if err != nil {
		t.Fatalf("InstallDetected failed: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 install results, got %d", len(results))
	}
	for _, res := range results {
		if res.Action != "created" {
			t.Errorf("expected action 'created', got %s", res.Action)
		}
		if _, err := os.Stat(res.Path); err != nil {
			t.Errorf("skill file was not created: %v", err)
		}
	}

	// Second install should report "unchanged"
	results2, err := mgr.InstallDetected(skillName, false)
	if err != nil {
		t.Fatalf("InstallDetected failed: %v", err)
	}
	for _, res := range results2 {
		if res.Action != "unchanged" {
			t.Errorf("expected action 'unchanged', got %s", res.Action)
		}
	}

	// Force install should report "updated"
	results3, err := mgr.InstallDetected(skillName, true)
	if err != nil {
		t.Fatalf("force InstallDetected failed: %v", err)
	}
	for _, res := range results3 {
		if res.Action != "updated" {
			t.Errorf("expected action 'updated', got %s", res.Action)
		}
	}

	// Check status now shows installed and up to date
	statusesAfter := mgr.StatusAll(skillName)
	for _, st := range statusesAfter {
		if st.Detected {
			if !st.Installed || !st.UpToDate {
				t.Errorf("target %s should be installed and up to date", st.Target.ID)
			}
		}
	}

	// Uninstall all
	uninstalled, err := mgr.UninstallAll(skillName)
	if err != nil {
		t.Fatalf("UninstallAll failed: %v", err)
	}
	if len(uninstalled) != 2 {
		t.Errorf("expected 2 uninstalled targets, got %d", len(uninstalled))
	}

	// After uninstall, should not be installed
	statusesEnd := mgr.StatusAll(skillName)
	for _, st := range statusesEnd {
		if st.Installed {
			t.Errorf("target %s should not be installed after uninstall", st.Target.ID)
		}
	}
}

func TestLocalWorkspaceInstallation(t *testing.T) {
	workDir := t.TempDir()
	mgr := NewManager()
	skillName := "sift-project-dotfile"

	res, err := mgr.InstallLocal(workDir, skillName, false)
	if err != nil {
		t.Fatalf("InstallLocal failed: %v", err)
	}
	if res.Action != "created" {
		t.Errorf("expected 'created', got %s", res.Action)
	}

	expectedPath := filepath.Join(workDir, ".agents", "skills", skillName, "SKILL.md")
	if _, err := os.Stat(expectedPath); err != nil {
		t.Errorf("expected skill at %s, got error: %v", expectedPath, err)
	}

	status := mgr.CheckLocal(workDir, skillName)
	if !status.Installed || !status.UpToDate {
		t.Errorf("expected local status installed and up to date")
	}

	removed, err := mgr.UninstallLocal(workDir, skillName)
	if err != nil || !removed {
		t.Fatalf("UninstallLocal failed: removed=%v, err=%v", removed, err)
	}

	if _, err := os.Stat(expectedPath); !os.IsNotExist(err) {
		t.Errorf("expected file to be deleted")
	}
}
