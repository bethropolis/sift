package serve

import (
	"sync"
	"testing"
	"time"
)

// fakeClock lets the token store's TTL be tested without sleeping.
type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *fakeClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) advance(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

func TestLaunchTokenSingleUse(t *testing.T) {
	store := newLaunchTokens()
	tok, err := store.mint(true)
	if err != nil {
		t.Fatal(err)
	}
	app, ok := store.exchange(tok)
	if !ok || !app {
		t.Fatalf("first exchange = %v/%v", ok, app)
	}
	// Replay must fail.
	if _, ok := store.exchange(tok); ok {
		t.Fatal("replayed token was accepted")
	}
}

func TestLaunchTokenWrongAndEmpty(t *testing.T) {
	store := newLaunchTokens()
	tok, _ := store.mint(true)
	if _, ok := store.exchange("nope"); ok {
		t.Fatal("wrong token accepted")
	}
	if _, ok := store.exchange(""); ok {
		t.Fatal("empty token accepted")
	}
	// The real token still works after failures.
	if _, ok := store.exchange(tok); !ok {
		t.Fatal("valid token rejected after failures")
	}
}

func TestLaunchTokenExpiry(t *testing.T) {
	clock := &fakeClock{t: time.Unix(1_700_000_000, 0)}
	store := newLaunchTokens()
	store.now = clock.now
	tok, _ := store.mint(true)

	clock.advance(launchTokenTTL - time.Second)
	if _, ok := store.exchange(tok); !ok {
		t.Fatal("token expired early")
	}

	tok2, _ := store.mint(false)
	clock.advance(launchTokenTTL + time.Second)
	if _, ok := store.exchange(tok2); ok {
		t.Fatal("expired token accepted")
	}
}

func TestLaunchTokenAppFlag(t *testing.T) {
	store := newLaunchTokens()
	appTok, _ := store.mint(true)
	fallbackTok, _ := store.mint(false)
	if app, ok := store.exchange(appTok); !ok || !app {
		t.Fatal("app token lost its flag")
	}
	if app, ok := store.exchange(fallbackTok); !ok || app {
		t.Fatal("fallback token should not be an app session")
	}
}

func TestLaunchTokenConcurrentExchangeOneWinner(t *testing.T) {
	store := newLaunchTokens()
	tok, _ := store.mint(true)

	var wg sync.WaitGroup
	var mu sync.Mutex
	wins := 0
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, ok := store.exchange(tok); ok {
				mu.Lock()
				wins++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if wins != 1 {
		t.Fatalf("concurrent exchange winners = %d, want exactly 1", wins)
	}
}

func TestLaunchTokenPurgeKeepsStoreBounded(t *testing.T) {
	clock := &fakeClock{t: time.Unix(1_700_000_000, 0)}
	store := newLaunchTokens()
	store.now = clock.now
	for i := 0; i < 50; i++ {
		if _, err := store.mint(true); err != nil {
			t.Fatal(err)
		}
		clock.advance(launchTokenTTL + time.Second)
	}
	// Every mint prunes the previous batch, so nothing accumulates.
	store.mu.Lock()
	n := len(store.tokens)
	store.mu.Unlock()
	if n > 1 {
		t.Fatalf("store grew to %d entries; pruning is not keeping it bounded", n)
	}
}
