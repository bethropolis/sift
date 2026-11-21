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
	if !strings.Contains(string(data), hookMarker) || !strings.Contains(string(data), `--target 'docs'`) {
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

func TestHookScriptQuotesTargets(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	argsFile := filepath.Join(dir, "args.txt")
	fakeSift := "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$ARGS_FILE\"\n"
	if err := os.WriteFile(filepath.Join(bin, "sift"), []byte(fakeSift), 0o755); err != nil {
		t.Fatal(err)
	}
	pwned := filepath.Join(dir, "pwned")
	target := "docs; touch " + pwned + "; $(touch " + pwned + ")"
	script := hookScript("pre-commit", []string{target})
	scriptPath := filepath.Join(dir, "hook.sh")
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("sh", scriptPath)
	cmd.Env = append(os.Environ(),
		"PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"),
		"ARGS_FILE="+argsFile,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("hook failed: %v\n%s\nscript:\n%s", err, out, script)
	}
	if _, err := os.Stat(pwned); !os.IsNotExist(err) {
		t.Fatalf("target injected a shell command: %v", err)
	}
	args, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(args), target) {
		t.Fatalf("quoted target not passed literally; args:\n%s", args)
	}
}

func TestHookRejectsUnsafeTargets(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cmd := exec.Command("git", "init", "-q")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	if err := os.WriteFile(filepath.Join(root, ".sift.toml"), []byte("[automation]\ngit_hook = \"pre-commit\"\nhook_targets = [\"ok\\nbad\"]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := runSift(t, "hook", "status", root); err == nil || !strings.Contains(err.Error(), "invalid hook target") {
		t.Fatalf("status error = %v, want invalid target rejection", err)
	}
}

func TestHookHonorsCustomConfigFlag(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cmd := exec.Command("git", "init", "-q")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	custom := filepath.Join(t.TempDir(), "custom.toml")
	if err := os.WriteFile(custom, []byte("[automation]\ngit_hook = \"pre-push\"\nhook_targets = [\"api\"]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := runSift(t, "hook", "install", "--config", custom, root)
	if err != nil {
		t.Fatalf("install --config: %v\n%s", err, out)
	}
	hookPath := filepath.Join(root, ".git", "hooks", "pre-push")
	data, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatalf("custom hook not installed: %v", err)
	}
	if !strings.Contains(string(data), "--target 'api'") {
		t.Fatalf("hook did not use custom config:\n%s", data)
	}
}
