package serve

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/bethropolis/sift/internal/state"
)

// installFakeGit puts a stand-in git first on PATH. `body` runs after the
// stand-in has marked that it started; $last is the destination argument.
func installFakeGit(t *testing.T, body string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("shell stand-in needs a POSIX sh")
	}
	dir := t.TempDir()
	script := "#!/bin/sh\nfor a; do last=$a; done\n" + body + "\n"
	if err := os.WriteFile(filepath.Join(dir, "git"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// successGit "clones" a tiny project and reports progress like git does.
const successGit = `mkdir -p "$last" && printf 'package main\n\nfunc main() {}\n' > "$last/main.go"
printf 'Receiving objects:  50%% (1/2)\rReceiving objects: 100%% (2/2), done.\n' >&2`

type cloneEvents []map[string]any

func (e cloneEvents) last() map[string]any { return e[len(e)-1] }

func (e cloneEvents) count(kind string) int {
	n := 0
	for _, ev := range e {
		if ev["type"] == kind {
			n++
		}
	}
	return n
}

func parseNDJSON(t *testing.T, body []byte) cloneEvents {
	t.Helper()
	var out cloneEvents
	sc := bufio.NewScanner(bytes.NewReader(body))
	for sc.Scan() {
		var ev map[string]any
		if err := json.Unmarshal(sc.Bytes(), &ev); err != nil {
			t.Fatalf("bad NDJSON line %q: %v", sc.Text(), err)
		}
		out = append(out, ev)
	}
	return out
}

func postClone(srv *Server, body any, cookies []*http.Cookie) *httptest.ResponseRecorder {
	return do(srv, "POST", "/api/clone", body, cookies)
}

func TestCloneEndToEnd(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	installFakeGit(t, successGit)
	srv, _ := testServer(t)
	t.Cleanup(srv.clones.removeAll)
	cookies := loginCookies(t, srv)

	rec := postClone(srv, map[string]any{"url": "https://example.com/owner/widget.git"}, cookies)
	if rec.Code != http.StatusOK {
		t.Fatalf("clone = %d: %s", rec.Code, rec.Body)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/x-ndjson" {
		t.Errorf("content-type = %q", ct)
	}
	events := parseNDJSON(t, rec.Body.Bytes())
	if events.count("progress") < 2 {
		t.Errorf("want progress events, got %v", events)
	}
	done := events.last()
	if done["type"] != "done" {
		t.Fatalf("last event = %v", done)
	}
	root, _ := done["root"].(string)
	if done["name"] != "widget" || !strings.HasPrefix(filepath.Base(filepath.Dir(root)), "sift-clone-") {
		t.Errorf("unexpected done event: %v", done)
	}

	// The checkout is a real project the whole API can open, though it lives
	// outside the startup roots.
	if rec := do(srv, "GET", "/api/tree?root="+root, nil, cookies); rec.Code != http.StatusOK {
		t.Fatalf("tree of clone = %d: %s", rec.Code, rec.Body)
	}
	// Its siblings and parent stay unreachable: only the repo dir is a root.
	if rec := do(srv, "GET", "/api/browse?path="+filepath.Dir(root), nil, cookies); rec.Code != http.StatusForbidden {
		t.Errorf("clone parent dir browse = %d, want 403", rec.Code)
	}

	// Listed for this session…
	rec = do(srv, "GET", "/api/clones", nil, cookies)
	var list []struct{ Root, Name, URL string }
	if err := json.NewDecoder(rec.Body).Decode(&list); err != nil || len(list) != 1 || list[0].Root != root {
		t.Fatalf("clones list = %v err=%v", list, err)
	}

	// …but never written to the persistent recents.
	if rec := do(srv, "POST", "/api/recents", map[string]string{"root": root}, cookies); rec.Code != http.StatusNoContent {
		t.Fatalf("recents post = %d", rec.Code)
	}
	if recents, _ := state.ListRecents(); len(recents) != 0 {
		t.Errorf("temporary clone leaked into recents: %v", recents)
	}

	// Meta advertises the feature.
	var meta struct {
		Features map[string]bool `json:"features"`
	}
	_ = json.NewDecoder(do(srv, "GET", "/api/meta", nil, cookies).Body).Decode(&meta)
	if !meta.Features["clone"] {
		t.Errorf("meta features = %v", meta.Features)
	}

	// Remove: directory gone, jail root withdrawn, second delete is a 404.
	if rec := do(srv, "DELETE", "/api/clone?root="+root, nil, cookies); rec.Code != http.StatusNoContent {
		t.Fatalf("delete = %d", rec.Code)
	}
	if _, err := os.Stat(filepath.Dir(root)); !os.IsNotExist(err) {
		t.Errorf("temp dir survived delete: %v", err)
	}
	if rec := do(srv, "GET", "/api/tree?root="+root, nil, cookies); rec.Code != http.StatusForbidden {
		t.Errorf("tree after delete = %d, want 403", rec.Code)
	}
	if rec := do(srv, "DELETE", "/api/clone?root="+root, nil, cookies); rec.Code != http.StatusNotFound {
		t.Errorf("repeat delete = %d, want 404", rec.Code)
	}
}

func TestCloneDeleteOnlyServerCreated(t *testing.T) {
	installFakeGit(t, successGit)
	srv, root := testServer(t)
	cookies := loginCookies(t, srv)
	// A real project dir inside the roots is not a clone and must survive.
	if rec := do(srv, "DELETE", "/api/clone?root="+root, nil, cookies); rec.Code != http.StatusNotFound {
		t.Fatalf("delete of foreign dir = %d, want 404", rec.Code)
	}
	if _, err := os.Stat(filepath.Join(root, "main.go")); err != nil {
		t.Fatalf("foreign dir touched: %v", err)
	}
}

func TestCloneRejectsBadRequests(t *testing.T) {
	installFakeGit(t, `touch "$last.ran"`)
	srv, root := testServer(t)
	cookies := loginCookies(t, srv)

	for name, body := range map[string]map[string]any{
		"local path":     {"url": root},
		"file scheme":    {"url": "file://" + root},
		"ext helper":     {"url": "ext::sh -c id"},
		"option":         {"url": "--upload-pack=id"},
		"credentials":    {"url": "https://u:secret@example.com/o/r"},
		"empty":          {"url": ""},
		"bad branch":     {"url": "https://example.com/o/r", "branch": "-x"},
		"depth high":     {"url": "https://example.com/o/r", "depth": 100000},
		"depth negative": {"url": "https://example.com/o/r", "depth": -1},
	} {
		if rec := postClone(srv, body, cookies); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status %d, want 400 (%s)", name, rec.Code, rec.Body)
		}
	}
	if len(srv.clones.temps) != 0 {
		t.Errorf("rejected requests left temp dirs: %v", srv.clones.temps)
	}
	if rec := do(srv, "POST", "/api/clone", map[string]any{"url": "https://example.com/o/r", "extra": 1}, cookies); rec.Code != http.StatusBadRequest {
		t.Errorf("unknown field = %d, want 400", rec.Code)
	}
	if rec := postClone(srv, map[string]any{"url": "https://example.com/o/r"}, nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("anonymous clone = %d, want 401", rec.Code)
	}
}

func TestCloneFailureReportsDetailAndCleans(t *testing.T) {
	installFakeGit(t, `printf 'fatal: Authentication failed for https://example.com/o/r\n' >&2; exit 128`)
	srv, _ := testServer(t)
	cookies := loginCookies(t, srv)

	rec := postClone(srv, map[string]any{"url": "https://example.com/o/r"}, cookies)
	ev := parseNDJSON(t, rec.Body.Bytes()).last()
	if ev["type"] != "error" || !strings.Contains(ev["detail"].(string), "Authentication failed") {
		t.Fatalf("error event = %v", ev)
	}
	if len(srv.clones.temps) != 0 || len(srv.clones.entries) != 0 {
		t.Errorf("failed clone left state: %v %v", srv.clones.temps, srv.clones.entries)
	}
	if srv.clones.isRunning() {
		t.Error("clone slot not released after failure")
	}
}

func TestCloneRefusesHostileProjectConfig(t *testing.T) {
	// A cloned repo is untrusted input: a .sift.toml pointing outside the
	// project is refused at clone time and the checkout is removed.
	installFakeGit(t, `mkdir -p "$last" && printf '[sift]\nprompt_file = "/etc/passwd"\n' > "$last/.sift.toml"`)
	srv, _ := testServer(t)
	cookies := loginCookies(t, srv)

	ev := parseNDJSON(t, postClone(srv, map[string]any{"url": "https://example.com/o/r"}, cookies).Body.Bytes()).last()
	if ev["type"] != "error" || !strings.Contains(ev["message"].(string), ".sift.toml") {
		t.Fatalf("event = %v", ev)
	}
	if len(srv.clones.temps) != 0 || len(srv.clones.entries) != 0 {
		t.Errorf("refused clone left state: %v", srv.clones.temps)
	}
}

func TestCloneOneAtATimeAndCapAndCancel(t *testing.T) {
	gate := filepath.Join(t.TempDir(), "gate")
	installFakeGit(t, `mkdir -p "$last"; while [ ! -e "`+gate+`" ]; do sleep 0.02; done`)
	srv, _ := testServer(t)
	t.Cleanup(srv.clones.removeAll)
	cookies := loginCookies(t, srv)

	finished := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		finished <- postClone(srv, map[string]any{"url": "https://example.com/o/first"}, cookies)
	}()
	waitFor(t, srv.clones.isRunning)

	if rec := postClone(srv, map[string]any{"url": "https://example.com/o/second"}, cookies); rec.Code != http.StatusConflict {
		t.Errorf("concurrent clone = %d, want 409", rec.Code)
	}
	if err := os.WriteFile(gate, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if ev := parseNDJSON(t, (<-finished).Body.Bytes()).last(); ev["type"] != "done" {
		t.Fatalf("first clone = %v", ev)
	}

	// Cap: fill the remaining slots, then the next is refused.
	for i := 1; i < maxClones; i++ {
		rec := postClone(srv, map[string]any{"url": "https://example.com/o/repo"}, cookies)
		if ev := parseNDJSON(t, rec.Body.Bytes()).last(); ev["type"] != "done" {
			t.Fatalf("clone %d = %v", i, ev)
		}
	}
	if rec := postClone(srv, map[string]any{"url": "https://example.com/o/over"}, cookies); rec.Code != http.StatusConflict {
		t.Errorf("over-cap clone = %d, want 409", rec.Code)
	}
}

func TestCloneCancelKillsAndCleans(t *testing.T) {
	installFakeGit(t, `exec sleep 30`)
	srv, _ := testServer(t)
	cookies := loginCookies(t, srv)

	ctx, cancel := context.WithCancel(context.Background())
	body, _ := json.Marshal(map[string]any{"url": "https://example.com/o/r"})
	req := httptest.NewRequest("POST", "/api/clone", bytes.NewReader(body)).WithContext(ctx)
	req.Host = "127.0.0.1:7777"
	req.Header.Set("Content-Type", "application/json")
	for _, c := range cookies {
		req.AddCookie(c)
	}
	done := make(chan struct{})
	go func() {
		srv.mux.ServeHTTP(httptest.NewRecorder(), req)
		close(done)
	}()
	waitFor(t, srv.clones.isRunning)
	cancel() // the browser aborting the request
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("handler did not return after the client aborted")
	}
	if srv.clones.isRunning() || len(srv.clones.temps) != 0 {
		t.Errorf("abort left state: running=%v temps=%v", srv.clones.isRunning(), srv.clones.temps)
	}
}

