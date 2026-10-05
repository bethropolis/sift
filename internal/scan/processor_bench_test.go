package scan

import (
	"strings"
	"testing"

	"github.com/bethropolis/sift/internal/logger"
)

// benchSource is a realistic mid-size Go file so BPE counting dominates the
// measurement, matching what a dump scan actually pays per file.
var benchSource = []byte("package main\n\n" + strings.Repeat(`// Handler processes the request and writes a response.
func Handler%d(w http.ResponseWriter, r *http.Request) error {
	dec := json.NewDecoder(r.Body)
	var payload struct {
		Name  string `+"`json:\"name\"`"+`
		Count int    `+"`json:\"count\"`"+`
	}
	if err := dec.Decode(&payload); err != nil {
		return fmt.Errorf("decode: %%w", err)
	}
	fmt.Fprintf(w, "handled %%s", payload.Name)
	return nil
}
`, 60))

func newBenchProcessor(b *testing.B, smart bool, cache *ContentCache) *Processor {
	b.Helper()
	p, err := New(Options{
		SmartFilter: smart,
		Compress:    false,
		Logger:      logger.New(discardWriter{}, false, false),
		Cache:       cache,
	})
	if err != nil {
		b.Fatal(err)
	}
	return p
}

// BenchmarkProcessFullSmartCold measures a scan-like pass: fresh cache every
// iteration, so hashing, token counting, and guardrail all run. This is the
// dump/watch cold path.
func BenchmarkProcessFullSmartCold(b *testing.B) {
	p := newBenchProcessor(b, true, NewContentCache())
	b.ReportAllocs()
	b.SetBytes(int64(len(benchSource)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.cache = NewContentCache()
		if _, err := p.Process("main.go", benchSource, ModeFull); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkProcessFullSmartWarm measures the watch re-render path: identical
// content served from the cache. Token counting must not run on a hit.
func BenchmarkProcessFullSmartWarm(b *testing.B) {
	cache := NewContentCache()
	p := newBenchProcessor(b, true, cache)
	if _, err := p.Process("main.go", benchSource, ModeFull); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.SetBytes(int64(len(benchSource)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := p.Process("main.go", benchSource, ModeFull); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkProcessNoFilterCold measures ModeFull without the smart filter
// (plain `sift dump`): one hash, one token count, one cache put.
func BenchmarkProcessNoFilterCold(b *testing.B) {
	p := newBenchProcessor(b, false, NewContentCache())
	b.ReportAllocs()
	b.SetBytes(int64(len(benchSource)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.cache = NewContentCache()
		if _, err := p.Process("main.go", benchSource, ModeFull); err != nil {
			b.Fatal(err)
		}
	}
}
