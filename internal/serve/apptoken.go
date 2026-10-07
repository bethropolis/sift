// apptoken.go holds the one-time tokens that let an app-mode window log
// itself in. A token is 256 bits of crypto/rand, base64url encoded, lives in
// memory only, expires in a minute, and is destroyed the first time it is
// exchanged.
//
// The token travels in a URL fragment, so it never reaches server logs or a
// Referer header. It is briefly visible in the browser's argv; single use and
// a short TTL are what make that acceptable.
package serve

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"sync"
	"time"
)

// launchTokenTTL is deliberately short: the browser is already running by the
// time the URL is handed to it.
const launchTokenTTL = 60 * time.Second

// launchToken is a pending auto-login credential.
type launchToken struct {
	value   string
	expires time.Time
	// app marks a token minted for the chromeless window. Fallback launches
	// get app=false so they auto-login without gaining the Quit button or the
	// window-liveness shutdown.
	app bool
}

// launchTokens is a tiny in-memory token store. It holds at most a handful of
// entries (one per recent launch) and is pruned lazily on every mint and
// exchange, so no janitor goroutine is needed.
type launchTokens struct {
	mu     sync.Mutex
	tokens []launchToken
	// now is injectable so expiry is testable without sleeping.
	now func() time.Time
}

func newLaunchTokens() *launchTokens {
	return &launchTokens{now: time.Now}
}

func (t *launchTokens) clock() time.Time {
	if t.now == nil {
		return time.Now()
	}
	return t.now()
}

// mint creates a new single-use token and returns it.
func (t *launchTokens) mint(app bool) (string, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("mint launch token: %w", err)
	}
	value := base64.RawURLEncoding.EncodeToString(raw[:])
	t.mu.Lock()
	defer t.mu.Unlock()
	t.pruneLocked()
	t.tokens = append(t.tokens, launchToken{value: value, expires: t.clock().Add(launchTokenTTL), app: app})
	return value, nil
}

// exchange consumes a token if it exists and matches in constant time. The
// entry is removed before the result is returned, so a replay always fails.
// Expired entries were already dropped by pruneLocked, which is why unknown,
// expired and reused tokens all take the same path and return the same thing.
func (t *launchTokens) exchange(candidate string) (app bool, ok bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.pruneLocked()
	if candidate == "" {
		return false, false
	}
	for i, tk := range t.tokens {
		// Constant-time compare so a wrong token costs the same as an
		// expired one.
		if subtle.ConstantTimeCompare([]byte(tk.value), []byte(candidate)) != 1 {
			continue
		}
		t.tokens = append(t.tokens[:i], t.tokens[i+1:]...)
		return tk.app, true
	}
	return false, false
}

// pruneLocked drops expired entries. Callers must hold the mutex.
func (t *launchTokens) pruneLocked() {
	now := t.clock()
	kept := t.tokens[:0]
	for _, tk := range t.tokens {
		if now.Before(tk.expires) {
			kept = append(kept, tk)
		}
	}
	t.tokens = kept
}