func TestCloneShutdownRemovesCheckouts(t *testing.T) {
	installFakeGit(t, successGit)
	srv, _ := testServer(t)
	cookies := loginCookies(t, srv)
	ev := parseNDJSON(t, postClone(srv, map[string]any{"url": "https://example.com/o/r"}, cookies).Body.Bytes()).last()
	root := ev["root"].(string)

	srv.clones.removeAll() // what Run's deferred cleanup does on every exit path
	if _, err := os.Stat(filepath.Dir(root)); !os.IsNotExist(err) {
		t.Errorf("checkout survived shutdown cleanup: %v", err)
	}
	srv.clones.removeAll() // idempotent
}

func TestCloneAvailability(t *testing.T) {
	// No git on PATH: hidden in meta and 404 on every route.
	t.Setenv("PATH", t.TempDir())
	srv, _ := testServer(t)
	cookies := loginCookies(t, srv)
	if srv.cloneOK {
		t.Fatal("clone enabled without git")
	}
	var meta struct {
		Features map[string]bool `json:"features"`
	}
	_ = json.NewDecoder(do(srv, "GET", "/api/meta", nil, cookies).Body).Decode(&meta)
	if meta.Features["clone"] {
		t.Error("meta advertises clone without git")
	}
	if rec := postClone(srv, map[string]any{"url": "https://example.com/o/r"}, cookies); rec.Code != http.StatusNotFound {
		t.Errorf("clone without git = %d, want 404", rec.Code)
	}
	if rec := do(srv, "GET", "/api/clones", nil, cookies); rec.Code != http.StatusNotFound {
		t.Errorf("list without git = %d, want 404", rec.Code)
	}
}

