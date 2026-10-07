package serve

import (
	"net/http"
	"strconv"
	"time"

	"github.com/bethropolis/sift/internal/serve/auth"
)

func itoa(n int) string { return strconv.Itoa(n) }

// appQuitGrace lets the 202 response reach the browser before the listener
// goes away.
const appQuitGrace = 50 * time.Millisecond

// launchRequestHeader must be present on app-control POSTs. Same-origin
// fetch with a custom header is not preflighted, and the header makes a
// cross-site trigger (which cannot set it without a successful CORS
// preflight, and this server never answers one) impossible.
const launchRequestHeader = "X-Sift-Request"

// handleSessionLaunch exchanges a one-time launch token for a normal session,
// so an app window logs itself in without the password ever touching a
// command line. The token is single-use with a 60s TTL; every failure returns
// the same generic 401.
func (s *Server) handleSessionLaunch(w http.ResponseWriter, r *http.Request) {
	// Tight body cap: the payload is one short token.
	r.Body = http.MaxBytesReader(w, r.Body, 1<<10)
	var body struct {
		Token string `json:"token"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	// Same lockout path as password login, so a local attacker cannot brute
	// force tokens without inheriting the per-IP backoff.
	if retryAfter, allowed := s.auth.CheckLogin(auth.ClientIP(r, s.cfg.BehindProxy)); !allowed {
		w.Header().Set("Retry-After", itoa(retryAfter))
		writeAPIError(w, http.StatusTooManyRequests, "too many attempts")
		return
	}

	app, ok := s.launchTokens.exchange(body.Token)
	if !ok {
		// Unknown, expired and reused tokens are indistinguishable on purpose.
		s.auth.RecordResult(auth.ClientIP(r, s.cfg.BehindProxy), false)
		writeAPIError(w, http.StatusUnauthorized, "invalid or expired launch token")
		return
	}
	s.auth.RecordResult(auth.ClientIP(r, s.cfg.BehindProxy), true)
	// The window is demonstrably alive, so the launch watchdog is satisfied
	// even if the heartbeat never connects.
	s.appCtl.markAlive()

	cookie := s.auth.MintSessionCookie(app)
	s.secureCookie(cookie)
	http.SetCookie(w, cookie)
	s.log.Info("launch token exchanged", "app", app)
	writeJSON(w, http.StatusOK, map[string]any{"app": app})
}

// handleAppHeartbeat keeps the liveness stream open for the duration of the
// window's life. Registered only in app mode; the connection count drives the
// graceful shutdown in appCtl.
func (s *Server) handleAppHeartbeat(w http.ResponseWriter, r *http.Request) {
	if !s.appLaunched {
		writeAPIError(w, http.StatusNotFound, "not available")
		return
	}
	appAliveStream(r, w, s.appCtl, s.done)
}

// handleAppQuit stops the server and, with it, the window. Requires an app
// session, the custom request header, and the origin checks apiChain already
// applies, so no other client can trigger it.
func (s *Server) handleAppQuit(w http.ResponseWriter, r *http.Request) {
	ok, app, _ := s.auth.SessionState(r)
	if !ok || !app {
		writeAPIError(w, http.StatusForbidden, "not an app session")
		return
	}
	if r.Header.Get(launchRequestHeader) != "1" {
		writeAPIError(w, http.StatusForbidden, "missing request header")
		return
	}
	w.WriteHeader(http.StatusAccepted)
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
	s.log.Info("app quit requested")
	go func() {
		time.Sleep(appQuitGrace)
		s.shutdown()
	}()
}
