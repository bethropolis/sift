package serve

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bethropolis/sift/internal/config"
)

func pairServer(t *testing.T, mutate func(*Config)) (*Server, string) {
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
	cfg := &Config{
		Listen:         "127.0.0.1:7777",
		Roots:          []string{root},
		Pair:           true,
		PublicURL:      "https://example.com",
		PairTTL:        time.Minute,
		PairSessionTTL: time.Hour,
	}
	if mutate != nil {
		mutate(cfg)
	}
	engineCfg := config.New()
	engineCfg.Quiet = true
	engineCfg.SmartFilter = false
	srv, err := New(cfg, engineCfg, nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	srv.pairBase = cfg.PublicURL
	return srv, root
}

func pairCookies(t *testing.T, srv *Server) []*http.Cookie {
	t.Helper()
	token, err := srv.pairTokens.mint(srv.cfg.PairTTL)
	if err != nil {
		t.Fatal(err)
	}
	rec := do(srv, "POST", "/api/session/pair", map[string]string{"token": token}, nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("pair = %d, want 204 (%s)", rec.Code, rec.Body.String())
	}
	resp := rec.Result()
	defer resp.Body.Close()
	return resp.Cookies()
}

func TestPairConfigRefusalMatrix(t *testing.T) {
	root := t.TempDir()
	base := Config{
		Listen:    "127.0.0.1:7777",
		Pair:      true,
		Roots:     []string{root},
		PublicURL: "https://example.com",
	}
	cases := []struct {
		name   string
		mutate func(*Config)
		want   string
	}{
		{"roots", func(c *Config) { c.Roots = nil }, "--root"},
		{"clone", func(c *Config) { c.AllowClone = true }, "--allow-clone"},
		{"unredacted", func(c *Config) { c.AllowUnredacted = true }, "--allow-unredacted"},
		{"app", func(c *Config) { c.App = true }, "--app"},
		{"no-url", func(c *Config) { c.PublicURL = "" }, "--public-url"},
		{"bad-scheme", func(c *Config) { c.PublicURL = "ftp://example.com" }, "scheme"},
		{"userinfo", func(c *Config) { c.PublicURL = "https://user@example.com" }, "userinfo"},
		{"query", func(c *Config) { c.PublicURL = "https://example.com?x=1" }, "query"},
		{"fragment", func(c *Config) { c.PublicURL = "https://example.com#x" }, "fragment"},
		{"plain-http", func(c *Config) { c.PublicURL = "http://example.com" }, "--insecure-http"},
		{"remote-no-proxy", func(c *Config) {
			c.Listen = "0.0.0.0:7777"
		}, "--behind-proxy"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := base
			tc.mutate(&cfg)
			if _, err := cfg.Validate(); err == nil {
				t.Fatal("invalid pair config accepted")
			} else if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error %q does not name %q", err, tc.want)
			}
		})
	}

	ok := base
	if _, err := ok.Validate(); err != nil {
		t.Fatalf("happy path rejected: %v", err)
	}
	if !ok.BehindProxy {
		t.Fatal("https public URL on a loopback bind should imply --behind-proxy")
	}

	httpCfg := base
	httpCfg.PublicURL = "http://example.com"
	httpCfg.InsecureHTTP = true
	if _, err := httpCfg.Validate(); err != nil {
		t.Fatalf("http+insecure rejected: %v", err)
	}

	noURL := base
	noURL.PublicURL = ""
	noURL.InsecureHTTP = true
	if _, err := noURL.Validate(); err != nil {
		t.Fatalf("no-url+insecure rejected: %v", err)
	}

	insecureWithoutPair := Config{Listen: "127.0.0.1:7777", InsecureHTTP: true}
	if _, err := insecureWithoutPair.Validate(); err == nil || !strings.Contains(err.Error(), "--pair") {
		t.Fatalf("--insecure-http without --pair = %v, want pair refusal", err)
	}
}

