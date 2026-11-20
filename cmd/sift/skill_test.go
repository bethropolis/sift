package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func skillSub(t *testing.T, root *cobra.Command, name string) *cobra.Command {
	t.Helper()
	cmd, _, err := root.Find([]string{name})
	if err != nil || cmd == nil {
		t.Fatalf("skill subcommand %q not found: %v", name, err)
	}
	return cmd
}

func TestSkillCommandBareShowsHelp(t *testing.T) {
	root := newSkillCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)

	if err := root.RunE(root, nil); err != nil {
		t.Fatalf("skill bare command failed: %v", err)
	}

	out := buf.String()
	lower := strings.ToLower(out)
	if !strings.Contains(lower, "usage:") || !strings.Contains(lower, "install") || !strings.Contains(lower, "status") {
		t.Errorf("expected skill command to show help, got:\n%s", out)
	}
}

func TestSkillCommandShow(t *testing.T) {
	root := newSkillCmd()
	buf := new(bytes.Buffer)
	show := skillSub(t, root, "show")
	show.SetOut(buf)
	show.SetErr(buf)

	if err := show.RunE(show, nil); err != nil {
		t.Fatalf("skill show failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "name: sift-project-dotfile") {
		t.Errorf("skill show missing frontmatter name: %s", out[:min(len(out), 100)])
	}
}

func TestSkillCommandStatus(t *testing.T) {
	root := newSkillCmd()
	buf := new(bytes.Buffer)
	status := skillSub(t, root, "status")
	status.SetOut(buf)
	status.SetErr(buf)

	if err := status.RunE(status, nil); err != nil {
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

	root := newSkillCmd()
	buf := new(bytes.Buffer)
	install := skillSub(t, root, "install")
	install.SetOut(buf)
	install.SetErr(buf)
	if err := install.Flags().Set("local", "true"); err != nil {
		t.Fatal(err)
	}

	if err := install.RunE(install, nil); err != nil {
		t.Fatalf("skill install --local failed: %v", err)
	}

	expectedSkillFile := filepath.Join(tempDir, ".agents", "skills", "sift-project-dotfile", "SKILL.md")
	if _, err := os.Stat(expectedSkillFile); err != nil {
		t.Fatalf("expected installed skill file at %s: %v", expectedSkillFile, err)
	}

	// Uninstall
	buf.Reset()
	uninstall := skillSub(t, root, "uninstall")
	uninstall.SetOut(buf)
	uninstall.SetErr(buf)
	if err := uninstall.Flags().Set("local", "true"); err != nil {
		t.Fatal(err)
	}

	if err := uninstall.RunE(uninstall, nil); err != nil {
		t.Fatalf("skill uninstall --local failed: %v", err)
	}

	if _, err := os.Stat(expectedSkillFile); !os.IsNotExist(err) {
		t.Errorf("expected skill file to be deleted")
	}
}
