package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestTokenVerify(t *testing.T) {
	a, err := NewToken("secret-token")
	if err != nil {
		t.Fatal(err)
	}
	if !a.VerifyToken("secret-token") {
		t.Fatal("valid token rejected")
	}
	if a.VerifyToken("wrong") {
		t.Fatal("wrong token accepted")
	}
	if a.VerifyPassword("secret-token") {
		t.Fatal("token accepted as password")
	}
}

func TestPasswordMinLength(t *testing.T) {
	if _, err := NewPassword("short"); err == nil {
		t.Fatal("short password accepted")
	}
	a, err := NewPassword("long-enough-password")
	if err != nil {
		t.Fatal(err)
	}
	if !a.VerifyPassword("long-enough-password") {
		t.Fatal("valid password rejected")
	}
	if a.VerifyPassword("long-enough-passwor") {
		t.Fatal("wrong password accepted")
	}
}

func TestCookieRoundTrip(t *testing.T) {
	a, _ := NewToken("x")
	c := a.MintCookie()
	if !c.HttpOnly || c.SameSite != http.SameSiteStrictMode || c.Path != "/" {
		t.Fatalf("weak cookie attributes: %+v", c)
	}
	if c.Secure {
		t.Fatal("cookie must not be Secure by default")
	}

	r := httptest.NewRequest("GET", "/", nil)
	r.AddCookie(c)
	ok, reissue := a.ValidSession(r)
	if !ok || reissue {
		t.Fatalf("fresh cookie invalid (ok=%v reissue=%v)", reissue, ok)
	}
}

func TestCookieExpiryAndReissue(t *testing.T) {
	a, _ := NewToken("x")

	expired := a.mintCookieAt(time.Now().Add(-time.Minute))
	r := httptest.NewRequest("GET", "/", nil)
	r.AddCookie(expired)
	if ok, _ := a.ValidSession(r); ok {
		t.Fatal("expired cookie accepted")
	}

	old := a.mintCookieAt(time.Now().Add(time.Hour)) // < half of 12h
	r2 := httptest.NewRequest("GET", "/", nil)
	r2.AddCookie(old)
	if ok, reissue := a.ValidSession(r2); !ok || !reissue {
		t.Fatalf("old cookie should reissue (ok=%v reissue=%v)", ok, reissue)
	}
}

func TestCookieTamper(t *testing.T) {
	a, _ := NewToken("x")
	c := a.MintCookie()

	for _, mutate := range []func(string) string{
		func(v string) string { return v + "x" },
		func(v string) string { return strings.Replace(v, "v1.", "v2.", 1) },
		func(v string) string { return "garbage" },
	} {
		r := httptest.NewRequest("GET", "/", nil)
		tampered := *c
		tampered.Value = mutate(c.Value)
		r.AddCookie(&tampered)
		if ok, _ := a.ValidSession(r); ok {
			t.Fatalf("tampered cookie accepted: %q", tampered.Value)
		}
	}

	// A cookie from another process (different key) must fail.
	other, _ := NewToken("x")
	r := httptest.NewRequest("GET", "/", nil)
	r.AddCookie(other.MintCookie())
	// Same token string but different key: MAC differs.
	a2req := r
	if ok, _ := a.ValidSession(a2req); ok {
		t.Fatal("foreign-process cookie accepted")
	}
}

func TestLoginThrottling(t *testing.T) {
	a, _ := NewToken("x")
	ip := "10.0.0.1"

	if _, ok := a.CheckLogin(ip); !ok {
		t.Fatal("first attempt throttled")
	}
	if wait := a.RecordResult(ip, false); wait != 0 {
		t.Fatalf("first failure should answer immediately, got %ds", wait)
	}
	// The first failure still arms a short lockout for the next attempt.
	if _, ok := a.CheckLogin(ip); ok {
		t.Fatal("attempt during lockout admitted")
	}
	// Fast-forward past it and fail again: now the backoff is reported.
	a.mu.Lock()
	a.attempts[ip].lockedTill = time.Now().Add(-time.Second)
	a.mu.Unlock()
	if _, ok := a.CheckLogin(ip); !ok {
		t.Fatal("expired lockout still enforced")
	}
	wait := a.RecordResult(ip, false)
	if wait <= 0 {
		t.Fatal("second failure should impose backoff")
	}
	if retry, ok := a.CheckLogin(ip); ok || retry <= 0 {
		t.Fatalf("locked IP admitted (ok=%v retry=%d)", ok, retry)
	}
	// Success clears the lockout.
	a.mu.Lock()
	a.attempts[ip].lockedTill = time.Now().Add(-time.Second)
	a.mu.Unlock()
	if _, ok := a.CheckLogin(ip); !ok {
		t.Fatal("expired lockout still enforced")
	}
	if a.RecordResult(ip, true) != 0 {
		t.Fatal("success should not impose backoff")
	}
	if _, ok := a.CheckLogin(ip); !ok {
		t.Fatal("cleared IP throttled")
	}
}

func TestClientIP(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "192.0.2.1:1234"
	r.Header.Set("X-Forwarded-For", "203.0.113.9")
	if got := ClientIP(r, false); got != "192.0.2.1" {
		t.Fatalf("direct IP = %q", got)
	}
	if got := ClientIP(r, true); got != "203.0.113.9" {
		t.Fatalf("proxied IP = %q", got)
	}
}