func TestPairRouteOnlyRegisteredWithPair(t *testing.T) {
	srv, _ := testServer(t)
	rec := do(srv, "POST", "/api/session/pair", map[string]string{"token": "x"}, nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("pair route without --pair = %d, want 404", rec.Code)
	}
	for _, rt := range srv.routes() {
		if rt.path == "/api/session/pair" {
			t.Fatal("pair route enumerated without --pair")
		}
	}
}

func TestPairTokenSingleUseAndInvalidation(t *testing.T) {
	store := newPairTokens()
	now := time.Unix(1000, 0)
	store.now = func() time.Time { return now }
	old, err := store.mint(time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if !store.exchange(old) {
		t.Fatal("live token rejected")
	}
	if store.exchange(old) {
		t.Fatal("replayed token accepted")
	}

	first, _ := store.mint(time.Minute)
	second, _ := store.mint(time.Minute)
	if store.exchange(first) {
		t.Fatal("minting a new token did not invalidate the old one")
	}
	if !store.exchange(second) {
		t.Fatal("newest token rejected")
	}

	expiring, _ := store.mint(time.Minute)
	now = now.Add(2 * time.Minute)
	if store.exchange(expiring) {
		t.Fatal("expired token accepted")
	}
}

func TestPairEndpointFailuresAreGeneric(t *testing.T) {
	bad := []any{
		map[string]string{"token": "wrong"},
		map[string]any{"token": 123},
		[]string{"not", "an", "object"},
	}
	for _, body := range bad {
		// Fresh server so each case starts outside the login backoff.
		srv, _ := pairServer(t, nil)
		rec := do(srv, "POST", "/api/session/pair", body, nil)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("failure %v = %d, want 401", body, rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "invalid or expired pairing token") {
			t.Fatalf("failure body = %s", rec.Body.String())
		}
	}
}

func TestPairEndpointThrottles(t *testing.T) {
	srv, _ := pairServer(t, nil)
	first := do(srv, "POST", "/api/session/pair", map[string]string{"token": "wrong"}, nil)
	if first.Code != http.StatusUnauthorized {
		t.Fatalf("first failure = %d, want 401", first.Code)
	}
	second := do(srv, "POST", "/api/session/pair", map[string]string{"token": "wrong"}, nil)
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("second failure = %d, want 429", second.Code)
	}
	if second.Header().Get("Retry-After") == "" {
		t.Fatal("429 missing Retry-After")
	}
}

func TestPairEndpointSessionCap(t *testing.T) {
	srv, _ := pairServer(t, func(c *Config) { c.PairMaxSessions = 1 })
	_ = pairCookies(t, srv)
	token, _ := srv.pairTokens.mint(srv.cfg.PairTTL)
	rec := do(srv, "POST", "/api/session/pair", map[string]string{"token": token}, nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("second paired session = %d, want 401", rec.Code)
	}
}

func TestReadOnlySessionDefaultDeny(t *testing.T) {
	srv, root := pairServer(t, nil)
	cookies := pairCookies(t, srv)

	denied := []struct {
		method string
		target string
	}{
		{"GET", "/api/recents"},
		{"POST", "/api/recents"},
		{"DELETE", "/api/recents?root=/x"},
		{"GET", "/api/settings"},
		{"PUT", "/api/settings"},
		{"GET", "/api/presets"},
		{"POST", "/api/presets"},
		{"DELETE", "/api/presets"},
		{"GET", "/api/clones"},
		{"POST", "/api/clone"},
		{"DELETE", "/api/clone?root=/x"},
		{"POST", "/api/modes"},
		{"POST", "/api/logout"},
	}
	for _, tc := range denied {
		rec := do(srv, tc.method, tc.target, nil, cookies)
		if rec.Code != http.StatusForbidden {
			t.Errorf("%s %s = %d, want 403", tc.method, tc.target, rec.Code)
		}
	}

	allowed := []struct {
		method string
		target string
		body   any
	}{
		{"GET", "/api/meta", nil},
		{"GET", "/api/browse?path=" + root, nil},
		{"GET", "/api/tree?root=" + root, nil},
		{"GET", "/api/file?root=" + root + "&path=main.go&mode=full", nil},
		{"POST", "/api/smart-select", map[string]any{"root": root, "budget": 1000}},
		{"POST", "/api/pack", map[string]any{"root": root, "selections": map[string]string{"main.go": "full"}}},
		{"POST", "/api/diff", map[string]any{"root": root}},
		{"POST", "/api/follow", map[string]any{"root": root, "seed": "main.go"}},
	}
	for _, tc := range allowed {
		rec := do(srv, tc.method, tc.target, tc.body, cookies)
		if rec.Code == http.StatusForbidden || rec.Code == http.StatusUnauthorized {
			t.Errorf("%s %s = %d, want read allowed", tc.method, tc.target, rec.Code)
		}
	}

	rec := do(srv, "GET", "/api/meta", nil, cookies)
	var meta map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&meta); err != nil {
		t.Fatal(err)
	}
	if meta["readOnly"] != true {
		t.Fatalf("meta readOnly = %v", meta["readOnly"])
	}
}

