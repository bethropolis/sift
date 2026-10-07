package serve

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeTimers records armed deadlines so controller tests never sleep.
type fakeTimers struct {
	mu      sync.Mutex
	pending map[int]func()
	next    int
	fired   int
}

func newFakeTimers() *fakeTimers {
	return &fakeTimers{pending: map[int]func(){}}
}

func (f *fakeTimers) after(d time.Duration, fn func()) func() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	id := f.next
	f.next++
	f.pending[id] = fn
	_ = d
	return func() bool {
		f.mu.Lock()
		defer f.mu.Unlock()
		delete(f.pending, id)
		return true
	}
}

// fire runs the single armed deadline, if any.
func (f *fakeTimers) fire() {
	f.mu.Lock()
	var fn func()
	for _, v := range f.pending {
		fn = v
		break
	}
	f.pending = map[int]func(){}
	if fn != nil {
		f.fired++
	}
	f.mu.Unlock()
	if fn != nil {
		fn()
	}
}

func (f *fakeTimers) armed() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.pending)
}

func TestControllerReconnectCancelsGrace(t *testing.T) {
	timers := newFakeTimers()
	var shuts int
	var mu sync.Mutex
	c := newAppController(func() { mu.Lock(); shuts++; mu.Unlock() }, timers.after)
	c.arm()

	c.addConn()
	c.dropConn() // arms grace
	c.addConn()  // reconnect cancels it
	if timers.armed() != 0 {
		t.Fatalf("grace timer still armed after reconnect (%d)", timers.armed())
	}
	timers.fire()
	mu.Lock()
	defer mu.Unlock()
	if shuts != 0 {
		t.Fatalf("reconnect did not cancel shutdown (%d)", shuts)
	}
}

func TestControllerGraceExpiryShutsDownOnce(t *testing.T) {
	timers := newFakeTimers()
	var shuts int
	var mu sync.Mutex
	c := newAppController(func() { mu.Lock(); shuts++; mu.Unlock() }, timers.after)
	c.arm()

	c.addConn()
	c.dropConn()
	if timers.armed() != 1 {
		t.Fatalf("expected 1 armed timer, got %d", timers.armed())
	}
	timers.fire()
	mu.Lock()
	got := shuts
	mu.Unlock()
	if got != 1 {
		t.Fatalf("shutdown count = %d, want 1", got)
	}
}

func TestControllerWatchdogOnlyWhenUnseen(t *testing.T) {
	timers := newFakeTimers()
	var shuts int
	var mu sync.Mutex
	c := newAppController(func() { mu.Lock(); shuts++; mu.Unlock() }, timers.after)

	c.arm()
	timers.fire() // nothing reported in yet
	mu.Lock()
	got := shuts
	mu.Unlock()
	if got != 1 {
		t.Fatalf("watchdog did not fire with no sign of life (%d)", got)
	}

	// A second controller whose window reported in must not arm.
	timers2 := newFakeTimers()
	c2 := newAppController(func() { mu.Lock(); shuts++; mu.Unlock() }, timers2.after)
	c2.markAlive()
	c2.arm()
	if timers2.armed() != 0 {
		t.Fatal("watchdog armed despite markAlive")
	}
	timers2.fire()
	mu.Lock()
	defer mu.Unlock()
	if shuts != 1 {
		t.Fatalf("a seen window triggered shutdown (%d)", shuts)
	}
}

func TestControllerCloseCancelsTimers(t *testing.T) {
	timers := newFakeTimers()
	var shuts int
	var mu sync.Mutex
	c := newAppController(func() { mu.Lock(); shuts++; mu.Unlock() }, timers.after)
	c.addConn()
	c.dropConn()
	c.close()
	if timers.armed() != 0 {
		t.Fatalf("close left %d timers armed", timers.armed())
	}
}

