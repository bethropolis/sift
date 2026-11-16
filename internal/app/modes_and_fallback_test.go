package app

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bethropolis/sift/internal/config"
	"github.com/bethropolis/sift/internal/format"
	"github.com/bethropolis/sift/internal/logger"
	"github.com/bethropolis/sift/internal/walker"
)

func TestModesCollection(t *testing.T) {
	dir := t.TempDir()

	// Create test files
	dataDir := filepath.Join(dir, "pkg", "data")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatal(err)
	}
	repoFile := filepath.Join(dataDir, "repo.go")
	if err := os.WriteFile(repoFile, []byte("package data\n\nfunc GetData() string {\n\treturn \"data\"\n}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	uiDir := filepath.Join(dir, "internal", "ui")
	if err := os.MkdirAll(uiDir, 0755); err != nil {
		t.Fatal(err)
	}
	viewFile := filepath.Join(uiDir, "view.go")
	if err := os.WriteFile(viewFile, []byte("package ui\n\nfunc RenderView() string {\n\treturn \"view\"\n}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	testFile := filepath.Join(dataDir, "repo_test.go")
	if err := os.WriteFile(testFile, []byte("package data\n\nfunc TestGetData() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := config.New()
	cfg.RootDir = dir
	cfg.Modes = config.ModesConfig{
		Full:       config.StringList{"**/data/**"},
		Signatures: config.StringList{"**/ui/**"},
		Skip:       config.StringList{"**/*_test.go"},
	}

	app, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}

	files, skipped, err := app.Collect()
	if err != nil {
		t.Fatal(err)
	}

	// Verify repo_test.go was skipped
	for _, f := range files {
		if strings.HasSuffix(f.Path, "repo_test.go") {
			t.Errorf("repo_test.go should have been skipped, but was collected")
		}
	}
	foundSkipReason := false
	for _, s := range skipped {
		if strings.HasSuffix(s.Path, "repo_test.go") && s.Reason == walker.ReasonFilteredPath {
			foundSkipReason = true
			break
		}
	}
	if !foundSkipReason {
		t.Errorf("expected repo_test.go in skipped items with ReasonFilteredPath")
	}

	// Verify repo.go is Full mode
	// Verify view.go is Signatures mode
	var foundRepo, foundView bool
	for _, f := range files {
		if strings.HasSuffix(f.Path, "repo.go") {
			foundRepo = true
			if f.IsCompressed {
				t.Errorf("repo.go should be full mode, but IsCompressed = true")
			}
		}
		if strings.HasSuffix(f.Path, "view.go") {
			foundView = true
			if !f.IsCompressed {
				t.Errorf("view.go should be signatures mode, but IsCompressed = false")
			}
		}
	}
	if !foundRepo {
		t.Errorf("repo.go was not collected")
	}
	if !foundView {
		t.Errorf("view.go was not collected")
	}
}

func TestRenderFallbackCompression(t *testing.T) {
	cfg := config.New()
	cfg.FallbackMode = "signatures"
	cfg.FallbackThreshold = 1.0
	cfg.Budget = 100

	var out bytes.Buffer

	app := &App{
		cfg:    cfg,
		log:    logger.New(&bytes.Buffer{}, false, false),
		output: &out,
	}

	// 3 files:
	// a.go: 50 tokens full, 10 tokens sig
	// b.go: 60 tokens full, 12 tokens sig -> overflows 100 budget in full, falls back to sig
	// c.go: 40 tokens full, 8 tokens sig -> fallback mode active, packed as sig
	files := []format.FileEntry{
		{
			Path:       "a.go",
			Content:    []byte("package a\n\nfunc FullA() string { return \"A\" }\n"),
			Tokens:     50,
			TokensFull: 50,
			SigContent: []byte("package a\n\nfunc FullA() string\n"),
			TokensSig:  10,
		},
		{
			Path:       "b.go",
			Content:    []byte("package b\n\nfunc FullB() string { return \"B\" }\n"),
			Tokens:     60,
			TokensFull: 60,
			SigContent: []byte("package b\n\nfunc FullB() string\n"),
			TokensSig:  12,
		},
		{
			Path:       "c.go",
			Content:    []byte("package c\n\nfunc FullC() string { return \"C\" }\n"),
			Tokens:     40,
			TokensFull: 40,
			SigContent: []byte("package c\n\nfunc FullC() string\n"),
			TokensSig:  8,
		},
	}

	err := app.render(files, nil, time.Second, nil, true)
	if err != nil {
		t.Fatal(err)
	}

	rendered := out.String()
	// All 3 files should be in the rendered output!
	if !strings.Contains(rendered, "a.go") {
		t.Errorf("rendered output missing a.go:\n%s", rendered)
	}
	if !strings.Contains(rendered, "b.go") {
		t.Errorf("rendered output missing b.go:\n%s", rendered)
	}
	if !strings.Contains(rendered, "c.go") {
		t.Errorf("rendered output missing c.go:\n%s", rendered)
	}

	// a.go should have its full body
	if !strings.Contains(rendered, "return \"A\"") {
		t.Errorf("rendered output missing full body for a.go:\n%s", rendered)
	}
	// b.go and c.go should be in signature mode (no return "B", return "C")
	if strings.Contains(rendered, "return \"B\"") {
		t.Errorf("b.go should have been compressed to signature, but found return B:\n%s", rendered)
	}
	if strings.Contains(rendered, "return \"C\"") {
		t.Errorf("c.go should have been compressed to signature, but found return C:\n%s", rendered)
	}
}

func TestCollectAndRenderFallbackEndToEnd(t *testing.T) {
	dir := t.TempDir()

	file1 := filepath.Join(dir, "alpha.go")
	file1Content := `package main

import "fmt"

func Alpha() {
	for i := 0; i < 100; i++ {
		fmt.Println("Alpha full implementation details")
	}
}
`
	if err := os.WriteFile(file1, []byte(file1Content), 0644); err != nil {
		t.Fatal(err)
	}

	file2 := filepath.Join(dir, "beta.go")
	file2Content := `package main

import "fmt"

func Beta() {
	for i := 0; i < 100; i++ {
		fmt.Println("Beta full implementation details")
	}
}
`
	if err := os.WriteFile(file2, []byte(file2Content), 0644); err != nil {
		t.Fatal(err)
	}

	// First let's check token count of one file so we can set an exact budget
	cfgInit := config.New()
	cfgInit.RootDir = dir
	appInit, err := New(cfgInit)
	if err != nil {
		t.Fatal(err)
	}
	filesInit, _, err := appInit.Collect()
	if err != nil {
		t.Fatal(err)
	}
	if len(filesInit) != 2 {
		t.Fatalf("expected 2 files, got %d", len(filesInit))
	}

	// Each file is ~45 tokens full, and ~15 tokens signatures.
	// Setting budget = 60 means:
	// alpha (45 tokens) fits full.
	// beta (45 tokens) overflows 60, falls back to signature (~15 tokens).
	// Total tokens: ~60 tokens <= 60 budget!
	cfg := config.New()
	cfg.RootDir = dir
	cfg.FallbackMode = "signatures"
	cfg.Budget = filesInit[0].Tokens + 20

	var out bytes.Buffer
	app, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	app.output = &out

	files, skipped, err := app.Collect()
	if err != nil {
		t.Fatal(err)
	}
	if err := app.Render(files, skipped, time.Second, nil); err != nil {
		t.Fatal(err)
	}

	rendered := out.String()
	// Both files must be present!
	if !strings.Contains(rendered, "alpha.go") {
		t.Errorf("rendered output missing alpha.go:\n%s", rendered)
	}
	if !strings.Contains(rendered, "beta.go") {
		t.Errorf("rendered output missing beta.go:\n%s", rendered)
	}

	// The first file kept must be full fidelity, and the second file must be compressed signature
	first := files[0].Path
	second := files[1].Path
	var firstPattern, secondPattern string
	if strings.Contains(first, "alpha") {
		firstPattern = "Alpha full implementation details"
		secondPattern = "Beta full implementation details"
	} else {
		firstPattern = "Beta full implementation details"
		secondPattern = "Alpha full implementation details"
	}

	if !strings.Contains(rendered, firstPattern) {
		t.Errorf("%s should have full implementation body:\n%s", first, rendered)
	}
	if strings.Contains(rendered, secondPattern) {
		t.Errorf("%s should have signature only, but found full implementation:\n%s", second, rendered)
	}
	if !strings.Contains(rendered, "/* ... */") {
		t.Errorf("expected signature compression /* ... */ in rendered output:\n%s", rendered)
	}
}
