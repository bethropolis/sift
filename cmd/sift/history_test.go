package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bethropolis/sift/internal/history"
)

func writeHistoryRepo(t *testing.T, toml string) (root, dataHome, cacheHome string) {
	t.Helper()
	root = t.TempDir()
	dataHome = t.TempDir()
	cacheHome = t.TempDir()
	t.Setenv("XDG_DATA_HOME", dataHome)
	t.Setenv("XDG_CACHE_HOME", cacheHome)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n\nfunc main() { println(\"hi\") }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if toml != "" {
		if err := os.WriteFile(filepath.Join(root, ".sift.toml"), []byte(toml), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root, dataHome, cacheHome
}

func readHistoryEntries(t *testing.T, dataHome string) []history.Entry {
	t.Helper()
	path := filepath.Join(dataHome, "sift", history.HistoryFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatal(err)
	}
	entries := make([]history.Entry, 0)
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line == "" {
			continue
		}
		var entry history.Entry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatalf("decode history line %q: %v", line, err)
		}
		entries = append(entries, entry)
	}
	return entries
}

func runSiftInput(t *testing.T, input string, args ...string) (string, error) {
	t.Helper()
	cmd := newRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetIn(strings.NewReader(input))
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	full := append([]string{"-c", "core.hooksPath=/dev/null"}, args...)
	cmd := exec.Command("git", full...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

func TestDumpRecordsHistoryAndHistoryCommands(t *testing.T) {
	root, dataHome, cacheHome := writeHistoryRepo(t, "[history]\nenabled = true\nkeep = 5\n")
	if _, err := runSift(t, "dump", "--quiet", root); err != nil {
		t.Fatal(err)
	}

	entries := readHistoryEntries(t, dataHome)
	if len(entries) != 1 {
		t.Fatalf("history entries = %d, want 1", len(entries))
	}
	entry := entries[0]
	if entry.Cmd != "dump" || entry.Name != filepath.Base(root) || entry.Style != "markdown" {
		t.Fatalf("unexpected entry: %+v", entry)
	}
	if entry.Root != root {
		t.Errorf("root = %q, want %q", entry.Root, root)
	}
	if want := filepath.Join(root, "codebase.md"); entry.Output != want {
		t.Errorf("output = %q, want %q", entry.Output, want)
	}
	if entry.Bytes == 0 || entry.Tokens == 0 || entry.Files != 1 {
		t.Errorf("unexpected metrics: %+v", entry)
	}
	output, err := os.ReadFile(filepath.Join(root, "codebase.md"))
	if err != nil {
		t.Fatal(err)
	}
	archive, err := os.ReadFile(entry.File)
	if err != nil {
		t.Fatal(err)
	}
	if entry.Bytes != int64(len(output)) || !bytes.Equal(archive, output) {
		t.Errorf("archive does not match delivered bytes (entry %d, output %d, archive %d)", entry.Bytes, len(output), len(archive))
	}
	if !strings.HasPrefix(entry.File, filepath.Join(cacheHome, "sift", "dumps")+string(filepath.Separator)) {
		t.Errorf("archive %q is not under the cache dumps directory", entry.File)
	}

	text, err := runSift(t, "history")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"WHEN", "CMD", "TOKENS", "SIZE", "dump", filepath.Base(root)} {
		if !strings.Contains(text, want) {
			t.Errorf("history table missing %q:\n%s", want, text)
		}
	}

	text, err = runSift(t, "history", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var envelope struct {
		V       int             `json:"v"`
		Entries []history.Entry `json:"entries"`
	}
	if err := json.Unmarshal([]byte(text), &envelope); err != nil {
		t.Fatalf("history --json: %v\n%s", err, text)
	}
	if envelope.V != 1 || len(envelope.Entries) != 1 || envelope.Entries[0].ID != entry.ID {
		t.Fatalf("unexpected JSON envelope: %+v", envelope)
	}
	if envelope.Entries[0].File != entry.File || envelope.Entries[0].Output != entry.Output {
		t.Fatalf("JSON paths = file %q output %q, want file %q output %q", envelope.Entries[0].File, envelope.Entries[0].Output, entry.File, entry.Output)
	}

	// The root --json flag should work before the subcommand too.
	text, err = runSift(t, "--json", "history")
	if err != nil {
		t.Fatal(err)
	}
	var rootEnvelope struct {
		V       int             `json:"v"`
		Entries []history.Entry `json:"entries"`
	}
	if err := json.Unmarshal([]byte(text), &rootEnvelope); err != nil {
		t.Fatalf("sift --json history: %v\n%s", err, text)
	}
	if rootEnvelope.V != 1 || len(rootEnvelope.Entries) != 1 || rootEnvelope.Entries[0].ID != entry.ID {
		t.Fatalf("unexpected root JSON envelope: %+v", rootEnvelope)
	}
	if rootEnvelope.Entries[0].Output != entry.Output {
		t.Fatalf("root JSON output = %q, want %q", rootEnvelope.Entries[0].Output, entry.Output)
	}

	text, err = runSift(t, "history", "--path")
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(text); got != filepath.Join(dataHome, "sift", history.HistoryFile) {
		t.Errorf("history --path = %q", got)
	}

	if _, err := runSift(t, "dump", "--quiet", "--no-history", root); err != nil {
		t.Fatal(err)
	}
	if got := len(readHistoryEntries(t, dataHome)); got != 1 {
		t.Errorf("--no-history recorded an entry; count = %d", got)
	}
}

func TestHistoryJSONEmpty(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	text, err := runSift(t, "history", "--json")
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(text); got != `{"v":1,"entries":[]}` {
		t.Fatalf("empty history JSON = %q", got)
	}
}

func TestHistoryArchiveErrorDoesNotFailDump(t *testing.T) {
	root, _, _ := writeHistoryRepo(t, "[history]\nenabled = true\n")
	notADir := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(notADir, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_DATA_HOME", notADir)

	original := os.Stderr
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = write
	_, runErr := runSift(t, "dump", "--quiet", root)
	write.Close()
	os.Stderr = original
	stderr, _ := io.ReadAll(read)
	read.Close()

	if runErr != nil {
		t.Fatalf("dump failed because history failed: %v", runErr)
	}
	if !strings.Contains(string(stderr), "sift: history:") {
		t.Fatalf("history error not reported to stderr:\n%s", stderr)
	}
	if _, err := os.Stat(filepath.Join(root, "codebase.md")); err != nil {
		t.Fatalf("successful render output missing: %v", err)
	}
}

func TestHistoryDisabledByDefault(t *testing.T) {
	root, dataHome, _ := writeHistoryRepo(t, "")
	if _, err := runSift(t, "dump", "--quiet", root); err != nil {
		t.Fatal(err)
	}
	if entries := readHistoryEntries(t, dataHome); len(entries) != 0 {
		t.Fatalf("history recorded with default config: %+v", entries)
	}
	if _, err := os.Stat(filepath.Join(dataHome, "sift", history.HistoryFile)); !os.IsNotExist(err) {
		t.Fatalf("history log created with default config: %v", err)
	}
}

func TestHistoryClearRequiresConfirmation(t *testing.T) {
	root, dataHome, _ := writeHistoryRepo(t, "[history]\nenabled = true\n")
	if _, err := runSift(t, "dump", "--quiet", root); err != nil {
		t.Fatal(err)
	}
	entries := readHistoryEntries(t, dataHome)
	if len(entries) != 1 {
		t.Fatalf("history entries = %d", len(entries))
	}

	text, err := runSiftInput(t, "n\n", "history", "clear")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "Cancelled") {
		t.Errorf("clear did not report cancellation:\n%s", text)
	}
	if _, err := os.Stat(entries[0].File); err != nil {
		t.Fatalf("archive removed after cancelled clear: %v", err)
	}

	if _, err := runSift(t, "history", "clear", "-y"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dataHome, "sift", history.HistoryFile)); !os.IsNotExist(err) {
		t.Errorf("history log still exists: %v", err)
	}
	if _, err := os.Stat(entries[0].File); !os.IsNotExist(err) {
		t.Errorf("archive still exists: %v", err)
	}
}

func TestSelectRecordsHistory(t *testing.T) {
	root, dataHome, _ := writeHistoryRepo(t, "[history]\nenabled = true\n")
	if _, err := runSift(t, "select", "--quiet", root); err != nil {
		t.Fatal(err)
	}
	entries := readHistoryEntries(t, dataHome)
	if len(entries) != 1 || entries[0].Cmd != "select" {
		t.Fatalf("history = %+v, want one select entry", entries)
	}
}

func TestDiffAndDeltaRecordHistory(t *testing.T) {
	root, dataHome, _ := writeHistoryRepo(t, "[history]\nenabled = true\n")
	runGit(t, root, "init", "-q")
	runGit(t, root, "add", "main.go")
	runGit(t, root, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-qm", "one")

	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n\nfunc main() { println(\"bye\") }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := runSift(t, "diff", "--quiet", "--dir", root); err != nil {
		t.Fatal(err)
	}
	entries := readHistoryEntries(t, dataHome)
	if len(entries) != 1 || entries[0].Cmd != "diff" {
		t.Fatalf("history after diff = %+v", entries)
	}

	runGit(t, root, "add", "main.go")
	runGit(t, root, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-qm", "two")
	if _, err := runSift(t, "delta", "--quiet", "--since", "HEAD~1", root); err != nil {
		t.Fatal(err)
	}
	entries = readHistoryEntries(t, dataHome)
	counts := map[string]int{}
	for _, entry := range entries {
		counts[entry.Cmd]++
	}
	if len(entries) != 2 || counts["diff"] != 1 || counts["delta"] != 1 {
		t.Fatalf("history after delta = %+v", entries)
	}
}