func TestEveryRouteClassified(t *testing.T) {
	for _, srv := range []*Server{mustTestServer(t), mustPairServer(t)} {
		for _, rt := range srv.routes() {
			if rt.access == accessUnclassified {
				t.Errorf("%s %s is unclassified", rt.method, rt.path)
			}
		}
	}
}

func mustTestServer(t *testing.T) *Server {
	t.Helper()
	srv, _ := testServer(t)
	return srv
}

func mustPairServer(t *testing.T) *Server {
	t.Helper()
	srv, _ := pairServer(t, nil)
	return srv
}

func TestPairSensitiveFilesExcludedAndRedacted(t *testing.T) {
	srv, root := pairServer(t, nil)
	files := map[string]string{
		"id_rsa":     "PRIVATE",
		"server.pem": "CERT",
		".env":       "TOKEN=1",
		"secret.txt": "const aws = \"AKIAIOSFODNN7EXAMPLE\"\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	cookies := pairCookies(t, srv)

	rec := do(srv, "GET", "/api/tree?root="+root, nil, cookies)
	if rec.Code != http.StatusOK {
		t.Fatalf("tree = %d", rec.Code)
	}
	tree := rec.Body.String()
	for _, name := range []string{"id_rsa", "server.pem", ".env"} {
		if strings.Contains(tree, name) {
			t.Errorf("tree leaked sensitive file %q: %s", name, tree)
		}
	}

	for _, name := range []string{"id_rsa", "server.pem", ".env"} {
		rec := do(srv, "GET", "/api/file?root="+root+"&path="+name+"&mode=full", nil, cookies)
		if rec.Code != http.StatusForbidden {
			t.Errorf("file %s = %d, want 403", name, rec.Code)
		}
	}

	rec = do(srv, "GET", "/api/file?root="+root+"&path=secret.txt&mode=full", nil, cookies)
	if rec.Code != http.StatusOK {
		t.Fatalf("secret.txt preview = %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "AKIAIOSFODNN7EXAMPLE") {
		t.Fatalf("preview leaked secret: %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "REDACTED_SECRET") {
		t.Fatalf("preview was not redacted: %s", rec.Body.String())
	}

	rec = do(srv, "POST", "/api/pack", map[string]any{
		"root": root, "selections": map[string]string{"secret.txt": "full", "id_rsa": "full"},
	}, cookies)
	if rec.Code != http.StatusOK {
		t.Fatalf("pack = %d", rec.Code)
	}
	var packed struct {
		Document string `json:"document"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&packed); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(packed.Document, "AKIAIOSFODNN7EXAMPLE") || strings.Contains(packed.Document, "PRIVATE") {
		t.Fatalf("pack leaked sensitive content:\n%s", packed.Document)
	}
}

func TestPairURLUsesFragment(t *testing.T) {
	srv, _ := pairServer(t, nil)
	srv.pairBase = "https://example.com"
	if got := srv.pairURL("tok"); got != "https://example.com/#pair=tok" {
		t.Fatalf("pairURL = %q", got)
	}
	if got := srv.pairBaseURL("http", "127.0.0.1:7777"); got == "" {
		t.Fatal("insecure pair base URL is empty")
	}
}
