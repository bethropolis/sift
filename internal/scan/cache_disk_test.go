package scan

import (
	"crypto/sha256"
	"testing"
)

// TestContentCacheDiskPromotion verifies a second in-memory cache backed by the
// same directory serves entries written by the first.
func TestContentCacheDiskPromotion(t *testing.T) {
	dir := t.TempDir()
	first := NewContentCache()
	first.EnableDisk(NewDiskCache(dir, 0))
	if !first.HasDisk() {
		t.Fatal("disk layer not attached")
	}

	content := []byte("package p\n")
	hash := sha256.Sum256(content)
	first.PutHashed(hash, ModeFull, "m", "go", CachedResult{Tokens: 3, TokensFull: 3})

	second := NewContentCache()
	second.EnableDisk(NewDiskCache(dir, 0))
	got, ok := second.GetHashed(hash, ModeFull, "m", "go")
	if !ok {
		t.Fatal("disk-backed cache miss")
	}
	if got.TokensFull != 3 {
		t.Fatalf("TokensFull = %d", got.TokensFull)
	}
	if len(got.Content) != 0 {
		t.Fatalf("disk hit should not carry raw content: %q", got.Content)
	}
}
