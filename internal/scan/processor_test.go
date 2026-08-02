package scan

import (
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/bethropolis/sift/internal/logger"
)

func newTestProcessor(t *testing.T, smartOn bool) *Processor {
	t.Helper()
	return newTestProcessorC(t, smartOn, true)
}

func newTestProcessorC(t *testing.T, smartOn, compressOn bool) *Processor {
	t.Helper()
	p, err := New(Options{
		SmartFilter:    smartOn,
		SmartMaxTokens: 0,
		Compress:       compressOn,
		Logger:         logger.New(discardWriter{}, false, false),
	})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

type discardWriter struct{}

func (discardWriter) Write(p []byte) (int, error) { return len(p), nil }

const goSource = `package main

// main runs the program.
func main() {
	done := false
	for !done {
		done = true
	}
	println("hi")
}
`

// TestProcessFullRetainsSource verifies ModeFull leaves content untouched and
// counts tokens once.
func TestProcessFullRetainsSource(t *testing.T) {
	p := newTestProcessor(t, false)
	e, err := p.Process("main.go", []byte(goSource), ModeFull)
	if err != nil {
		t.Fatal(err)
	}
	if string(e.Content) != goSource {
		t.Error("ModeFull mutated content")
	}
	if e.IsCompressed {
		t.Error("IsCompressed set in full mode")
	}
	if e.Tokens <= 0 {
		t.Errorf("Tokens = %d, want > 0", e.Tokens)
	}
	if e.TokensFull != e.Tokens || e.TokensSig != e.Tokens {
		t.Errorf("TokensFull/TokensSig must match Tokens: %d/%d/%d", e.TokensFull, e.TokensSig, e.Tokens)
	}
}

// TestProcessSignaturesRendersCompressed verifies ModeSignatures swaps the raw
// body for the signature summary and counts the compressed text.
func TestProcessSignaturesRendersCompressed(t *testing.T) {
	p := newTestProcessor(t, false)
	e, err := p.Process("main.go", []byte(goSource), ModeSignatures)
	if err != nil {
		t.Fatal(err)
	}
	if !e.IsCompressed {
		t.Error("IsCompressed false in signatures mode")
	}
	if !strings.Contains(string(e.Content), "func main() { /* ... */ }") {
		t.Errorf("compressed content missing signature:\n%s", e.Content)
	}
	if strings.Contains(string(e.Content), "for !done") {
		t.Error("raw body leaked into signatures mode")
	}
	if e.Tokens <= 0 || e.Tokens >= 30 {
		t.Errorf("Tokens = %d, want a small compressed count", e.Tokens)
	}
}

// TestProcessPickerRetainsBoth verifies ModePicker keeps full and signature
// variants plus both token counts, leaving Tokens unset.
func TestProcessPickerRetainsBoth(t *testing.T) {
	p := newTestProcessor(t, false)
	e, err := p.Process("main.go", []byte(goSource), ModePicker)
	if err != nil {
		t.Fatal(err)
	}
	if string(e.Content) != goSource {
		t.Error("ModePicker mutated Content")
	}
	if e.Tokens != -1 {
		t.Errorf("Tokens = %d, want -1 until the view is chosen", e.Tokens)
	}
	if e.SigContent == nil {
		t.Fatal("SigContent nil in picker mode")
	}
	if !strings.Contains(string(e.SigContent), "func main()") {
		t.Errorf("SigContent missing signature:\n%s", e.SigContent)
	}
	if e.TokensFull <= 0 || e.TokensSig <= 0 || e.TokensSig >= e.TokensFull {
		t.Errorf("unexpected counts: full=%d sig=%d", e.TokensFull, e.TokensSig)
	}
}

// TestProcessUnsupportedLanguageFallback verifies non-source files keep their
// content with signature equal to the full variant.
func TestProcessUnsupportedLanguageFallback(t *testing.T) {
	p := newTestProcessor(t, false)
	e, err := p.Process("notes.txt", []byte("plain text\n"), ModePicker)
	if err != nil {
		t.Fatal(err)
	}
	if e.SigContent != nil {
		t.Error("SigContent set for unsupported language")
	}
	if e.TokensFull != e.TokensSig {
		t.Errorf("TokensFull=%d TokensSig=%d, want equal", e.TokensFull, e.TokensSig)
	}
}

// TestProcessFullWithoutCompression verifies full mode is unaffected when the
// compressor is disabled (the default for ordinary scans).
func TestProcessFullWithoutCompression(t *testing.T) {
	p := newTestProcessorC(t, false, false)
	e, err := p.Process("main.go", []byte(goSource), ModeFull)
	if err != nil {
		t.Fatal(err)
	}
	if string(e.Content) != goSource || e.IsCompressed {
		t.Error("full mode with compression disabled must keep raw content")
	}
	if e.Tokens <= 0 {
		t.Errorf("Tokens = %d, want > 0", e.Tokens)
	}
}

// TestProcessSignaturesWithoutCompression verifies signatures mode falls back
// to raw content instead of panicking when the compressor is disabled.
func TestProcessSignaturesWithoutCompression(t *testing.T) {
	p := newTestProcessorC(t, false, false)
	e, err := p.Process("main.go", []byte(goSource), ModeSignatures)
	if err != nil {
		t.Fatal(err)
	}
	if string(e.Content) != goSource {
		t.Error("signatures mode with compression disabled must keep raw content")
	}
	if e.IsCompressed {
		t.Error("IsCompressed set despite disabled compressor")
	}
	if e.TokensFull != e.Tokens || e.TokensSig != e.Tokens {
		t.Errorf("counts must match without compression: full=%d sig=%d tok=%d", e.TokensFull, e.TokensSig, e.Tokens)
	}
}

// TestProcessPickerWithoutCompression verifies picker mode leaves SigContent
// nil and falls back to the full token count when the compressor is disabled.
func TestProcessPickerWithoutCompression(t *testing.T) {
	p := newTestProcessorC(t, false, false)
	e, err := p.Process("main.go", []byte(goSource), ModePicker)
	if err != nil {
		t.Fatal(err)
	}
	if e.SigContent != nil {
		t.Error("SigContent set despite disabled compressor")
	}
	if e.Tokens != -1 {
		t.Errorf("Tokens = %d, want -1 until the view is chosen", e.Tokens)
	}
	if e.TokensSig != e.TokensFull || e.TokensFull <= 0 {
		t.Errorf("counts must equal the full count without compression: full=%d sig=%d", e.TokensFull, e.TokensSig)
	}
}

// TestProcessSmartSkips verifies the smart filter rejects generated and
// oversized files via the ErrFileSkipped sentinel, with the reason wrapped.
func TestProcessSmartSkips(t *testing.T) {
	p := newTestProcessor(t, true)
	_, err := p.Process("gen.go", []byte("// Code generated by tool. DO NOT EDIT.\npackage gen\n"), ModeFull)
	if !errors.Is(err, ErrFileSkipped) {
		t.Fatalf("generated file: err = %v, want ErrFileSkipped", err)
	}

	_, err = p.Process("package-lock.json", []byte("{\"lockfileVersion\":3}"), ModeFull)
	if !errors.Is(err, ErrFileSkipped) {
		t.Fatalf("lockfile: err = %v, want ErrFileSkipped", err)
	}
}

// TestProcessTokenLimitSkip verifies the token guardrail skip.
func TestProcessTokenLimitSkip(t *testing.T) {
	p, err := New(Options{SmartFilter: true, SmartMaxTokens: 1, Logger: logger.New(discardWriter{}, false, false)})
	if err != nil {
		t.Fatal(err)
	}
	_, err = p.Process("big.go", []byte(goSource), ModeFull)
	if !errors.Is(err, ErrFileSkipped) {
		t.Fatalf("err = %v, want ErrFileSkipped", err)
	}
}

// TestProcessConcurrent verifies one Processor is safe for concurrent use,
// exercising the shared tokenizer and compressor from many goroutines.
func TestProcessConcurrent(t *testing.T) {
	p := newTestProcessor(t, false)
	var wg sync.WaitGroup
	errs := make(chan error, 64)
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 8; j++ {
				e, err := p.Process("main.go", []byte(goSource), ModePicker)
				if err != nil {
					errs <- err
					return
				}
				if e.SigContent == nil || e.TokensFull <= 0 {
					errs <- errors.New("unexpected picker entry")
					return
				}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}
