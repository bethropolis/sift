package printer

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
)

func TestPrintFilePlain(t *testing.T) {
	var buf bytes.Buffer
	p := New()
	p.WithOutput(&buf)
	p.WithColors(false)

	p.PrintFile("a.txt", []byte("hello"))

	want := "a.txt\nhello\n\n"
	if buf.String() != want {
		t.Errorf("output = %q, want %q", buf.String(), want)
	}
	if got := p.GetCount(); got != 1 {
		t.Errorf("GetCount() = %d, want 1", got)
	}
}

func TestPrintFileJSON(t *testing.T) {
	var buf bytes.Buffer
	p := New()
	p.WithOutput(&buf)
	p.WithJSON(true)

	p.PrintFile("a.txt", []byte("hello"))
	p.PrintFile("b.go", []byte("package main"))
	p.Finalize()

	var entries []JSONFileEntry
	if err := json.Unmarshal(buf.Bytes(), &entries); err != nil {
		t.Fatalf("invalid JSON output: %v\n%s", err, buf.String())
	}
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}
	if entries[0].Path != "a.txt" {
		t.Errorf("entries[0].Path = %q, want a.txt", entries[0].Path)
	}
	got, err := base64.StdEncoding.DecodeString(entries[0].Content)
	if err != nil {
		t.Fatalf("invalid base64 content: %v", err)
	}
	if string(got) != "hello" {
		t.Errorf("entries[0].Content = %q, want hello", got)
	}
}

func TestPrintFileMarkdown(t *testing.T) {
	var buf bytes.Buffer
	p := New()
	p.WithOutput(&buf)
	p.WithMarkdown(true)

	p.PrintFile("a.txt", []byte("hello"))

	want := "file: a.txt\n\n```\nhello\n```\n\n"
	if buf.String() != want {
		t.Errorf("output = %q, want %q", buf.String(), want)
	}
}

func TestFinalizeNoopWithoutJSON(t *testing.T) {
	var buf bytes.Buffer
	p := New()
	p.WithOutput(&buf)
	p.WithColors(false)
	p.PrintFile("a.txt", []byte("hello"))
	p.Finalize()

	want := "a.txt\nhello\n\n"
	if buf.String() != want {
		t.Errorf("output = %q, want %q", buf.String(), want)
	}
}

func TestConcurrentPrintFile(t *testing.T) {
	p := New()
	var buf bytes.Buffer
	p.WithOutput(&buf)
	p.WithColors(false)

	const goroutines = 20
	const perGoroutine = 50

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func(g int) {
			defer wg.Done()
			for i := 0; i < perGoroutine; i++ {
				p.PrintFile(fmt.Sprintf("dir/file_%d_%d.txt", g, i), []byte(fmt.Sprintf("content %d %d", g, i)))
			}
		}(g)
	}
	wg.Wait()

	if got := p.GetCount(); got != goroutines*perGoroutine {
		t.Errorf("GetCount() = %d, want %d", got, goroutines*perGoroutine)
	}

	var wantLen int64
	for g := 0; g < goroutines; g++ {
		for i := 0; i < perGoroutine; i++ {
			path := fmt.Sprintf("dir/file_%d_%d.txt", g, i)
			content := fmt.Sprintf("content %d %d", g, i)
			wantLen += int64(len(path) + 1 + len(content) + 2)
		}
	}
	if int64(buf.Len()) != wantLen {
		t.Errorf("output length = %d, want %d", buf.Len(), wantLen)
	}
}
