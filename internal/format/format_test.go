package format

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

func render(t *testing.T, style Style, doc *Document, useColors bool) string {
	t.Helper()
	r, err := NewRenderer(style, useColors)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := r.Render(doc, &buf); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

func TestBuildTree(t *testing.T) {
	paths := []string{
		"internal/app/app.go",
		"internal/format/format.go",
		"cmd/dumper/main.go",
		"go.mod",
	}
	got := BuildTree(paths)
	want := ".\n" +
		"├── cmd/\n" +
		"│   └── dumper/\n" +
		"│       └── main.go\n" +
		"├── go.mod\n" +
		"└── internal/\n" +
		"    ├── app/\n" +
		"    │   └── app.go\n" +
		"    └── format/\n" +
		"        └── format.go\n"
	if got != want {
		t.Errorf("BuildTree output:\n%s\nwant:\n%s", got, want)
	}
}

func TestRenderPlain(t *testing.T) {
	doc := &Document{
		Files: []FileEntry{
			{Path: "a.txt", Content: []byte("hello")},
		},
	}
	want := "a.txt\nhello\n\n"
	if got := render(t, StylePlain, doc, false); got != want {
		t.Errorf("plain = %q, want %q", got, want)
	}
}

func TestRenderPlainColor(t *testing.T) {
	doc := &Document{
		Files: []FileEntry{{Path: "a.txt", Content: []byte("hello")}},
	}
	got := render(t, StylePlain, doc, true)
	if !strings.Contains(got, "\033[1;36m") {
		t.Errorf("plain colored output missing ANSI code: %q", got)
	}
}

func TestRenderJSON(t *testing.T) {
	doc := &Document{
		Files: []FileEntry{
			{Path: "a.txt", Content: []byte("hello"), Tokens: 3},
			{Path: "b.go", Content: []byte("package main")},
		},
	}
	got := render(t, StyleJSON, doc, false)

	var entries []JSONFileEntry
	if err := json.Unmarshal([]byte(got), &entries); err != nil {
		t.Fatalf("invalid JSON output: %v\n%s", err, got)
	}
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}
	if entries[0].Path != "a.txt" || entries[0].Tokens != 3 {
		t.Errorf("entries[0] = %+v, want a.txt with tokens=3", entries[0])
	}
	content, err := base64.StdEncoding.DecodeString(entries[1].Content)
	if err != nil {
		t.Fatalf("invalid base64: %v", err)
	}
	if string(content) != "package main" {
		t.Errorf("entries[1].Content = %q, want package main", content)
	}
}

func TestRenderJSONEmpty(t *testing.T) {
	if got := render(t, StyleJSON, &Document{}, false); got != "[]\n" {
		t.Errorf("empty JSON = %q, want %q", got, "[]\n")
	}
}

func TestRenderInstructions(t *testing.T) {
	doc := &Document{
		Instructions: "Review this codebase for race conditions.",
		Files:        []FileEntry{{Path: "a.go", Content: []byte("package a")}},
	}

	if got := render(t, StylePlain, doc, false); !strings.Contains(got, "# Instructions\nReview this codebase for race conditions.") {
		t.Errorf("plain instructions missing:\n%s", got)
	}
	if got := render(t, StyleMarkdown, doc, false); !strings.Contains(got, "## Task / Instructions\n\nReview this codebase for race conditions.") {
		t.Errorf("markdown instructions missing:\n%s", got)
	}
	xmlOut := render(t, StyleXML, doc, false)
	if !strings.Contains(xmlOut, "<instructions>") || !strings.Contains(xmlOut, "Review this codebase for race conditions.") {
		t.Errorf("xml instructions missing:\n%s", xmlOut)
	}
	jsonOut := render(t, StyleJSON, doc, false)
	var jd jsonDoc
	if err := json.Unmarshal([]byte(jsonOut), &jd); err != nil {
		t.Fatalf("invalid JSON with instructions: %v\n%s", err, jsonOut)
	}
	if jd.Instructions != "Review this codebase for race conditions." {
		t.Errorf("json instructions = %q", jd.Instructions)
	}
	if len(jd.Files) != 1 {
		t.Errorf("json files = %d, want 1", len(jd.Files))
	}
}

func TestRenderMarkdown(t *testing.T) {
	doc := &Document{
		DirectoryTree: ".\n└── a.txt\n",
		Files: []FileEntry{
			{Path: "a.txt", Content: []byte("hello")},
			{Path: "b.go", Content: []byte("package main")},
		},
	}
	want := "```\n.\n└── a.txt\n```\n\n" +
		"file: a.txt\n\n```\nhello\n```\n\n" +
		"file: b.go\n\n```go\npackage main\n```\n\n"
	if got := render(t, StyleMarkdown, doc, false); got != want {
		t.Errorf("markdown = %q, want %q", got, want)
	}
}

func TestRenderMarkdownFenceNegotiation(t *testing.T) {
	doc := &Document{
		Files: []FileEntry{
			{Path: "doc.md", Content: []byte("before\n```go\ncode\n```\nafter")},
		},
	}
	want := "file: doc.md\n\n````markdown\nbefore\n```go\ncode\n```\nafter\n````\n\n"
	if got := render(t, StyleMarkdown, doc, false); got != want {
		t.Errorf("markdown = %q, want %q", got, want)
	}
}

func TestRenderXML(t *testing.T) {
	doc := &Document{
		DirectoryTree: ".\n└── main.go\n",
		Files: []FileEntry{
			{Path: "main.go", Content: []byte("package main"), Tokens: 5},
			{Path: "sub/a.txt", Content: []byte("a<&")},
		},
	}
	got := render(t, StyleXML, doc, false)

	for _, wantPart := range []string{
		"<repository>",
		"<structure>",
		"└── main.go",
		"<file path=\"main.go\" language=\"go\" tokens=\"5\" mode=\"full\">",
		"<![CDATA[package main]]>",
		"<file path=\"sub/a.txt\" language=\"\" tokens=\"0\" mode=\"full\">",
		"<![CDATA[a<&]]>",
		"</repository>",
	} {
		if !strings.Contains(got, wantPart) {
			t.Errorf("XML output missing %q:\n%s", wantPart, got)
		}
	}
}

func TestRenderXMLCDataEscaping(t *testing.T) {
	doc := &Document{
		Files: []FileEntry{
			{Path: "a.txt", Content: []byte("x]]>y")},
		},
	}
	got := render(t, StyleXML, doc, false)
	if !strings.Contains(got, "<![CDATA[x]]]]><![CDATA[>y]]>") {
		t.Errorf("CDATA escaping failed:\n%s", got)
	}
}

func TestRenderXMLCompressed(t *testing.T) {
	doc := &Document{
		Files: []FileEntry{
			{Path: "a.go", Content: []byte("func F()"), IsCompressed: true},
		},
	}
	got := render(t, StyleXML, doc, false)
	if !strings.Contains(got, "mode=\"signatures\"") {
		t.Errorf("XML missing signatures mode:\n%s", got)
	}
}

func TestParseStyle(t *testing.T) {
	tests := []struct {
		in   string
		want Style
	}{
		{in: "xml", want: StyleXML},
		{in: "JSON", want: StyleJSON},
		{in: " Markdown ", want: StyleMarkdown},
		{in: "plain", want: StylePlain},
		{in: "", want: StylePlain},
		{in: "weird", want: StylePlain},
	}
	for _, tc := range tests {
		if got := ParseStyle(tc.in); got != tc.want {
			t.Errorf("ParseStyle(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
