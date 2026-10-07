// Package auth holds the serve authentication primitives: startup token or
// configured password verification, stateless signed session cookies, and
// login throttling. There is no session table and no background janitor;
// everything is verified cryptographically or pruned lazily on access.
package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// SessionLifetime is the cookie lifetime. Sessions are re-issued when less
// than half remains, so active users are never interrupted.
const SessionLifetime = 12 * time.Hour

// CookieName is the session cookie.
const CookieName = "sift_session"

const cookieVersion = "v1"

// lockoutBase is the first backoff step; each consecutive failure doubles it.
const lockoutBase = 2 * time.Second

// lockoutMax caps the per-IP backoff. Entries older than this are pruned
// lazily on access, so the map cannot grow without bound.
const lockoutMax = 15 * time.Minute

// globalMaxPerMinute caps login attempts across all IPs.
const globalMaxPerMinute = 60

// Auth verifies credentials and mints session cookies. The zero value is
// unusable; build one with NewToken or NewPassword.
type Auth struct {
	token    string
	password string

	key [32]byte

	mu       sync.Mutex
	attempts map[string]*attempt
	global   []time.Time
}

type attempt struct {
	failures   int
	lockedTill time.Time
}

// NewToken builds token auth (local mode): token is 256 bits of base64url.
func NewToken(token string) (*Auth, error) {
	if token == "" {
		return nil, fmt.Errorf("empty token")
	}
	return newAuth(token, ""), nil
}

// NewPassword builds password auth (remote mode).
func NewPassword(password string) (*Auth, error) {
	if len(password) < 12 {
		return nil, fmt.Errorf("password must be at least 12 characters")
	}
	return newAuth("", password), nil
}

func newAuth(token, password string) *Auth {
	a := &Auth{token: token, password: password, attempts: map[string]*attempt{}}
	if _, err := rand.Read(a.key[:]); err != nil {
		panic("auth: no randomness: " + err.Error())
	}
	return a
}

// SetPassword adds a secondary password credential (local mode with an
// explicitly configured password). Either credential unlocks a session.
func (a *Auth) SetPassword(password string) error {
	if len(password) < 12 {
		return fmt.Errorf("password must be at least 12 characters")
	}
	a.password = password
	return nil
}

// HasPassword reports whether password login is enabled.
func (a *Auth) HasPassword() bool { return a.password != "" }

// Token returns the startup token (local mode) for the login URL.
func (a *Auth) Token() string { return a.token }

// Kind reports "token" or "password" for banners and /api/meta. Local mode
// (which always has a token) reports token even when a password is also set.
func (a *Auth) Kind() string {
	if a.token != "" {
		return "token"
	}
	return "password"
}

// GenerateToken creates a 256-bit base64url startup token.
func GenerateToken() (string, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw[:]), nil
}

// VerifyToken checks a token-mode credential in constant time.
func (a *Auth) VerifyToken(candidate string) bool {
	if a.token == "" || candidate == "" {
		return false
	}
	got := hmacSHA256(a.key[:], []byte(candidate))
	want := hmacSHA256(a.key[:], []byte(a.token))
	return subtle.ConstantTimeCompare(got, want) == 1
}

// VerifyPassword checks a password-mode credential in constant time. The
// password itself is never persisted; only this process's keyed MAC exists.
func (a *Auth) VerifyPassword(candidate string) bool {
	if a.password == "" || candidate == "" {
		return false
	}
	got := hmacSHA256(a.key[:], []byte(candidate))
	want := hmacSHA256(a.key[:], []byte(a.password))
	return subtle.ConstantTimeCompare(got, want) == 1
}

func hmacSHA256(key, msg []byte) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write(msg)
	return mac.Sum(nil)
}

// Session cookies carry an explicit app-mode flag:
// v1.<expiry>.<app>.<nonce>.<HMAC>, where app is "1" for a session created by
// `sift serve --app` and "0" otherwise. The claim lives inside the signed body,
// so the server stays stateless (no session table). It needs its own field
// rather than a nonce prefix because the cookie body is dot-delimited and a
// base64url nonce already contains dashes and underscores.

// MintCookie issues a stateless signed session cookie.
// Restarting the server invalidates all sessions because the key is
// per-process.
func (a *Auth) MintCookie() *http.Cookie {
	return a.MintSessionCookie(false)
}

// MintSessionCookie issues a session cookie, marking it as an app-mode
// session when app is true.
func (a *Auth) MintSessionCookie(app bool) *http.Cookie {
	return a.mintCookieAt(time.Now().Add(SessionLifetime), app)
}