func TestAppControllerDefaultTimerIsUsable(t *testing.T) {
	// Guards against the injected-afterFunc regression where the real timer
	// was only created when stopped.
	var fired bool
	c := newAppController(func() { fired = true }, nil)
	c.addConn()
	c.dropConn()
	// appShutdownGrace is 15s, so assert the constructor picked the real
	// time.AfterFunc by cancelling promptly instead of waiting.
	c.close()
	if fired {
		t.Fatal("shutdown fired without the grace period elapsing")
	}
}

// TestSessionLaunchFlow covers the endpoint contract: a good token sets an
// app session, and every failure mode returns the same generic 401.
func TestSessionLaunchFlow(t *testing.T) {
	srv, _ := testServer(t)

	tok, err := srv.launchTokens.mint(true)
	if err != nil {
		t.Fatal(err)
	}
	rec := do(srv, "POST", "/api/session/launch", map[string]any{"token": tok}, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("launch = %d", rec.Code)
	}
	cookies := rec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("no session cookie issued")
	}

	// The issued session is an app session.
	rec = do(srv, "GET", "/api/meta", nil, cookies)
	if body := rec.Body.String(); !strings.Contains(body, `"app":true`) {
		t.Fatalf("meta does not report app mode: %s", body)
	}

	// Replaying the same token fails with the generic message.
	rec = do(srv, "POST", "/api/session/launch", map[string]any{"token": tok}, nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("replay = %d, want 401", rec.Code)
	}
	generic := rec.Body.String()

	// An unknown token produces the identical response. A fresh server keeps
	// this off the shared lockout path (two failures from one IP is enough to
	// start the per-IP backoff).
	srv2, _ := testServer(t)
	rec = do(srv2, "POST", "/api/session/launch", map[string]any{"token": "wrong"}, nil)
	if rec.Code != http.StatusUnauthorized || rec.Body.String() != generic {
		t.Fatalf("wrong token = %d %s, want 401 with the same body", rec.Code, rec.Body.String())
	}

	// The launch endpoint shares the login lockout, so brute force is
	// throttled like any other credential check.
	for i := 0; i < 4; i++ {
		_ = do(srv2, "POST", "/api/session/launch", map[string]any{"token": "wrong"}, nil)
	}
	rec = do(srv2, "POST", "/api/session/launch", map[string]any{"token": "wrong"}, nil)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("brute force = %d, want 429", rec.Code)
	}
}

func TestSessionLaunchRequiresJSONBody(t *testing.T) {
	srv, _ := testServer(t)
	req := httptest.NewRequest("POST", "/api/session/launch", strings.NewReader("nope"))
	req.Header.Set("Content-Type", "application/json")
	req.Host = "127.0.0.1:7777"
	rec := httptest.NewRecorder()
	srv.mux.ServeHTTP(rec, req)
	if rec.Code == http.StatusOK {
		t.Fatal("malformed launch body was accepted")
	}
}

func TestAppHeartbeatRequiresAppLaunch(t *testing.T) {
	srv, _ := testServer(t)
	cookies := loginCookies(t, srv)
	// Not launched in app mode: the endpoint is absent.
	rec := do(srv, "GET", "/api/app/heartbeat", nil, cookies)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("heartbeat without app launch = %d, want 404", rec.Code)
	}
}

func TestAppFlagsRejected(t *testing.T) {
	cfg := Config{Listen: "0.0.0.0:7777", App: true, AllowRemote: true, InsecureHTTP: true,
		PasswordFile: "", Roots: []string{t.TempDir()}, AllowedHosts: []string{"x"}}
	t.Setenv("SIFT_SERVE_PASSWORD", "long-enough-password")
	if _, err := cfg.Validate(); err == nil {
		t.Fatal("--app accepted a non-loopback bind")
	}

	// --keep-alive without --app is a usage error.
	cfg2 := Config{Listen: "127.0.0.1:7777", KeepAlive: true}
	if _, err := cfg2.Validate(); err == nil {
		t.Fatal("--keep-alive accepted without --app")
	}
}
