package highlight

import (
	"container/list"
	"crypto/sha256"
	"sync"
)

// SyntaxCache stores theme-independent documents by path and content hash.
// It is intentionally process-local; semantic results are not persisted.
type SyntaxCache struct {
	mu         sync.Mutex
	entries    map[string]*list.Element
	lru        *list.List
	maxEntries int
}

type syntaxCacheEntry struct {
	path     string
	hash     [32]byte
	doc      *Document
	parser   uint32
	enabled  bool
	theme    Theme
	maxBytes int
}

const syntaxParserVersion = 1

// NewSyntaxCache creates an empty cache with a bounded number of documents.
func NewSyntaxCache() *SyntaxCache { return NewSyntaxCacheWithCapacity(256) }

// NewSyntaxCacheWithCapacity creates a cache holding at most maxEntries
// documents. A non-positive capacity uses the default of 256.
func NewSyntaxCacheWithCapacity(maxEntries int) *SyntaxCache {
	if maxEntries <= 0 {
		maxEntries = 256
	}
	return &SyntaxCache{
		entries:    make(map[string]*list.Element),
		lru:        list.New(),
		maxEntries: maxEntries,
	}
}

// Get returns a cached document or parses content once for the current hash.
// Parsing deliberately happens outside the mutex: a large preview must not
// block another caller from invalidating or reading a different entry.
func (c *SyntaxCache) Get(path string, content []byte, options Options) *Document {
	hash := sha256.Sum256(content)
	maxBytes := options.MaxBytes
	if maxBytes <= 0 {
		maxBytes = defaultMaxBytes
	}

	c.mu.Lock()
	if c.entries == nil {
		c.entries = make(map[string]*list.Element)
	}
	if c.lru == nil {
		c.lru = list.New()
	}
	if element, ok := c.entries[path]; ok {
		entry := element.Value.(*syntaxCacheEntry)
		if entry.hash == hash && entry.parser == syntaxParserVersion && entry.enabled == options.Enabled && entry.theme == options.Theme && entry.maxBytes == maxBytes {
			c.lru.MoveToFront(element)
			doc := entry.doc
			c.mu.Unlock()
			return doc
		}
	}
	c.mu.Unlock()

	doc := Parse(path, content, options)

	c.mu.Lock()
	defer c.mu.Unlock()
	if element, ok := c.entries[path]; ok {
		entry := element.Value.(*syntaxCacheEntry)
		if entry.hash == hash && entry.parser == syntaxParserVersion && entry.enabled == options.Enabled && entry.theme == options.Theme && entry.maxBytes == maxBytes {
			c.lru.MoveToFront(element)
			return entry.doc
		}
		c.lru.Remove(element)
		delete(c.entries, path)
	}
	entry := &syntaxCacheEntry{path: path, hash: hash, doc: doc, parser: syntaxParserVersion, enabled: options.Enabled, theme: options.Theme, maxBytes: maxBytes}
	c.entries[path] = c.lru.PushFront(entry)
	for c.lru.Len() > c.maxEntries {
		oldest := c.lru.Back()
		if oldest == nil {
			break
		}
		c.lru.Remove(oldest)
		delete(c.entries, oldest.Value.(*syntaxCacheEntry).path)
	}
	return doc
}

// Invalidate removes one path from the cache.
func (c *SyntaxCache) Invalidate(path string) {
	c.mu.Lock()
	if element, ok := c.entries[path]; ok {
		c.lru.Remove(element)
		delete(c.entries, path)
	}
	c.mu.Unlock()
}
