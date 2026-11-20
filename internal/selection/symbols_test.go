package selection

import (
	"testing"

	"github.com/bethropolis/sift/internal/format"
)

func TestSymbolsExtraction(t *testing.T) {
	cases := []struct {
		name string
		file format.FileEntry
		want []string
	}{
		{
			name: "go",
			file: format.FileEntry{Path: "a.go", Content: []byte("package p\n\nfunc Alpha() {}\nfunc (s *Server) Start() {}\ntype Config struct{}\n")},
			want: []string{"Alpha", "Config", "Start"},
		},
		{
			name: "python",
			file: format.FileEntry{Path: "a.py", Content: []byte("class Engine:\n    pass\n\ndef run():\n    pass\n")},
			want: []string{"Engine", "run"},
		},
		{
			name: "typescript",
			file: format.FileEntry{Path: "a.ts", Content: []byte("export function loadConfig() {}\nexport class Store {}\n")},
			want: []string{"Store", "loadConfig"},
		},
		{
			name: "signature preferred",
			file: format.FileEntry{
				Path:       "a.go",
				Content:    []byte("package p\n\nfunc HiddenBody() {}\n"),
				SigContent: []byte("package p\n\nfunc FromSignature()\n"),
			},
			want: []string{"FromSignature"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Symbols(tc.file)
			if len(got) != len(tc.want) {
				t.Fatalf("Symbols = %v, want %v", got, tc.want)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Fatalf("Symbols = %v, want %v", got, tc.want)
				}
			}
		})
	}
}

func TestTaskRelevanceUsesSymbols(t *testing.T) {
	defining := Candidate{
		File:    format.FileEntry{Path: "internal/util/helper.go", Content: []byte("package util\n\nfunc ParseConfig() {}\n")},
		Symbols: []string{"ParseConfig"},
	}
	unrelated := Candidate{
		File:    format.FileEntry{Path: "internal/util/other.go", Content: []byte("package util\n\nfunc Other() {}\n")},
		Symbols: []string{"Other"},
	}
	prompt := "How does ParseConfig load the config file?"
	got := taskRelevance(defining, prompt)
	want := taskRelevance(unrelated, prompt)
	if got <= want {
		t.Fatalf("symbol match relevance = %f, unrelated = %f", got, want)
	}
	if got < 0.2 {
		t.Fatalf("symbol match relevance too low: %f", got)
	}
}

func TestTaskRelevancePathStillCounts(t *testing.T) {
	byPath := Candidate{File: format.FileEntry{Path: "internal/config/loader.go"}}
	if got := taskRelevance(byPath, "config loader"); got == 0 {
		t.Fatal("path terms should still contribute relevance")
	}
}
