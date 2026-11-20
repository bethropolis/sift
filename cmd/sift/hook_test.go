package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func hookRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q")
	if err := os.WriteFile(filepath.Join(root, ".sift.toml"), []byte(`[automation]
git_hook = "pre-commit"
hook_targets = ["docs", "code"]
`), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestHookInstallStatusUninstall(t *testing.T) {
	root := hookRepo(t)
	out, err := runSift(t, "hook", "status", root)
	if err != nil || !strings.Contains(out, "not installed") {
		t.Fatalf("status = %q, %v", out, err)
	}
	out, err = runSift(t, "hook", "install", root)
	if err != nil || !strings.Contains(out, "Installed") {
		t.Fatalf("install = %q, %v", out, err)
	}
	hookPath := filepath.Join(root, ".git", "hooks", "pre-commit")
	data, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), hookMarker) || !strings.Contains(string(data), `--target "docs"`) {
		t.Fatalf("hook content:\n%s", data)
	}
	out, err = runSift(t, "hook", "status", root)
	if err != nil || !strings.Contains(out, "installed") {
		t.Fatalf("status = %q, %v", out, err)
	}
	if _, err := runSift(t, "hook", "uninstall", root); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(hookPath); !os.IsNotExist(err) {
		t.Fatalf("hook still exists: %v", err)
	}
}

func TestHookRefusesForeignFile(t *testing.T) {
	root := hookRepo(t)
	hookPath := filepath.Join(root, ".git", "hooks", "pre-commit")
	if err := os.WriteFile(hookPath, []byte("#!/bin/sh\necho foreign\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := runSift(t, "hook", "install", root); err == nil || !strings.Contains(err.Error(), "--force") {
		t.Fatalf("install error = %v", err)
	}
	if _, err := runSift(t, "hook", "uninstall", root); err == nil || !strings.Contains(err.Error(), "leaving it alone") {
		t.Fatalf("uninstall error = %v", err)
	}
	if _, err := runSift(t, "hook", "install", "--force", root); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(hookPath)
	if !strings.Contains(string(data), hookMarker) {
		t.Fatal("--force did not replace the foreign hook")
	}
}
