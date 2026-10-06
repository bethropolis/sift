package serve

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/bethropolis/sift/internal/config"
)

// testServer builds an unbound local server rooted at a temp project.
func testServer(t *testing.T) (*Server, string) {
	t.Helper()
	root := t.TempDir()
	for name, content := range map[string]string{
		"main.go":   "package main\n\nfunc main() {}\n",
		"README.md": "# test\n",
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	engineCfg := config.New()
	engineCfg.Quiet = true
	engineCfg.SmartFilter = false
	srv, err := New(&Config{Listen: "127.0.0.1:7777", Roots: []string{root}}, engineCfg, nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return srv, root
}

// do sends a request through the full middleware stack with a valid Host.
func do(srv *Server, method, target string, body any, cookies []*http.Cookie) *httptest.ResponseRecorder {
	var reader io.Reader
	if body != nil {
		data, _ := json.Marshal(body)
		reader = bytes.NewReader(data)
	}
	req := httptest.NewRequest(method, target, reader)
	req.Host = "127.0.0.1:7777"
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	srv.mux.ServeHTTP(rec, req)
	return rec
}

func loginCookies(t *testing.T, srv *Server) []*http.Cookie {
	t.Helper()
	rec := do(srv, "POST", "/api/login", map[string]string{"token": srv.auth.Token()}, nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("login = %d, want 204", rec.Code)
	}
	resp := rec.Result()
	defer resp.Body.Close()
	return resp.Cookies()
}

// TestPublicRouteSet asserts the public set is exactly static assets,
// GET /api/meta, and POST /api/login; everything else 401s without a session.
func TestPublicRouteSet(t *testing.T) {
	srv, _ := testServer(t)

	public := map[string]bool{
		"GET /": true, "GET /api/meta": true, "POST /api/login": true,
	}
	// /assets/* is public when the file exists; the test build embeds the
	// real UI, so probe one known asset instead of enumerating.
	for _, rt := range srv.routes() {
		if strings.HasPrefix(rt.path, "/assets/") {
			continue
		}
		key := rt.method + " " + rt.path
		target := rt.path
		if rt.method == "DELETE" {
			target += "?root=/nonexistent"
		}
		if rt.method == "GET" && (rt.path == "/api/browse" || rt.path == "/api/tree" || rt.path == "/api/file") {
			target += "?path=/nonexistent&root=/nonexistent"
		}
		rec := do(srv, rt.method, target, rt.method == "POST" || rt.method == "PUT" || rt.method == "DELETE" && false, nil)
		if public[key] {
			if rec.Code == http.StatusUnauthorized {
				t.Fatalf("%s is public but returned 401", key)
			}
			continue
		}
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("%s without session = %d, want 401", key, rec.Code)
		}
	}
}

// TestHostRebinding rejects foreign Host headers everywhere.
func TestHostRebinding(t *testing.T) {
	srv, _ := testServer(t)
	for _, target := range []string{"/", "/api/meta"} {
		req := httptest.NewRequest("GET", target, nil)
		req.Host = "evil.example.com"
		rec := httptest.NewRecorder()
		srv.mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusMisdirectedRequest {
			t.Fatalf("%s with evil host = %d, want 421", target, rec.Code)
		}
	}
}

// TestRequestHygiene covers Origin, fetch-metadata, content-type, OPTIONS,
// CORS absence, and the security headers.
func TestRequestHygiene(t *testing.T) {
	srv, _ := testServer(t)

	// Mismatched Origin on POST.
	req := httptest.NewRequest("POST", "/api/login", strings.NewReader(`{}`))
	req.Host = "127.0.0.1:7777"
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://evil.example.com")
	rec := httptest.NewRecorder()
	srv.mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("mismatched Origin = %d, want 403", rec.Code)
	}

	// Cross-site fetch metadata.
	req = httptest.NewRequest("POST", "/api/login", strings.NewReader(`{}`))
	req.Host = "127.0.0.1:7777"
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	rec = httptest.NewRecorder()
	srv.mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("cross-site fetch = %d, want 403", rec.Code)
	}

	// Non-JSON POST body.
	req = httptest.NewRequest("POST", "/api/login", strings.NewReader(`password=x`))
	req.Host = "127.0.0.1:7777"
	req.Header.Set("Content-Type", "text/plain")
	rec = httptest.NewRecorder()
	srv.mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("text/plain POST = %d, want 415", rec.Code)
	}

	// OPTIONS is never CORS.
	req = httptest.NewRequest("OPTIONS", "/api/meta", nil)
	req.Host = "127.0.0.1:7777"
	rec = httptest.NewRecorder()
	srv.mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("OPTIONS = %d, want 405", rec.Code)
	}

	// Headers present, CORS absent, on API and static alike.
	for _, target := range []string{"/api/meta", "/"} {
		req := httptest.NewRequest("GET", target, nil)
		req.Host = "127.0.0.1:7777"
		rec := httptest.NewRecorder()
		srv.mux.ServeHTTP(rec, req)
		h := rec.Header()
		if h.Get("Content-Security-Policy") == "" || h.Get("X-Content-Type-Options") == "" ||
			h.Get("Referrer-Policy") == "" || h.Get("X-Frame-Options") == "" ||
			h.Get("Cross-Origin-Opener-Policy") == "" || h.Get("Cross-Origin-Resource-Policy") == "" {
			t.Fatalf("%s missing security headers: %v", target, h)
		}
		for k := range h {
			if strings.HasPrefix(strings.ToLower(k), "access-control-") {
				t.Fatalf("%s emits CORS header %s", target, k)
			}
		}
		if strings.HasPrefix(target, "/api/") && h.Get("Cache-Control") != "no-store" {
			t.Fatalf("%s missing no-store", target)
		}
	}
}