func TestCloneGatedByFlag(t *testing.T) {
	installFakeGit(t, successGit)
	srv, _ := testServer(t)
	if !srv.cloneAllowed() {
		t.Error("loopback bind with --allow-clone should allow clone")
	}

	// Without the flag the feature is off everywhere, even on loopback with
	// git installed: routes 404 and meta hides it.
	plain, err := New(&Config{Listen: "127.0.0.1:7777", Roots: []string{t.TempDir()}}, srv.engineCfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	if plain.cloneAllowed() {
		t.Error("clone allowed without --allow-clone")
	}
	cookies := loginCookies(t, plain)
	if rec := postClone(plain, map[string]any{"url": "https://example.com/o/r"}, cookies); rec.Code != http.StatusNotFound {
		t.Errorf("clone without flag = %d, want 404", rec.Code)
	}
	if rec := do(plain, "GET", "/api/clones", nil, cookies); rec.Code != http.StatusNotFound {
		t.Errorf("list without flag = %d, want 404", rec.Code)
	}
	var meta struct {
		Features map[string]bool `json:"features"`
	}
	_ = json.NewDecoder(do(plain, "GET", "/api/meta", nil, cookies).Body).Decode(&meta)
	if meta.Features["clone"] {
		t.Error("meta advertises clone without the flag")
	}

	// The flag cannot override a missing git.
	t.Setenv("PATH", t.TempDir())
	srv.cfg.AllowClone = true
	if srv.cloneAllowed() {
		t.Error("--allow-clone enabled clone without git")
	}
}

func TestIdleWaitsForRunningClone(t *testing.T) {
	srv, _ := testServer(t)
	srv.cfg.IdleTimeout = 20 * time.Millisecond
	srv.idleMu.Lock()
	srv.idleTimer = time.AfterFunc(time.Hour, func() {})
	srv.idleMu.Unlock()
	t.Cleanup(func() {
		srv.idleMu.Lock()
		srv.idleTimer.Stop()
		srv.idleMu.Unlock()
	})

	srv.clones.mu.Lock()
	srv.clones.running = true
	srv.clones.mu.Unlock()
	srv.onIdle()
	time.Sleep(120 * time.Millisecond)
	select {
	case <-srv.done:
		t.Fatal("idle timeout fired during a running clone")
	default:
	}

	srv.clones.mu.Lock()
	srv.clones.running = false
	srv.clones.mu.Unlock()
	waitFor(t, func() bool {
		select {
		case <-srv.done:
			return true
		default:
			return false
		}
	})
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition not reached in time")
}
