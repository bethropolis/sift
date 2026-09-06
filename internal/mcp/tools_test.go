package mcp

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bethropolis/sift/internal/config"
)

const mcpTestSource = `package main

// main runs the program.
func main() {
	println("hi")
}
`

// writeMCPRepo builds a temp git repo with one committed file plus one
// uncommitted edit, and points XDG_CONFIG_HOME at a temp dir so the
// statelessness assertions observe a clean slate.
func writeMCPRepo(t *testing.T) (root, configHome string) {
	t.Helper()
	root = t.TempDir()
	configHome = t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q")
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte(mcpTestSource), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "-A")
	run("commit", "-qm", "init")
	// Uncommitted edit for the default pack_diff path.
	f, err := os.OpenFile(filepath.Join(root, "main.go"), os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("// edited\n"); err != nil {
		t.Fatal(err)
	}
	f.Close()
	return root, configHome
}

func mcpTestConfig(root string) *config.Config {
	cfg := config.New()
	cfg.Version = "test"
	cfg.Quiet = true
	cfg.RootDir = root
	cfg.Budget = 10000
	return cfg
}

func TestPackContextRespectsBudget(t *testing.T) {
	root, _ := writeMCPRepo(t)
	text, err := handlePackContext(context.Background(), map[string]any{
		"budget": float64(20), "mode": "signatures",
	}, mcpTestConfig(root))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "func main()") {
		t.Errorf("pack_context output missing signature:\n%s", text)
	}
	if strings.Contains(text, `println("hi")`) {
		t.Error("pack_context leaked the function body in signatures mode")
	}
}

func TestPackContextRejectsBadMode(t *testing.T) {
	root, _ := writeMCPRepo(t)
	if _, err := handlePackContext(context.Background(), map[string]any{"mode": "lossy"}, mcpTestConfig(root)); err == nil {
		t.Error("expected error for invalid mode")
	}
}

func TestPackDiffPatchBlock(t *testing.T) {
	root, _ := writeMCPRepo(t)
	text, err := handlePackDiff(context.Background(), map[string]any{"patch": true}, mcpTestConfig(root))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "<context_update") || !strings.Contains(text, "// edited") {
		t.Errorf("pack_diff patch missing block or edit:\n%s", text)
	}
}

func TestListTreeMinimal(t *testing.T) {
	root, _ := writeMCPRepo(t)
	// Uncommitted nested file: exercises derived directory entries.
	if err := os.MkdirAll(filepath.Join(root, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "sub", "extra.go"), []byte("package sub\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	text, err := handleListTree(context.Background(), map[string]any{}, mcpTestConfig(root))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "main.go") || !strings.Contains(text, "extra.go") {
		t.Errorf("list_tree missing files:\n%s", text)
	}
	if !strings.Contains(text, "sub/") {
		t.Errorf("list_tree missing derived dir entry:\n%s", text)
	}
	if !strings.Contains(text, "2 files, 1 dirs") {
		t.Errorf("list_tree wrong count line:\n%s", text)
	}
	if strings.Contains(text, "package main") {
		t.Error("list_tree leaked file content")
	}
}

// TestHandlersAreStateless asserts the tool calls never create or modify the
// shared delta baseline: a human's next pick session must be unaffected.
func TestHandlersAreStateless(t *testing.T) {
	root, configHome := writeMCPRepo(t)
	cfg := mcpTestConfig(root)
	ctx := context.Background()
	if _, err := handlePackContext(ctx, map[string]any{"budget": float64(5000)}, cfg); err != nil {
		t.Fatal(err)
	}
	if _, err := handlePackDiff(ctx, map[string]any{"patch": true}, cfg); err != nil {
		t.Fatal(err)
	}
	if _, err := handleListTree(ctx, map[string]any{}, cfg); err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(configHome, "sift", "state.json")
	if _, err := os.Stat(statePath); !os.IsNotExist(err) {
		t.Errorf("state.json exists after tool calls: %v", err)
	}
	// No output document may be created in the scanned tree either.
	if _, err := os.Stat(filepath.Join(root, "codebase.md")); !os.IsNotExist(err) {
		t.Error("codebase.md created in scanned tree")
	}
}

// TestStdioEndToEnd pipes initialize, tools/list, and a tools/call through
// Run and asserts every stdout line is a JSON-RPC response.
func TestStdioEndToEnd(t *testing.T) {
	root, _ := writeMCPRepo(t)
	_ = root
	in := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"t","version":"1"}}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"list_tree"}}`,
	}, "\n")
	// list_tree runs against cfg.RootDir; point it at the repo.
	cfg := mcpTestConfig(root)
	var out bytes.Buffer
	if err := Run(context.Background(), strings.NewReader(in), &out, cfg); err != nil {
		t.Fatal(err)
	}
	resps := decodeResponses(t, out.String())
	if len(resps) != 3 {
		t.Fatalf("responses = %d, want 3", len(resps))
	}
	last, _ := resps[2]["result"].(map[string]any)
	content, _ := last["content"].([]any)
	if len(content) == 0 {
		t.Fatal("tools/call returned no content")
	}
	block, _ := content[0].(map[string]any)
	if !strings.Contains(block["text"].(string), "main.go") {
		t.Errorf("tools/call list_tree missing main.go: %v", block["text"])
	}
}