// TestRemoteRefusalMatrix checks each missing requirement refuses before
// binding, naming the flag.
func TestRemoteRefusalMatrix(t *testing.T) {
	base := Config{
		Listen:        "0.0.0.0:7777",
		AllowRemote:   true,
		Roots:         []string{t.TempDir()},
		AllowedHosts:  []string{"example.com"},
		TLSSelfSigned: true,
	}
	cases := []struct {
		name   string
		mutate func(*Config)
		want   string
	}{
		{"allow-remote", func(c *Config) { c.AllowRemote = false }, "--allow-remote"},
		{"password", func(c *Config) {}, "--password-file"},
		{"roots", func(c *Config) { c.Roots = nil }, "--root"},
		{"hosts", func(c *Config) { c.AllowedHosts = nil }, "--allowed-host"},
		{"tls", func(c *Config) { c.TLSSelfSigned = false }, "--tls-cert"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := base
			tc.mutate(&cfg)
			if tc.name == "password" {
				// No password configured anywhere.
				os.Unsetenv("SIFT_SERVE_PASSWORD")
			} else {
				t.Setenv("SIFT_SERVE_PASSWORD", "long-enough-password")
			}
			if _, err := cfg.Validate(); err == nil {
				t.Fatal("remote config accepted without requirement")
			} else if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error %q does not name %q", err, tc.want)
			}
		})
	}
}

// TestLoginAndSession exercises the cookie flow end to end.
func TestLoginAndSession(t *testing.T) {
	srv, _ := testServer(t)

	cookies := loginCookies(t, srv)

	// Authenticated meta carries roots and styles; anonymous does not.
	rec := do(srv, "GET", "/api/meta", nil, cookies)
	var meta map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&meta); err != nil {
		t.Fatal(err)
	}
	if meta["authenticated"] != true || meta["roots"] == nil || meta["styles"] == nil {
		t.Fatalf("authed meta incomplete: %v", meta)
	}
	anon := do(srv, "GET", "/api/meta", nil, nil)
	var anonMeta map[string]any
	if err := json.NewDecoder(anon.Body).Decode(&anonMeta); err != nil {
		t.Fatal(err)
	}
	if anonMeta["authenticated"] != false {
		t.Fatalf("anon meta authenticated: %v", anonMeta)
	}
	if _, hasRoots := anonMeta["roots"]; hasRoots {
		t.Fatalf("anon meta leaks roots: %v", anonMeta)
	}

	// Logout clears the session.
	if rec := do(srv, "POST", "/api/logout", nil, cookies); rec.Code != http.StatusNoContent {
		t.Fatalf("logout = %d", rec.Code)
	}

	// Wrong credentials (first failure answers 401 immediately).
	if rec := do(srv, "POST", "/api/login", map[string]string{"token": "nope"}, nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("bad login = %d, want 401", rec.Code)
	}
}

