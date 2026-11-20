package scan

import (
	"container/list"
	"crypto/sha256"
	"sync"

	"github.com/bethropolis/sift/internal/lang"
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
	hash     [32]byte
	mode     Mode
	model    string
	language string
}

// cacheEntry is one LRU element.
type cacheEntry struct {
	key    cacheKey
	result CachedResult
}

// Default ContentCache bounds. Signatures and source slices are retained, so
// the byte cap matters more than the entry count on long watch sessions.
const (
	defaultCacheEntries  = 1024
	defaultCacheMaxBytes = 64 << 20 // 64 MiB
)

// ContentCache maps sha256(content) + processing options + language to
// computed token counts and signature summaries, so watch re-renders and
// repeated scans skip tree-sitter compression and BPE counting for unchanged
// files. It is safe for concurrent use by walker workers and bounded by an
// LRU over both entry count and retained bytes.
//
// Cached SigContent and Content slices are shared read-only: entries produced
// from a hit must never mutate them.
type ContentCache struct {
	mu         sync.Mutex
	entries    map[cacheKey]*list.Element
	lru        *list.List
	maxEntries int
	maxBytes   int64
	bytes      int64
}

func NewContentCache() *ContentCache {
	return NewContentCacheWithLimits(defaultCacheEntries, defaultCacheMaxBytes)
}

// NewContentCacheWithLimits builds a cache that evicts least-recently-used
// entries once either bound is exceeded. Non-positive limits use the defaults.
func NewContentCacheWithLimits(maxEntries int, maxBytes int64) *ContentCache {
	if maxEntries <= 0 {
		maxEntries = defaultCacheEntries
	}
	if maxBytes <= 0 {
		maxBytes = defaultCacheMaxBytes
	}
	return &ContentCache{
		entries:    make(map[cacheKey]*list.Element),
		lru:        list.New(),
		maxEntries: maxEntries,
		maxBytes:   maxBytes,
	}
}

// languageForPath returns the signature language a path maps to, without
// requiring the compressor (and therefore cgo) to be built. It keeps the cache
// key language-aware so identical bytes under different extensions cannot
// poison each other's SigContent/Language.
func languageForPath(path string) string {
	l, ok := lang.ForPath(path)
	if !ok {
		return ""
	}
	id := l.ID()
	if resolver, ok := l.(lang.SignatureResolver); ok {
		id = resolver.SignatureLanguage(path)
	}
	return string(id)
}

func (c *ContentCache) Get(content []byte, mode Mode, model string) (CachedResult, bool) {
	if c == nil {
		return CachedResult{}, false
	}
	return c.GetHashed(sha256.Sum256(content), mode, model, "")
}

// GetHashed looks up a precomputed content hash, letting callers hash once
// and reuse the key for Get+Put instead of paying sha256 twice per file.
func (c *ContentCache) GetHashed(hash [32]byte, mode Mode, model, language string) (CachedResult, bool) {
	if c == nil {
		return CachedResult{}, false
	}
	key := cacheKey{hash: hash, mode: mode, model: model, language: language}

	c.mu.Lock()
	defer c.mu.Unlock()
	element, ok := c.entries[key]
	if !ok {
		return CachedResult{}, false
	}
	c.lru.MoveToFront(element)
	return element.Value.(*cacheEntry).result, true
}

func (c *ContentCache) Put(content []byte, mode Mode, model string, result CachedResult) {
	if c == nil {
		return
	}
	c.PutHashed(sha256.Sum256(content), mode, model, "", result)
}

// PutHashed stores under a precomputed hash; see GetHashed.
func (c *ContentCache) PutHashed(hash [32]byte, mode Mode, model, language string, result CachedResult) {
	if c == nil {
		return
	}
	key := cacheKey{hash: hash, mode: mode, model: model, language: language}

	c.mu.Lock()
	defer c.mu.Unlock()
	if element, ok := c.entries[key]; ok {
		old := element.Value.(*cacheEntry)
		c.bytes -= entrySize(old.result)
		old.result = result
		c.bytes += entrySize(result)
		c.lru.MoveToFront(element)
	} else {
		element := c.lru.PushFront(&cacheEntry{key: key, result: result})
		c.entries[key] = element
		c.bytes += entrySize(result)
	}
	c.evictLocked()
}

// evictLocked drops least-recently-used entries until both bounds hold.
func (c *ContentCache) evictLocked() {
	for c.lru.Len() > c.maxEntries || c.bytes > c.maxBytes {
		oldest := c.lru.Back()
		if oldest == nil {
			return
		}
		entry := oldest.Value.(*cacheEntry)
		c.lru.Remove(oldest)
		delete(c.entries, entry.key)
		c.bytes -= entrySize(entry.result)
	}
	if c.bytes < 0 {
		c.bytes = 0
	}
}

func entrySize(r CachedResult) int64 {
	return int64(len(r.Content) + len(r.SigContent))
}