func (a *Auth) mintCookieAt(expiry time.Time, app bool) *http.Cookie {
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		panic("auth: no randomness: " + err.Error())
	}
	flag := "0"
	if app {
		flag = "1"
	}
	return a.cookieFor(expiry, flag, base64.RawURLEncoding.EncodeToString(nonce[:]))
}

func (a *Auth) cookieFor(expiry time.Time, appFlag, nonce string) *http.Cookie {
	body := cookieVersion + "." + strconv.FormatInt(expiry.Unix(), 10) + "." + appFlag + "." + nonce
	sig := base64.RawURLEncoding.EncodeToString(hmacSHA256(a.key[:], []byte(body)))
	return &http.Cookie{
		Name:     CookieName,
		Value:    body + "." + sig,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(SessionLifetime.Seconds()),
	}
}

// ValidSession reports whether r carries a live session cookie, and whether
// the cookie should be re-issued (less than half the lifetime remains).
func (a *Auth) ValidSession(r *http.Request) (ok, reissue bool) {
	ok, _, reissue = a.SessionState(r)
	return ok, reissue
}

// SessionState reports whether r carries a live session cookie, whether that
// session was created by an app-mode launch, and whether the cookie should be
// re-issued. The app flag is authenticated: it is part of the signed body.
func (a *Auth) SessionState(r *http.Request) (ok, app, reissue bool) {
	c, err := r.Cookie(CookieName)
	if err != nil {
		return false, false, false
	}
	parts := strings.Split(c.Value, ".")
	if len(parts) != 5 || parts[0] != cookieVersion {
		return false, false, false
	}
	body := strings.Join(parts[:4], ".")
	sig, err := base64.RawURLEncoding.DecodeString(parts[4])
	if err != nil {
		return false, false, false
	}
	want := hmacSHA256(a.key[:], []byte(body))
	if subtle.ConstantTimeCompare(sig, want) != 1 {
		return false, false, false
	}
	expiryUnix, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return false, false, false
	}
	expiry := time.Unix(expiryUnix, 0)
	now := time.Now()
	if now.After(expiry) {
		return false, false, false
	}
	return true, parts[2] == "1", expiry.Sub(now) < SessionLifetime/2
}

// SecureCookie marks the session cookie Secure for TLS/proxied responses.
func SecureCookie(c *http.Cookie) {
	c.Secure = true
}

// ClientIP extracts the client IP: RemoteAddr always, X-Forwarded-For only
// behind a trusted proxy.
func ClientIP(r *http.Request, behindProxy bool) string {
	if behindProxy {
		if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
			if comma := strings.IndexByte(fwd, ','); comma >= 0 {
				fwd = fwd[:comma]
			}
			if ip := strings.TrimSpace(fwd); ip != "" {
				return ip
			}
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// CheckLogin enforces throttling before a credential check. It returns nil
// when the attempt may proceed, or the seconds to wait with ok=false.
func (a *Auth) CheckLogin(ip string) (retryAfter int, ok bool) {
	now := time.Now()
	a.mu.Lock()
	defer a.mu.Unlock()

	// Lazy prune: drop quiet entries and stale global marks. No timers.
	for key, at := range a.attempts {
		if now.After(at.lockedTill.Add(lockoutMax)) {
			delete(a.attempts, key)
		}
	}
	window := now.Add(-time.Minute)
	kept := a.global[:0]
	for _, t := range a.global {
		if t.After(window) {
			kept = append(kept, t)
		}
	}
	a.global = kept
	if len(a.global) >= globalMaxPerMinute {
		return 60, false
	}

	if at, found := a.attempts[ip]; found && now.Before(at.lockedTill) {
		return int(at.lockedTill.Sub(now).Seconds()) + 1, false
	}
	return 0, true
}

// RecordResult updates throttling after an attempt and returns the backoff
// for a failure (0 when the failure is not yet throttled).
func (a *Auth) RecordResult(ip string, success bool) (retryAfter int) {
	now := time.Now()
	a.mu.Lock()
	defer a.mu.Unlock()

	a.global = append(a.global, now)
	if success {
		delete(a.attempts, ip)
		return 0
	}
	at := a.attempts[ip]
	if at == nil {
		at = &attempt{}
		a.attempts[ip] = at
	}
	at.failures++
	backoff := lockoutBase << (at.failures - 1)
	if backoff <= 0 || backoff > lockoutMax {
		backoff = lockoutMax
	}
	at.lockedTill = now.Add(backoff)
	if at.failures == 1 {
		return 0 // first failure answers immediately; the second waits
	}
	return int(backoff.Seconds())
}