// TestLoginThrottling locks out brute force with 429 + retryAfter.
func TestLoginThrottling(t *testing.T) {
	srv, _ := testServer(t)
	var last *httptest.ResponseRecorder
	for i := 0; i < 8; i++ {
		last = do(srv, "POST", "/api/login", map[string]string{"password": "wrong-password"}, nil)
	}
	if last.Code != http.StatusTooManyRequests {
		t.Fatalf("after brute force = %d, want 429", last.Code)
	}
	var body map[string]any
	if err := json.NewDecoder(last.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if _, ok := body["retryAfter"]; !ok {
		t.Fatalf("429 lacks retryAfter: %v", body)
	}
}

// TestDataEndpoints walks the jail-gated data path with a real project.
func TestDataEndpoints(t *testing.T) {
	srv, root := testServer(t)
	cookies := loginCookies(t, srv)

	// Browse.
	rec := do(srv, "GET", "/api/browse?path="+root, nil, cookies)
	if rec.Code != http.StatusOK {
		t.Fatalf("browse = %d", rec.Code)
	}
	// Outside roots.
	if rec := do(srv, "GET", "/api/browse?path=/etc", nil, cookies); rec.Code != http.StatusForbidden {
		t.Fatalf("outside browse = %d, want 403", rec.Code)
	}

	// Tree.
	rec = do(srv, "GET", "/api/tree?root="+root, nil, cookies)
	if rec.Code != http.StatusOK {
		t.Fatalf("tree = %d", rec.Code)
	}
	var tree struct {
		Files []struct {
			Path     string  `json:"path"`
			Tokens   int     `json:"tokens"`
			Language string  `json:"language"`
			Score    float64 `json:"score"`
		} `json:"files"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&tree); err != nil {
		t.Fatal(err)
	}
	if len(tree.Files) != 2 {
		t.Fatalf("tree files = %d, want 2", len(tree.Files))
	}

	// File preview (redacted, capped).
	rec = do(srv, "GET", "/api/file?root="+root+"&path=main.go&mode=full", nil, cookies)
	if rec.Code != http.StatusOK {
		t.Fatalf("file = %d", rec.Code)
	}

	// Smart select.
	rec = do(srv, "POST", "/api/smart-select", map[string]any{"root": root, "budget": 64000}, cookies)
	if rec.Code != http.StatusOK {
		t.Fatalf("smart-select = %d", rec.Code)
	}

	// Pack with explicit selections.
	rec = do(srv, "POST", "/api/pack", map[string]any{
		"root": root, "budget": 64000, "style": "xml", "prompt": "", "redact": true,
		"selections": map[string]string{"main.go": "full", "README.md": "skip"},
	}, cookies)
	if rec.Code != http.StatusOK {
		t.Fatalf("pack = %d", rec.Code)
	}
	var pack struct {
		FileCount int      `json:"fileCount"`
		Skipped   []string `json:"skipped"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&pack); err != nil {
		t.Fatal(err)
	}
	if pack.FileCount != 1 {
		t.Fatalf("pack fileCount = %d, want 1", pack.FileCount)
	}

	// Recents round-trip.
	if rec := do(srv, "POST", "/api/recents", map[string]string{"root": root}, cookies); rec.Code != http.StatusNoContent {
		t.Fatalf("recents post = %d", rec.Code)
	}
	if rec := do(srv, "GET", "/api/recents", nil, cookies); rec.Code != http.StatusOK {
		t.Fatalf("recents get = %d", rec.Code)
	}

	// Settings round-trip.
	if rec := do(srv, "PUT", "/api/settings", map[string]any{"defaultStyle": "xml", "defaultBudget": 32000, "theme": "nord"}, cookies); rec.Code != http.StatusNoContent {
		t.Fatalf("settings put = %d", rec.Code)
	}
	if rec := do(srv, "PUT", "/api/settings", map[string]any{"theme": "../evil"}, cookies); rec.Code != http.StatusBadRequest {
		t.Fatalf("bad theme = %d, want 400", rec.Code)
	}
}

// TestBrowseHiddenDefault ensures dot-directories stay hidden unless
// ?hidden=1, entries carry modTime, and authed meta names a defaultBrowse
// inside the jail.
func TestBrowseHiddenDefault(t *testing.T) {
	srv, root := testServer(t)
	cookies := loginCookies(t, srv)

	if err := os.MkdirAll(filepath.Join(root, "visible"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".hidden"), 0o755); err != nil {
		t.Fatal(err)
	}

	var browse struct {
		Entries []struct {
			Name    string `json:"name"`
			ModTime int64  `json:"modTime"`
		} `json:"entries"`
	}
	rec := do(srv, "GET", "/api/browse?path="+root, nil, cookies)
	if rec.Code != http.StatusOK {
		t.Fatalf("browse = %d", rec.Code)
	}
	if err := json.NewDecoder(rec.Body).Decode(&browse); err != nil {
		t.Fatal(err)
	}
	for _, e := range browse.Entries {
		if strings.HasPrefix(e.Name, ".") {
			t.Fatalf("dot-dir %q listed without ?hidden=1", e.Name)
		}
		if e.ModTime <= 0 {
			t.Fatalf("entry %q lacks modTime", e.Name)
		}
	}
	seen := false
	for _, e := range browse.Entries {
		if e.Name == "visible" {
			seen = true
		}
	}
	if !seen {
		t.Fatalf("visible dir missing: %+v", browse.Entries)
	}

	rec = do(srv, "GET", "/api/browse?path="+root+"&hidden=1", nil, cookies)
	if rec.Code != http.StatusOK {
		t.Fatalf("browse hidden=1 = %d", rec.Code)
	}
	browse.Entries = nil
	if err := json.NewDecoder(rec.Body).Decode(&browse); err != nil {
		t.Fatal(err)
	}
	seen = false
	for _, e := range browse.Entries {
		if e.Name == ".hidden" {
			seen = true
		}
	}
	if !seen {
		t.Fatalf(".hidden missing with ?hidden=1: %+v", browse.Entries)
	}

	var meta struct {
		DefaultBrowse string   `json:"defaultBrowse"`
		Roots         []string `json:"roots"`
	}
	rec = do(srv, "GET", "/api/meta", nil, cookies)
	if err := json.NewDecoder(rec.Body).Decode(&meta); err != nil {
		t.Fatal(err)
	}
	if len(meta.Roots) == 0 || meta.DefaultBrowse == "" {
		t.Fatalf("meta lacks roots/defaultBrowse: %+v", meta)
	}
	if meta.DefaultBrowse != root && !strings.HasPrefix(meta.DefaultBrowse, root+string(os.PathSeparator)) {
		// ~/Projects inside the jail wins when present; otherwise the root.
		t.Fatalf("defaultBrowse %q escapes the jail root %q", meta.DefaultBrowse, root)
	}
}

// TestSettingsBrowserPrefs round-trips the folder-browser settings and
// rejects bad fileSort values.
func TestSettingsBrowserPrefs(t *testing.T) {
	srv, _ := testServer(t)
	cookies := loginCookies(t, srv)

	if rec := do(srv, "PUT", "/api/settings", map[string]any{"showHidden": true, "fileSort": "updated"}, cookies); rec.Code != http.StatusNoContent {
		t.Fatalf("settings put = %d", rec.Code)
	}
	var got struct {
		ShowHidden bool   `json:"showHidden"`
		FileSort   string `json:"fileSort"`
	}
	rec := do(srv, "GET", "/api/settings", nil, cookies)
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if !got.ShowHidden || got.FileSort != "updated" {
		t.Fatalf("settings = %+v, want showHidden/fileSort", got)
	}
	if rec := do(srv, "PUT", "/api/settings", map[string]any{"fileSort": "evil"}, cookies); rec.Code != http.StatusBadRequest {
		t.Fatalf("bad fileSort = %d, want 400", rec.Code)
	}
	// Restore defaults so other tests see a clean state file.
	if rec := do(srv, "PUT", "/api/settings", map[string]any{"showHidden": false, "fileSort": "name"}, cookies); rec.Code != http.StatusNoContent {
		t.Fatalf("settings restore = %d", rec.Code)
	}
}

// TestRemoteRedactRefused rejects redact=false without --allow-unredacted.
func TestRemoteRedactRefused(t *testing.T) {
	root := t.TempDir()
	engineCfg := config.New()
	engineCfg.Quiet = true
	t.Setenv("SIFT_SERVE_PASSWORD", "long-enough-password")
	srv, err := New(&Config{
		Listen: "0.0.0.0:7777", AllowRemote: true, Roots: []string{root},
		AllowedHosts: []string{"example.com"}, TLSSelfSigned: true,
	}, engineCfg, nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	cookies := func() []*http.Cookie {
		t.Helper()
		rec := do(srv, "POST", "/api/login", map[string]string{"password": "long-enough-password"}, nil)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("login = %d, want 204", rec.Code)
		}
		resp := rec.Result()
		defer resp.Body.Close()
		return resp.Cookies()
	}()
	rec := do(srv, "POST", "/api/pack", map[string]any{
		"root": root, "budget": 1000, "redact": false, "selections": map[string]string{},
	}, cookies)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("remote redact=false = %d, want 400", rec.Code)
	}
}

// TestIdleGoroutines asserts an idle server returns to its goroutine
// baseline (modulo keep-alive connections, which httptest closes).
func TestIdleGoroutines(t *testing.T) {
	srv, _ := testServer(t)
	ts := httptest.NewServer(srv.mux)
	defer ts.Close()

	baseline := runtime.NumGoroutine()
	client := &http.Client{Transport: &http.Transport{DisableKeepAlives: true}}
	for i := 0; i < 5; i++ {
		resp, err := client.Get(ts.URL + "/api/meta")
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
	}
	deadline := time.Now().Add(10 * time.Second)
	for runtime.NumGoroutine() > baseline+3 && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
	if got := runtime.NumGoroutine(); got > baseline+3 {
		t.Fatalf("goroutines %d, baseline %d: server does not settle", got, baseline)
	}
}
