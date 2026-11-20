package app

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/bethropolis/sift/internal/config"
	"github.com/bethropolis/sift/internal/format"
	"github.com/bethropolis/sift/internal/tokenize"
)

// TestBudgetBoundsRenderedDocument verifies the emitted document, including
// prompt, tree, wrapper, and framing, stays within --budget for every style.
func TestBudgetBoundsRenderedDocument(t *testing.T) {
	tz, err := tokenize.New("")
	if err != nil {
		t.Fatal(err)
	}
	prompt := strings.Repeat("Review this codebase for race conditions and error handling. ", 8)

	for _, style := range []string{"xml", "markdown", "json", "plain"} {
		t.Run(style, func(t *testing.T) {
			cfg := config.New()
			cfg.Style = style
			cfg.Budget = 400
			cfg.SecretScan = false
			application, err := NewBuffered(cfg)
			if err != nil {
				t.Fatal(err)
			}

			files := make([]format.FileEntry, 0, 24)
			for i := 0; i < 24; i++ {
				content := []byte(fmt.Sprintf("package p%d\n\nfunc Function%d() int {\n\treturn %d\n}\n", i, i, i))
				n, countErr := tz.Count(content)
				if countErr != nil {
					t.Fatal(countErr)
				}
				files = append(files, format.FileEntry{
					Path:    fmt.Sprintf("pkg%d/file%d.go", i%4, i),
					Content: content,
					Tokens:  n,
				})
			}

			kept := application.fitToBudget(files, prompt)
			if len(kept) == 0 {
				t.Fatal("budget of 400 should keep at least one small file")
			}
			var buf bytes.Buffer
			if _, err := application.renderDocumentTo(kept, prompt, &buf, nil); err != nil {
				t.Fatal(err)
			}
			n, err := tz.Count(buf.Bytes())
			if err != nil {
				t.Fatal(err)
			}
			if n > cfg.Budget {
				t.Fatalf("rendered %s document = %d tokens, budget %d", style, n, cfg.Budget)
			}
		})
	}
}

// TestBudgetConsumedByPrompt checks the explicit edge case: when the prompt
// and framing alone exceed the budget, no files are selected.
func TestBudgetConsumedByPrompt(t *testing.T) {
	cfg := config.New()
	cfg.Budget = 10
	cfg.SecretScan = false
	application, err := NewBuffered(cfg)
	if err != nil {
		t.Fatal(err)
	}
	prompt := strings.Repeat("very long instruction ", 50)
	files := []format.FileEntry{{Path: "a.go", Content: []byte("package a\n"), Tokens: 5}}
	if kept := application.fitToBudget(files, prompt); len(kept) != 0 {
		t.Fatalf("selected %d files despite prompt exceeding budget", len(kept))
	}
}
