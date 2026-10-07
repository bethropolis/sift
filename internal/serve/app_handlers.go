package serve

import (
	"net/http"
	"strconv"

	"github.com/bethropolis/sift/internal/serve/auth"
)

func itoa(n int) string { return strconv.Itoa(n) }

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
