package scan

import (
	"crypto/sha256"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestDiskCacheRoundTrip(t *testing.T) {
	d := NewDiskCache(t.TempDir(), 0)
	if !d.Enabled() {
		t.Fatal("cache not enabled")
	}
	hash := sha256.Sum256([]byte("content"))
	result := CachedResult{
		Tokens: 12, TokensFull: 12, TokensSig: 4,
		IsCompressed: true, Language: "go",
		SigContent: []byte("package p\n\nfunc F()\n"),
	}
	d.Put(hash, ModePicker, "cl100k", "go", result)

	got, ok := d.Get(hash, ModePicker, "cl100k", "go")
	if !ok {
		t.Fatal("cache miss after put")
	}
	if got.Tokens != 12 || got.TokensSig != 4 || !got.IsCompressed || got.Language != "go" {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
	if string(got.SigContent) != string(result.SigContent) {
		t.Fatalf("signature mismatch: %q", got.SigContent)
	}
	if len(got.Content) != 0 {
		t.Fatalf("raw source must not be persisted: %q", got.Content)
	}

	// Mode/model/language are part of the key.
	if _, ok := d.Get(hash, ModeFull, "cl100k", "go"); ok {
		t.Error("cross-mode hit")
	}
	if _, ok := d.Get(hash, ModePicker, "o200k", "go"); ok {
		t.Error("cross-model hit")
	}
	if _, ok := d.Get(hash, ModePicker, "cl100k", "rust"); ok {
		t.Error("cross-language hit")
	}
}

func TestDiskCacheVersionInvalidation(t *testing.T) {
	dir := t.TempDir()
	hash := sha256.Sum256([]byte("content"))
	NewDiskCacheWithVersion(dir, 0, 1).Put(hash, ModeFull, "m", "", CachedResult{TokensFull: 7})

	if _, ok := NewDiskCacheWithVersion(dir, 0, 2).Get(hash, ModeFull, "m", ""); ok {
		t.Fatal("version bump did not invalidate entry")
	}
	if got, ok := NewDiskCacheWithVersion(dir, 0, 1).Get(hash, ModeFull, "m", ""); !ok || got.TokensFull != 7 {
		t.Fatalf("same-version entry lost: ok=%v got=%+v", ok, got)
	}
}

func TestDiskCacheCorruptFileIsMiss(t *testing.T) {
	dir := t.TempDir()
	d := NewDiskCache(dir, 0)
	hash := sha256.Sum256([]byte("content"))
	path := filepath.Join(dir, diskKey(hash, ModeFull, "m", "", diskCacheVersion))
	if err := os.WriteFile(path, []byte("not gzip"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ok := d.Get(hash, ModeFull, "m", ""); ok {
		t.Fatal("corrupt entry reported as hit")
	}
}

func TestDiskCacheSkipsSecretSignatures(t *testing.T) {
	d := NewDiskCache(t.TempDir(), 0)
	hash := sha256.Sum256([]byte("content"))
	d.Put(hash, ModePicker, "m", "go", CachedResult{
		TokensSig:  10,
		SigContent: []byte("const aws = \"AKIAIOSFODNN7EXAMPLE\"\n"),
	})
	if _, ok := d.Get(hash, ModePicker, "m", "go"); ok {
		t.Fatal("entry containing a secret was persisted")
	}
}

func TestDiskCacheConcurrent(t *testing.T) {
	d := NewDiskCache(t.TempDir(), 0)
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			hash := sha256.Sum256([]byte{byte(i)})
			d.Put(hash, ModeFull, "m", "", CachedResult{TokensFull: i})
			d.Get(hash, ModeFull, "m", "")
		}(i)
	}
	wg.Wait()
}

func TestDiskCachePrune(t *testing.T) {
	dir := t.TempDir()
	d := NewDiskCache(dir, 300)
	for i := 0; i < 4; i++ {
		hash := sha256.Sum256([]byte{byte(i)})
		d.Put(hash, ModePicker, "m", "go", CachedResult{
			TokensSig:  100,
			SigContent: []byte("signature payload padding padding padding"),
		})
	}
	d.prune()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var total int64
	for _, e := range entries {
		info, _ := e.Info()
		total += info.Size()
	}
	if total > 300 {
		t.Fatalf("cache size %d exceeds cap after prune", total)
	}
}
