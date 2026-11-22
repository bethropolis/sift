package serve

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bethropolis/sift/internal/config"
)

func diffRepo(t *testing.T, dirty bool) string {
	t.Helper()
	root := t.TempDir()
	git := func(args ...string) {
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
	git("init", "-q", "-b", "main")
	git("config", "user.email", "t@t")
	git("config", "user.name", "t")
	if err := os.WriteFile(filepath.Join(root, "a.go"), []byte("package a\n\nfunc A() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	git("add", ".")
	git("commit", "-q", "-m", "initial")
	if dirty {
		if err := os.WriteFile(filepath.Join(root, "a.go"), []byte("package a\n\nfunc A() { println(\"changed\") }\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func diffServer(t *testing.T, root string) *Server {
	t.Helper()
	engineCfg := config.New()
	engineCfg.Quiet = true
	engineCfg.SmartFilter = false
	srv, err := New(&Config{Listen: "127.0.0.1:7777", Roots: []string{root}}, engineCfg, nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return srv
}

func diffResponse(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("diff = %d: %s", rec.Code, rec.Body.String())
	}
	var out map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestDiffWorktreeAndRef(t *testing.T) {
	root := diffRepo(t, true)
	srv := diffServer(t, root)
	cookies := loginCookies(t, srv)

	out := diffResponse(t, do(srv, "POST", "/api/diff", map[string]any{"root": root}, cookies))
	doc, _ := out["document"].(string)
	if !strings.Contains(doc, "a.go") || !strings.Contains(doc, "changed") {
		t.Fatalf("worktree diff missing content:\n%s", doc)
	}
	if out["tokens"].(float64) <= 0 {
		t.Fatal("diff tokens = 0")
	}

	// Commit the change and ask for the ref range instead.
	cmd := exec.Command("git", "add", ".")
	cmd.Dir = root
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	cmd = exec.Command("git", "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "-m", "change")
	cmd.Dir = root
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	out = diffResponse(t, do(srv, "POST", "/api/diff", map[string]any{"root": root, "from": "HEAD~1"}, cookies))
	doc, _ = out["document"].(string)
	if !strings.Contains(doc, "a.go") {
		t.Fatalf("ref diff missing content:\n%s", doc)
	}

	// Clean tree, no ref: empty result, not an error.
	out = diffResponse(t, do(srv, "POST", "/api/diff", map[string]any{"root": root}, cookies))
	if out["document"] != "" || out["tokens"].(float64) != 0 {
		t.Fatalf("clean tree = %+v", out)
	}
}

func TestDiffRedactsPatchAndRejectsBadInput(t *testing.T) {
	root := diffRepo(t, false)
	if err := os.WriteFile(filepath.Join(root, "a.go"), []byte("package a\n\nconst aws = \"AKIAIOSFODNN7EXAMPLE\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	srv := diffServer(t, root)
	cookies := loginCookies(t, srv)

	out := diffResponse(t, do(srv, "POST", "/api/diff", map[string]any{"root": root, "redact": true}, cookies))
	doc, _ := out["document"].(string)
	if strings.Contains(doc, "AKIAIOSFODNN7EXAMPLE") {
		t.Fatalf("diff leaked a secret:\n%s", doc)
	}
	if !strings.Contains(doc, "REDACTED_SECRET") {
		t.Fatalf("diff patch was not redacted:\n%s", doc)
	}

	if rec := do(srv, "POST", "/api/diff", map[string]any{"root": root, "from": "--output=/tmp/x"}, cookies); rec.Code != http.StatusBadRequest {
		t.Fatalf("ref injection = %d, want 400", rec.Code)
	}
	if rec := do(srv, "POST", "/api/diff", map[string]any{"root": root, "from": "no-such-ref"}, cookies); rec.Code != http.StatusBadRequest {
		t.Fatalf("unknown ref = %d, want 400", rec.Code)
	}
	if rec := do(srv, "POST", "/api/diff", map[string]any{"root": "/etc"}, cookies); rec.Code != http.StatusForbidden {
		t.Fatalf("jail escape = %d, want 403", rec.Code)
	}

	plainRoot := t.TempDir()
	plain := diffServer(t, plainRoot)
	plainCookies := loginCookies(t, plain)
	if rec := do(plain, "POST", "/api/diff", map[string]any{"root": plainRoot}, plainCookies); rec.Code != http.StatusBadRequest {
		t.Fatalf("non-git root = %d, want 400", rec.Code)
	}
}

func TestPairDiffExcludesSensitivePatch(t *testing.T) {
	root := diffRepo(t, false)
	if err := os.WriteFile(filepath.Join(root, ".env"), []byte("AWS=AKIAIOSFODNN7EXAMPLE\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	gitAdd := exec.Command("git", "add", ".env")
	gitAdd.Dir = root
	if err := gitAdd.Run(); err != nil {
		t.Fatal(err)
	}
	gitCommit := exec.Command("git", "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "-m", "env")
	gitCommit.Dir = root
	if err := gitCommit.Run(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".env"), []byte("AWS=AKIAIOSFODNN7EXAMPLE\nEXTRA=1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "a.go"), []byte("package a\n\nfunc A() { println(\"safe change\") }\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	engineCfg := config.New()
	engineCfg.Quiet = true
	engineCfg.SmartFilter = false
	srv, err := New(&Config{
		Listen: "127.0.0.1:7777", Roots: []string{root}, Pair: true,
		PublicURL: "https://example.com", PairTTL: time.Minute, PairSessionTTL: time.Hour,
	}, engineCfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	cookies := pairCookies(t, srv)
	rec := do(srv, "POST", "/api/diff", map[string]any{"root": root}, cookies)
	if rec.Code != http.StatusOK {
		t.Fatalf("diff = %d: %s", rec.Code, rec.Body.String())
	}
	var out struct {
		Document string `json:"document"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.Document, ".env") || strings.Contains(out.Document, "AKIAIOSFODNN7EXAMPLE") || strings.Contains(out.Document, "EXTRA=1") {
		t.Fatalf("pair diff leaked a sensitive file:\n%s", out.Document)
	}
	if !strings.Contains(out.Document, "a.go") || !strings.Contains(out.Document, "safe change") {
		t.Fatalf("pair diff dropped the safe change:\n%s", out.Document)
	}
}
