package tokenize

import (
	"testing"

	"github.com/bethropolis/sift/internal/format"
	"github.com/tiktoken-go/tokenizer"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		enc     string
		wantErr bool
	}{
		{name: "empty defaults to cl100k", enc: ""},
		{name: "cl100k", enc: "cl100k_base"},
		{name: "o200k", enc: "o200k_base"},
		{name: "case insensitive", enc: "CL100K_BASE"},
		{name: "unsupported", enc: "bogus", wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tk, err := New(tc.enc)
			if (err != nil) != tc.wantErr {
				t.Fatalf("New(%q) err = %v, wantErr %v", tc.enc, err, tc.wantErr)
			}
			if err == nil && tk == nil {
				t.Error("New returned nil tokenizer with nil error")
			}
		})
	}
}

func TestCount(t *testing.T) {
	tk, err := New("cl100k_base")
	if err != nil {
		t.Fatal(err)
	}

	got, err := tk.Count([]byte("hello world"))
	if err != nil {
		t.Fatal(err)
	}
	if got == 0 {
		t.Error("Count returned 0 tokens for non-empty text")
	}

	// Empty input must be countable.
	got, err = tk.Count(nil)
	if err != nil {
		t.Fatal(err)
	}
	if got != 0 {
		t.Errorf("Count(nil) = %d, want 0", got)
	}
}

func TestFitToBudget(t *testing.T) {
	files := []format.FileEntry{
		{Path: "a", Tokens: 10},
		{Path: "b", Tokens: 20},
		{Path: "c", Tokens: 30},
	}

	// No budget keeps everything.
	kept, used := FitToBudget(files, 0)
	if len(kept) != 3 || used != 60 {
		t.Errorf("no budget: kept %d, used %d; want 3, 60", len(kept), used)
	}

	// Budget 45 keeps a+b (30) and stops before c (30 would exceed).
	kept, used = FitToBudget(files, 45)
	if len(kept) != 2 || used != 30 {
		t.Errorf("budget 45: kept %d, used %d; want 2, 30", len(kept), used)
	}

	// Budget that fits nothing.
	kept, used = FitToBudget(files, 5)
	if len(kept) != 0 || used != 0 {
		t.Errorf("budget 5: kept %d, used %d; want 0, 0", len(kept), used)
	}
}

// TestFitToBudgetWithFallbackLazy proves signature work runs only for files
// that actually fall back, and never when everything fits in full.
func TestFitToBudgetWithFallbackLazy(t *testing.T) {
	files := []format.FileEntry{
		{Path: "a.go", Content: []byte("full a"), Tokens: 20, TokensFull: 20},
		{Path: "b.go", Content: []byte("full b"), Tokens: 20, TokensFull: 20},
		{Path: "c.go", Content: []byte("full c"), Tokens: 20, TokensFull: 20},
	}
	calls := 0
	provide := func(f format.FileEntry) ([]byte, int, bool) {
		calls++
		return []byte("sig " + f.Path), 5, true
	}

	// Everything fits full: no compression work at all.
	kept, used := FitToBudgetWithFallbackLazy(files, 100, 1.0, provide)
	if calls != 0 {
		t.Fatalf("provider called %d times when all files fit full", calls)
	}
	if len(kept) != 3 || used != 60 {
		t.Fatalf("full fit: kept=%d used=%d", len(kept), used)
	}

	// Fallback after the threshold compresses only the overflowing tail.
	calls = 0
	kept, used = FitToBudgetWithFallbackLazy(files, 100, 0.2, provide)
	if calls != 2 {
		t.Fatalf("provider calls = %d, want 2 (b and c)", calls)
	}
	if len(kept) != 3 || used != 30 {
		t.Fatalf("fallback fit: kept=%d used=%d", len(kept), used)
	}
	if kept[0].IsCompressed {
		t.Error("first file should stay full")
	}
	if !kept[1].IsCompressed || !kept[2].IsCompressed {
		t.Error("tail files should be compressed")
	}

	// A provider that cannot compress leaves files full rather than failing.
	calls = 0
	kept, _ = FitToBudgetWithFallbackLazy(files, 100, 0.2, func(format.FileEntry) ([]byte, int, bool) {
		calls++
		return nil, 0, false
	})
	if calls == 0 {
		t.Fatal("provider should be consulted for fallback candidates")
	}
	if len(kept) != 3 {
		t.Fatalf("uncompressible fallback kept %d files", len(kept))
	}
}

