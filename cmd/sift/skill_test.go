package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSkillCommandBareShowsHelp(t *testing.T) {
	buf := new(bytes.Buffer)
	skillCmd.SetOut(buf)
	skillCmd.SetErr(buf)

	if err := skillCmd.RunE(skillCmd, nil); err != nil {
		t.Fatalf("skill bare command failed: %v", err)
	}

	out := buf.String()
	lower := strings.ToLower(out)
	if !strings.Contains(lower, "usage:") || !strings.Contains(lower, "install") || !strings.Contains(lower, "status") {
		t.Errorf("expected skill command to show help, got:\n%s", out)
	}
}

func TestSkillCommandShow(t *testing.T) {
	buf := new(bytes.Buffer)
	skillCmd.SetOut(buf)
	skillCmd.SetErr(buf)

	skillShowCmd.SetOut(buf)
	skillShowCmd.SetErr(buf)

	if err := skillShowCmd.RunE(skillShowCmd, nil); err != nil {
		t.Fatalf("skill show failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "name: sift-project-dotfile") {
		t.Errorf("skill show missing frontmatter name: %s", out[:min(len(out), 100)])
	}
}

func TestSkillCommandStatus(t *testing.T) {
	buf := new(bytes.Buffer)
	skillStatusCmd.SetOut(buf)
	skillStatusCmd.SetErr(buf)

	if err := skillStatusCmd.RunE(skillStatusCmd, nil); err != nil {
		t.Fatalf("skill status failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "OpenCode") || !strings.Contains(out, "Antigravity") {
		t.Errorf("skill status output missing expected targets:\n%s", out)
	}
}

func TestSkillCommandLocalLifecycle(t *testing.T) {
	tempDir := t.TempDir()
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tempDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origWd)

	buf := new(bytes.Buffer)
	skillInstallCmd.SetOut(buf)
	skillInstallCmd.SetErr(buf)

	skillLocalFlag = true
	defer func() { skillLocalFlag = false }()

	if err := skillInstallCmd.RunE(skillInstallCmd, nil); err != nil {
		t.Fatalf("skill install --local failed: %v", err)
	}

	expectedSkillFile := filepath.Join(tempDir, ".agents", "skills", "sift-project-dotfile", "SKILL.md")
	if _, err := os.Stat(expectedSkillFile); err != nil {
		t.Fatalf("expected installed skill file at %s: %v", expectedSkillFile, err)
	}

	// Uninstall
	buf.Reset()
	skillUninstallCmd.SetOut(buf)
	skillUninstallCmd.SetErr(buf)

	if err := skillUninstallCmd.RunE(skillUninstallCmd, nil); err != nil {
		t.Fatalf("skill uninstall --local failed: %v", err)
	}

	if _, err := os.Stat(expectedSkillFile); !os.IsNotExist(err) {
		t.Errorf("expected skill file to be deleted")
	}
}
