package serve

import (
	"bufio"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/bethropolis/sift/internal/serve/auth"
	"github.com/bethropolis/sift/internal/serve/qr"
)

// pairToken is the single live pairing credential. Only its SHA-256 hash is
// retained, so a heap dump reveals no usable token.
type pairToken struct {
	hash    [32]byte
	expires time.Time
}

// pairTokens holds at most one live token. Minting replaces (and therefore
// invalidates) the previous one. Every operation prunes lazily; there is no
// janitor goroutine.
type pairTokens struct {
	mu      sync.Mutex
	current *pairToken
	now     func() time.Time
}

func newPairTokens() *pairTokens {
	return &pairTokens{now: time.Now}
}

func (p *pairTokens) clock() time.Time {
	if p.now == nil {
		return time.Now()
	}
	return p.now()
}

// mint creates a 256-bit base64url token and makes it the only live token.
func (p *pairTokens) mint(ttl time.Duration) (string, error) {
	if ttl <= 0 {
		return "", fmt.Errorf("pair token TTL must be positive")
	}
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("mint pair token: %w", err)
	}
	value := base64.RawURLEncoding.EncodeToString(raw[:])
	token := &pairToken{hash: sha256.Sum256([]byte(value)), expires: p.clock().Add(ttl)}
	p.mu.Lock()
	p.current = token
	p.mu.Unlock()
	return value, nil
}

// exchange consumes the token if it is live and matches. Unknown, expired,
// replayed and malformed values take the same constant-time path. The entry
// is cleared before returning, so a replay always fails.
func (p *pairTokens) exchange(candidate string) bool {
	got := sha256.Sum256([]byte(candidate))
	p.mu.Lock()
	defer p.mu.Unlock()
	now := p.clock()
	if p.current != nil && !now.Before(p.current.expires) {
		p.current = nil
	}
	var want [32]byte
	if p.current != nil {
		want = p.current.hash
	}
	match := subtle.ConstantTimeCompare(got[:], want[:]) == 1
	if !match || p.current == nil {
		return false
	}
	p.current = nil
	return true
}

// handleSessionPair exchanges a one-time QR token for a read-only session.
// Every credential failure answers with the same generic 401; throttling
// failures use 429 so a phone can show a retry hint.
func (s *Server) handleSessionPair(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.Pair {
		http.NotFound(w, r)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4<<10)
	var body struct {
		Token string `json:"token"`
	}
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&body); err != nil {
		body.Token = ""
	}
	ip := auth.ProxiedClientIP(r, s.cfg.BehindProxy)
	if retry, ok := s.auth.CheckLogin(ip); !ok {
		w.Header().Set("Retry-After", itoa(retry))
		writeAPIError(w, http.StatusTooManyRequests, "too many attempts")
		return
	}
	if !s.pairTokens.exchange(body.Token) {
		wait := s.auth.RecordResult(ip, false)
		s.log.Warn("pairing failed", "ip", ip)
		if wait > 0 {
			w.Header().Set("Retry-After", itoa(wait))
			writeAPIError(w, http.StatusTooManyRequests, "too many attempts")
			return
		}
		writeAPIError(w, http.StatusUnauthorized, "invalid or expired pairing token")
		return
	}
	s.auth.RecordResult(ip, true)
	cookie, ok := s.auth.MintPairCookie(s.cfg.PairSessionTTL, s.cfg.PairMaxSessions)
	if !ok {
		// The token was valid, but the paired-session cap is full. The phone
		// must mint a fresh QR to try again.
		s.log.Warn("pairing session cap reached", "ip", ip, "cap", s.cfg.PairMaxSessions)
		writeAPIError(w, http.StatusUnauthorized, "invalid or expired pairing token")
		return
	}
	s.secureCookie(cookie, r)
	http.SetCookie(w, cookie)
	s.log.Info("pairing session created", "ip", ip)
	w.WriteHeader(http.StatusNoContent)
}

// pairBaseURL returns the external base URL used in the QR. It is either the
// explicit --public-url or, with --insecure-http, a reachable LAN address.
func (s *Server) pairBaseURL(scheme, addr string) string {
	if s.cfg.PublicURL != "" {
		return strings.TrimRight(s.cfg.PublicURL, "/")
	}
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return ""
	}
	if host == "" || isLoopbackHost(host) {
		if addrs, err := net.InterfaceAddrs(); err == nil {
			if ips := lanIPv4s(addrs); len(ips) > 0 {
				host = ips[0]
			}
		}
	}
	if host == "" {
		host = "127.0.0.1"
	}
	if scheme == "" {
		scheme = "http"
	}
	return scheme + "://" + net.JoinHostPort(host, port)
}

// pairURL builds the fragment-bearing login URL. The token never travels in
// the query string, so proxies and server logs never see it.
func (s *Server) pairURL(token string) string {
	base := s.pairBase
	if base == "" {
		return ""
	}
	return base + "/#pair=" + token
}

// mintAndPrintPairing replaces any live token and prints a fresh QR. It is
// called once at startup and again on Enter/SIGUSR1.
func (s *Server) mintAndPrintPairing() bool {
	token, err := s.pairTokens.mint(s.cfg.PairTTL)
	if err != nil {
		s.log.Error("cannot mint pairing token", "err", err)
		return false
	}
	u := s.pairURL(token)
	if u == "" {
		s.log.Error("cannot build pairing URL")
		return false
	}
	if code, err := qr.Encode(u, qr.M); err == nil {
		fmt.Fprint(os.Stderr, code.Terminal())
	} else {
		s.log.Warn("cannot render pairing QR", "err", err)
	}
	fmt.Fprintf(os.Stderr, "\n  %s %s\n", serveDim.Sprint("pair →"), serveURL.Sprint(u))
	fmt.Fprintf(os.Stderr, "  %s\n", serveDim.Sprintf("read-only · token expires in %s · press Enter for a new QR", s.cfg.PairTTL))
	if s.cfg.InsecureHTTP {
		fmt.Fprintf(os.Stderr, "  %s\n", serveWarn.Sprint("plain HTTP: Android Chrome needs HTTPS for PWA install and Share"))
	}
	return true
}

// startPairInput starts the single stdin reader that re-mints on Enter when
// the process has a terminal. Line-buffered input is the contract; the
// standard library has no raw-mode helper, and a single goroutine is all this
// needs. Without --pair this function is never called.
func (s *Server) startPairInput() {
	info, err := os.Stdin.Stat()
	if err != nil || info.Mode()&os.ModeCharDevice == 0 {
		return
	}
	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			s.mintAndPrintPairing()
		}
	}()
}
