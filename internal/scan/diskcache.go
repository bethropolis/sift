package scan

import (
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync/atomic"
	"time"

	"github.com/bethropolis/sift/internal/secrets"
)

// diskCacheVersion invalidates persisted entries whenever the stored shape or
// parser/tokenizer assumptions change.
const diskCacheVersion = 2

// diskCacheMaxBytes bounds the cache directory. Pruning runs lazily on writes.
const diskCacheMaxBytes int64 = 128 << 20

// diskEntry is the persisted subset of CachedResult. Raw source is never
// written; signatures are omitted when secret scanning finds a credential.
type diskEntry struct {
	Version      int    `json:"v"`
	Tokens       int    `json:"tokens"`
	TokensFull   int    `json:"tokens_full"`
	TokensSig    int    `json:"tokens_sig"`
	IsCompressed bool   `json:"compressed"`
	Language     string `json:"language"`
	SigContent   []byte `json:"sig,omitempty"`
}

// DiskCache persists token counts and signature summaries under a directory.
// Every I/O or decode failure is treated as a miss; a scan never fails because
// of caching.
type DiskCache struct {
	dir      string
	maxBytes int64
	version  int
	puts     atomic.Int64
	scanner  *secrets.Scanner
}

// NewDiskCache creates a cache rooted at dir. A non-positive maxBytes uses the
// default cap.
func NewDiskCache(dir string, maxBytes int64) *DiskCache {
	if maxBytes <= 0 {
		maxBytes = diskCacheMaxBytes
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil
	}
	return &DiskCache{dir: dir, maxBytes: maxBytes, version: diskCacheVersion}
}

// NewDiskCacheWithVersion is NewDiskCache with an explicit format version,
// used by tests to prove that upgrades invalidate old entries.
func NewDiskCacheWithVersion(dir string, maxBytes int64, version int) *DiskCache {
	d := NewDiskCache(dir, maxBytes)
	if d != nil {
		d.version = version
	}
	return d
}

// DefaultDiskCache returns the per-user cache under os.UserCacheDir, or nil
// when no cache directory is available.
func DefaultDiskCache() *DiskCache {
	base, err := os.UserCacheDir()
	if err != nil || base == "" {
		return nil
	}
	return NewDiskCache(filepath.Join(base, "sift", "cache"), diskCacheMaxBytes)
}

// Enabled reports whether the cache has a usable directory.
func (d *DiskCache) Enabled() bool { return d != nil && d.dir != "" }

// Get returns a persisted entry. Missing, corrupt, or version-mismatched files
// are misses.
func (d *DiskCache) Get(hash [32]byte, mode Mode, model, language string) (CachedResult, bool) {
	if !d.Enabled() {
		return CachedResult{}, false
	}
	path := filepath.Join(d.dir, diskKey(hash, mode, model, language, d.version))
	f, err := os.Open(path)
	if err != nil {
		return CachedResult{}, false
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		return CachedResult{}, false
	}
	var entry diskEntry
	if err := json.NewDecoder(zr).Decode(&entry); err != nil {
		return CachedResult{}, false
	}
	if entry.Version != d.version {
		return CachedResult{}, false
	}
	return CachedResult{
		Tokens:       entry.Tokens,
		TokensFull:   entry.TokensFull,
		TokensSig:    entry.TokensSig,
		IsCompressed: entry.IsCompressed,
		Language:     entry.Language,
		SigContent:   entry.SigContent,
	}, true
}

// Put persists an entry atomically. Entries whose signature contains a
// detected secret are not written at all.
func (d *DiskCache) Put(hash [32]byte, mode Mode, model, language string, result CachedResult) {
	if !d.Enabled() {
		return
	}
	if len(result.SigContent) > 0 {
		if d.scanner == nil {
			d.scanner = secrets.New()
		}
		if _, detections := d.scanner.RedactContent(result.SigContent); len(detections) > 0 {
			return
		}
	}
	entry := diskEntry{
		Version:      d.version,
		Tokens:       result.Tokens,
		TokensFull:   result.TokensFull,
		TokensSig:    result.TokensSig,
		IsCompressed: result.IsCompressed,
		Language:     result.Language,
		SigContent:   result.SigContent,
	}
	tmp, err := os.CreateTemp(d.dir, ".entry-*.tmp")
	if err != nil {
		return
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	zw := gzip.NewWriter(tmp)
	if err := json.NewEncoder(zw).Encode(entry); err != nil {
		zw.Close()
		tmp.Close()
		return
	}
	if err := zw.Close(); err != nil {
		tmp.Close()
		return
	}
	if err := tmp.Close(); err != nil {
		return
	}
	if err := os.Rename(tmpName, filepath.Join(d.dir, diskKey(hash, mode, model, language, d.version))); err != nil {
		return
	}
	if d.puts.Add(1)%64 == 0 {
		d.prune()
	}
}

// prune removes oldest entries until the directory is back under 80% of the
// cap. It runs on the write path only, never in a background goroutine.
func (d *DiskCache) prune() {
	entries, err := os.ReadDir(d.dir)
	if err != nil {
		return
	}
	type item struct {
		name string
		size int64
		mod  time.Time
	}
	items := make([]item, 0, len(entries))
	var total int64
	for _, e := range entries {
		info, err := e.Info()
		if err != nil || info.IsDir() {
			continue
		}
		items = append(items, item{name: e.Name(), size: info.Size(), mod: info.ModTime()})
		total += info.Size()
	}
	if total <= d.maxBytes {
		return
	}
	sort.Slice(items, func(i, j int) bool { return items[i].mod.Before(items[j].mod) })
	target := d.maxBytes * 8 / 10
	for _, it := range items {
		if total <= target {
			break
		}
		if err := os.Remove(filepath.Join(d.dir, it.name)); err == nil {
			total -= it.size
		}
	}
}

// diskKey namespaces an entry by content hash, processing options, language,
// and cache format version.
func diskKey(hash [32]byte, mode Mode, model, language string, version int) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%x|%d|%s|%s|%d", hash, mode, model, language, version)))
	return hex.EncodeToString(sum[:]) + ".json.gz"
}