func TestFitToBudgetWithFallback(t *testing.T) {
	files := []format.FileEntry{
		{
			Path:       "a.go",
			Content:    []byte("full a"),
			Tokens:     50,
			TokensFull: 50,
			SigContent: []byte("sig a"),
			TokensSig:  10,
		},
		{
			Path:       "b.go",
			Content:    []byte("full b"),
			Tokens:     60,
			TokensFull: 60,
			SigContent: []byte("sig b"),
			TokensSig:  12,
		},
		{
			Path:       "c.go",
			Content:    []byte("full c"),
			Tokens:     40,
			TokensFull: 40,
			SigContent: []byte("sig c"),
			TokensSig:  8,
		},
	}

	// Case 1: Threshold 1.0, budget 100.
	// a fits full (50 tokens).
	// b overflows in full (50+60 = 110 > 100), switches to signature (12 tokens, used = 62).
	// c is in fallback mode, packed as signature (8 tokens, used = 70).
	// All 3 files kept!
	kept, used := FitToBudgetWithFallback(files, 100, 1.0)
	if len(kept) != 3 {
		t.Fatalf("expected 3 files, got %d", len(kept))
	}
	if used != 70 {
		t.Errorf("used = %d, want 70", used)
	}
	if kept[0].IsCompressed || string(kept[0].Content) != "full a" {
		t.Errorf("file 0 should be full: compressed=%v content=%s", kept[0].IsCompressed, kept[0].Content)
	}
	if !kept[1].IsCompressed || string(kept[1].Content) != "sig b" {
		t.Errorf("file 1 should be signature: compressed=%v content=%s", kept[1].IsCompressed, kept[1].Content)
	}
	if !kept[2].IsCompressed || string(kept[2].Content) != "sig c" {
		t.Errorf("file 2 should be signature: compressed=%v content=%s", kept[2].IsCompressed, kept[2].Content)
	}

	// Case 2: Threshold 0.5 (50%), budget 100.
	// a takes 50 tokens (<= 50).
	// Threshold reached! b and c are packed as signatures.
	kept, used = FitToBudgetWithFallback(files, 100, 0.5)
	if len(kept) != 3 {
		t.Fatalf("threshold 0.5: expected 3 files, got %d", len(kept))
	}
	if used != 70 {
		t.Errorf("threshold 0.5: used = %d, want 70", used)
	}

	// Case 3: Percentage passed as 80 instead of 0.8
	kept, used = FitToBudgetWithFallback(files, 100, 80)
	if len(kept) != 3 || used != 70 {
		t.Errorf("threshold 80: expected 3 files / 70 tokens, got %d / %d", len(kept), used)
	}

	// Case 4: No budget returns everything unchanged
	kept, used = FitToBudgetWithFallback(files, 0, 1.0)
	if len(kept) != 3 || used != 150 {
		t.Errorf("no budget: kept %d, used %d", len(kept), used)
	}
}

func TestParseEncodingAliases(t *testing.T) {
	cases := map[string]tokenizer.Encoding{
		"cl100k":        tokenizer.Cl100kBase,
		"CL100K_BASE":   tokenizer.Cl100kBase,
		"claude":        tokenizer.Cl100kBase,
		"anthropic":     tokenizer.Cl100kBase,
		"gpt-4":         tokenizer.Cl100kBase,
		"o200k":         tokenizer.O200kBase,
		"gpt-5":         tokenizer.O200kBase,
		"gemini":        tokenizer.O200kBase,
		" GPT-4O-MINI ": tokenizer.O200kBase,
		"r50k":          tokenizer.R50kBase,
		"gpt2":          tokenizer.R50kBase,
	}
	for alias, want := range cases {
		got, err := parseEncoding(alias)
		if err != nil || got != want {
			t.Errorf("parseEncoding(%q) = %v, %v; want %v", alias, got, err, want)
		}
	}
	if _, err := parseEncoding("definitely-not-a-model"); err == nil {
		t.Error("unknown encoding should fail")
	}
	if err := ValidateModel(""); err != nil {
		t.Errorf("empty model should use the default: %v", err)
	}
}
