package serve

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func followFixture(t *testing.T, srv *Server, root string) {
	t.Helper()
	for name, content := range map[string]string{
		"go.mod":     "module example.com/srvtest\n\ngo 1.26\n",
		"main.go":    "package main\n\nimport \"example.com/srvtest/sub\"\n\nfunc main() { sub.Hi() }\n",
		"sub/sub.go": "package sub\n\nfunc Hi() {}\n",
	} {
		full := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestFollowHappyPath(t *testing.T) {
	srv, root := testServer(t)
	followFixture(t, srv, root)
	cookies := loginCookies(t, srv)

	rec := do(srv, "POST", "/api/follow", map[string]any{
		"root": root, "path": "sub/sub.go", "direction": "dependents",
	}, cookies)
	if rec.Code != 200 {
		t.Fatalf("follow = %d, want 200", rec.Code)
	}
	var res struct {
		Seed       string `json:"seed"`
		Direction  string `json:"direction"`
		Unanalyzed int    `json:"unanalyzed"`
		Hits       []struct {
			Path     string `json:"path"`
			Distance int    `json:"distance"`
			Via      string `json:"via"`
			Mode     string `json:"mode"`
		} `json:"hits"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&res); err != nil {
		t.Fatal(err)
	}
	if res.Seed != "sub/sub.go" || res.Direction != "dependents" {
		t.Fatalf("seed/direction = %q/%q", res.Seed, res.Direction)
	}
	byPath := map[string]int{}
	for _, h := range res.Hits {
		byPath[h.Path] = h.Distance
	}
	if byPath["sub/sub.go"] != 0 {
		t.Fatalf("seed missing or wrong distance: %v", res.Hits)
	}
	if byPath["main.go"] != 1 {
		t.Fatalf("main.go should be a hop-1 dependent: %v", res.Hits)
	}
	for _, h := range res.Hits {
		if h.Path == "main.go" && (h.Via != "sub/sub.go" || h.Mode != "full") {
			t.Fatalf("main.go hit = %+v, want via seed in full", h)
		}
	}
}

func TestFollowErrors(t *testing.T) {
	srv, root := testServer(t)
	followFixture(t, srv, root)
	cookies := loginCookies(t, srv)

	cases := map[string]map[string]any{
		"unsupported type": {"root": root, "path": "README.md"},
		"missing file":     {"root": root, "path": "nope.go"},
		"bad direction":    {"root": root, "path": "sub/sub.go", "direction": "up"},
		"outside root":     {"root": root, "path": "../escape.go"},
	}
	for name, body := range cases {
		if rec := do(srv, "POST", "/api/follow", body, cookies); rec.Code != 400 {
			t.Errorf("%s = %d, want 400", name, rec.Code)
		}
	}
	if rec := do(srv, "POST", "/api/follow", map[string]any{"root": "/nonexistent", "path": "x.go"}, cookies); rec.Code != 403 {
		t.Errorf("foreign root = %d, want 403", rec.Code)
	}
	if rec := do(srv, "POST", "/api/follow", map[string]any{"root": root, "path": "sub/sub.go"}, nil); rec.Code != 401 {
		t.Errorf("no session = %d, want 401", rec.Code)
	}
}

func TestTreeMarksFollowable(t *testing.T) {
	srv, root := testServer(t)
	followFixture(t, srv, root)
	cookies := loginCookies(t, srv)

	rec := do(srv, "GET", "/api/tree?root="+root, nil, cookies)
	if rec.Code != 200 {
		t.Fatalf("tree = %d", rec.Code)
	}
	var res struct {
		Files []struct {
			Path       string `json:"path"`
			Followable bool   `json:"followable"`
		} `json:"files"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&res); err != nil {
		t.Fatal(err)
	}
	byPath := map[string]bool{}
	for _, f := range res.Files {
		byPath[f.Path] = f.Followable
	}
	if !byPath["sub/sub.go"] || !byPath["main.go"] {
		t.Errorf("go files should be followable: %v", byPath)
	}
	if byPath["README.md"] {
		t.Error("README.md should not be followable")
	}
}
