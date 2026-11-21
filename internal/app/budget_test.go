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
// TestBudgetRestoresSignatureTokensExactly guards the framing asymmetry: the
// fitter adds framing to TokensSig, and JSON framing depends on the token
// count. Restoring by recomputing after inflation corrupts the count.
func TestBudgetRestoresSignatureTokensExactly(t *testing.T) {
	cfg := config.New()
	cfg.Style = "json"
	cfg.FallbackMode = "signatures"
	cfg.FallbackThreshold = 1.0
	cfg.SecretScan = false
	application, err := NewBuffered(cfg)
	if err != nil {
		t.Fatal(err)
	}
	files := []format.FileEntry{{
		Path:       "big.go",
		Content:    []byte("package big\n"),
		Tokens:     200,
		TokensFull: 200,
		TokensSig:  100,
		SigContent: []byte("package big\n\nfunc Big()\n"),
	}}
	tz, err := tokenize.New("")
	if err != nil {
		t.Fatal(err)
	}
	style := format.ParseStyle(cfg.EffectiveStyle())
	cfg.Budget = application.overheadTokens(tz, style, "", files) + 200
	kept := application.fitToBudget(files, "")
	if len(kept) != 1 {
		t.Fatalf("kept %d files, want 1", len(kept))
	}
	if !kept[0].IsCompressed {
		t.Fatal("file was not compressed by fallback")
	}
	if kept[0].Tokens != 100 || kept[0].TokensSig != 100 {
		t.Fatalf("restored tokens = %d/%d, want 100/100", kept[0].Tokens, kept[0].TokensSig)
	}
}

// TestBudgetPinnedFullIsNeverDowngraded proves [modes].full wins over fallback
// compression: a pinned file is dropped rather than turned into signatures.
func TestBudgetPinnedFullIsNeverDowngraded(t *testing.T) {
	cfg := config.New()
	cfg.Style = "plain"
	cfg.FallbackMode = "signatures"
	cfg.FallbackThreshold = 1.0
	cfg.SecretScan = false
	application, err := NewBuffered(cfg)
	if err != nil {
		t.Fatal(err)
	}
	files := []format.FileEntry{
		{
			Path:       "small.go",
			Content:    []byte("package small\n"),
			Tokens:     10,
			TokensFull: 10,
			TokensSig:  5,
			SigContent: []byte("package small\n"),
		},
		{
			Path:       "big.go",
			Content:    []byte("package big\n"),
			Tokens:     100,
			TokensFull: 100,
			TokensSig:  10,
			SigContent: []byte("package big\n"),
			PinnedFull: true,
		},
	}
	tz, err := tokenize.New("")
	if err != nil {
		t.Fatal(err)
	}
	style := format.ParseStyle(cfg.EffectiveStyle())
	remaining := files[0].Tokens + fileFramingTokens(style, files[0].Path, files[0].Tokens)
	cfg.Budget = application.overheadTokens(tz, style, "", files) + remaining
	kept := application.fitToBudget(files, "")
	if len(kept) != 1 || kept[0].Path != "small.go" {
		t.Fatalf("kept = %v, want [small.go] (pinned full must not compress)", pathsOf(kept))
	}
}

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
