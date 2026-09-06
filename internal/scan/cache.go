package scan

import (
	"crypto/sha256"
	"sync"
)

// CachedResult is the expensive half of Process: the enriched fields a
// cache hit replays verbatim so a hit is indistinguishable from a miss.
// Content aliases the caller's bytes (or the one-off compressed output);
// like SigContent it is shared read-only and must never be mutated.
type CachedResult struct {
	Content      []byte
	Tokens       int
	TokensFull   int
	SigContent   []byte
	TokensSig    int
	IsCompressed bool
	Language     string
}

type cacheKey struct {
	hash  [32]byte
	mode  Mode
	model string
}

// ContentCache maps sha256(content) + processing options to computed token
// counts and signature summaries, so watch re-renders and repeated scans
// skip tree-sitter compression and BPE counting for unchanged files. It is
// safe for concurrent use by walker workers.
//
// Cached SigContent and Content slices are shared read-only: entries
// produced from a hit must never mutate them. The key includes the tokenizer
// model so one cache is never poisoned by counts from another encoding.
type ContentCache struct {
	mu      sync.RWMutex
	entries map[cacheKey]CachedResult
}

func NewContentCache() *ContentCache {
	return &ContentCache{
		entries: make(map[cacheKey]CachedResult),
	}
}

func (c *ContentCache) Get(content []byte, mode Mode, model string) (CachedResult, bool) {
	if c == nil {
		return CachedResult{}, false
	}
	key := cacheKey{
		hash:  sha256.Sum256(content),
		mode:  mode,
		model: model,
	}
	c.mu.RLock()
	res, ok := c.entries[key]
	c.mu.RUnlock()
	return res, ok
}

func (c *ContentCache) Put(content []byte, mode Mode, model string, result CachedResult) {
	if c == nil {
		return
	}
	key := cacheKey{
		hash:  sha256.Sum256(content),
		mode:  mode,
		model: model,
	}
	c.mu.Lock()
	c.entries[key] = result
	c.mu.Unlock()
}
