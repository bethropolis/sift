package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeStatsRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write := func(name, body string) {
		t.Helper()
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("main.go", "package main\n\nfunc main() {}\n")
	write("web/app.ts", "export const app = 1;\n")
	write("README.md", "# demo\n")
	return root
}

func runSift(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := newRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

func TestStatsHumanAndJSON(t *testing.T) {
	root := writeStatsRepo(t)
	text, err := runSift(t, "stats", root)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"files", "Directories", "web", "Languages", "Heaviest files", "main.go"} {
		if !strings.Contains(text, want) {
			t.Errorf("stats output missing %q:\n%s", want, text)
		}
	}

	text, err = runSift(t, "stats", "--json", root)
	if err != nil {
		t.Fatal(err)
	}
	var report statsReport
	if err := json.Unmarshal([]byte(text), &report); err != nil {
		t.Fatalf("stats --json is invalid: %v\n%s", err, text)
	}
	if report.Files == 0 || report.Tokens == 0 {
		t.Fatalf("empty stats report: %+v", report)
	}
}

func TestInitGeneratesValidConfig(t *testing.T) {
	root := writeStatsRepo(t)
	text, err := runSift(t, "init", "--stdout", root)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "[sift]") || !strings.Contains(text, "ignore = [") {
		t.Fatalf("init output missing sections:\n%s", text)
	}
	if !strings.Contains(text, "prompt_file = \"README.md\"") {
		t.Fatalf("init did not add a guidance target:\n%s", text)
	}
	if _, err := runSift(t, "init", root); err != nil {
		t.Fatal(err)
	}
	if _, err := runSift(t, "init", root); err == nil {
		t.Fatal("init should refuse to overwrite without --force")
	}
	if _, err := runSift(t, "init", "--force", root); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, ".sift.toml")); err != nil {
		t.Fatal(err)
	}
}
