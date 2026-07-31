package tokenize

import (
	"testing"

	"github.com/bethropolis/dir-dumper/internal/format"
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
