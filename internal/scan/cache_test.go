package scan

import (
	"reflect"
	"sync"
	"testing"

	"github.com/bethropolis/sift/internal/logger"
)

func newCacheTestProcessor(t *testing.T, compressOn bool, cache *ContentCache) *Processor {
	t.Helper()
	p, err := New(Options{
		SmartFilter:    false,
		SmartMaxTokens: 0,
		Compress:       compressOn,
		Logger:         logger.New(discardWriter{}, false, false),
		Cache:          cache,
	})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// TestProcessCacheReplayIdentical verifies a second Process of identical
// content returns a bit-identical entry in every mode, proving hits replay
// rather than recompute.
func TestProcessCacheReplayIdentical(t *testing.T) {
	for _, mode := range []Mode{ModeFull, ModePicker, ModeSignatures} {
		p := newCacheTestProcessor(t, true, NewContentCache())
		first, err := p.Process("main.go", []byte(goSource), mode)
		if err != nil {
			t.Fatalf("mode %d first: %v", mode, err)
		}
		second, err := p.Process("main.go", []byte(goSource), mode)
		if err != nil {
			t.Fatalf("mode %d second: %v", mode, err)
		}
		if !reflect.DeepEqual(first, second) {
			t.Errorf("mode %d: replay differs:\nfirst=%+v\nsecond=%+v", mode, first, second)
		}
	}
}

// TestProcessCacheSkipsExpensivePaths verifies the hit path never touches
// the compressor or tokenizer: a processor built without a compressor
// serves the compressed entry purely from a shared cache populated by a
// compression-enabled processor.
func TestProcessCacheSkipsExpensivePaths(t *testing.T) {
	cache := NewContentCache()
	full := newCacheTestProcessor(t, true, cache)
	want, err := full.Process("main.go", []byte(goSource), ModePicker)
	if err != nil {
		t.Fatal(err)
	}
	if want.SigContent == nil {
		t.Fatal("setup: SigContent nil")
	}

	lean := newCacheTestProcessor(t, false, cache)
	got, err := lean.Process("main.go", []byte(goSource), ModePicker)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("lean processor replay differs:\nwant=%+v\ngot=%+v", want, got)
	}
}

// TestContentCacheModelSeparation verifies counts are namespaced by
// tokenizer model so one encoding never serves another's numbers.
func TestContentCacheModelSeparation(t *testing.T) {
	c := NewContentCache()
	content := []byte("package main\n")
	c.Put(content, ModeFull, "cl100k", CachedResult{TokensFull: 10})
	if _, hit := c.Get(content, ModeFull, "o200k"); hit {
		t.Error("cross-model cache hit")
	}
	if _, hit := c.Get(content, ModePicker, "cl100k"); hit {
		t.Error("cross-mode cache hit")
	}
	if got, hit := c.Get(content, ModeFull, "cl100k"); !hit || got.TokensFull != 10 {
		t.Errorf("same model+mode: hit=%v value=%+v", hit, got)
	}
}

// TestContentCacheNilSafe verifies a nil cache never panics and always
// misses, so zero-value processors stay correct without sharing.
func TestContentCacheNilSafe(t *testing.T) {
	var c *ContentCache
	if _, hit := c.Get([]byte("x"), ModeFull, "m"); hit {
		t.Error("nil cache hit")
	}
	c.Put([]byte("x"), ModeFull, "m", CachedResult{TokensFull: 1})
}

// TestContentCacheConcurrent verifies parallel walker workers can share one
// cache; run under -race to catch data races on the entry map.
func TestContentCacheConcurrent(t *testing.T) {
	c := NewContentCache()
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			content := []byte(goSource)
			c.Put(content, ModeFull, "m", CachedResult{TokensFull: i})
			_, _ = c.Get(content, ModeFull, "m")
		}(i)
	}
	wg.Wait()
}
