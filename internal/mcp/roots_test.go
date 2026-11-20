package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPingReply(t *testing.T) {
	in := `{"jsonrpc":"2.0","id":1,"method":"ping"}`
	var out bytes.Buffer
	if err := Run(context.Background(), strings.NewReader(in), &out, testConfig()); err != nil {
		t.Fatal(err)
	}
	resps := decodeResponses(t, out.String())
	if len(resps) != 1 {
		t.Fatalf("responses = %d, want 1", len(resps))
	}
	if _, ok := resps[0]["result"].(map[string]any); !ok {
		t.Fatalf("ping result = %v, want an empty object", resps[0]["result"])
	}
}

func TestRootsListReplacesActiveRoot(t *testing.T) {
	serverRoot := t.TempDir()
	clientRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(serverRoot, "server.go"), []byte("package server\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(clientRoot, "client.go"), []byte("package client\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := testConfig()
	cfg.RootDir = serverRoot

	uri := (&url.URL{Scheme: "file", Path: filepath.ToSlash(clientRoot)}).String()
	rootsReply, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      "sift-roots-1",
		"result": map[string]any{
			"roots": []map[string]any{{"uri": uri, "name": "client"}},
		},
	})
	in := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{"roots":{"listChanged":true}}}}`,
		string(rootsReply),
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"list_tree"}}`,
	}, "\n")
	var out bytes.Buffer
	if err := Run(context.Background(), strings.NewReader(in), &out, cfg); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	if !strings.Contains(text, "client.go") {
		t.Fatalf("list_tree did not use client roots:\n%s", text)
	}
	if strings.Contains(text, "server.go") {
		t.Fatalf("list_tree kept the startup root after roots/list:\n%s", text)
	}
	if !strings.Contains(text, `"method":"roots/list"`) {
		t.Fatalf("server did not request roots/list:\n%s", text)
	}
}

func TestToolPathRejectsTraversalAndOutsideRoots(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret.go"), []byte("package secret\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := newSession(mcpTestConfig(root))

	if _, err := s.scopeRoot(map[string]any{"path": "../"}); err == nil {
		t.Error("expected traversal to be rejected")
	}
	if _, err := s.scopeRoot(map[string]any{"path": outside}); err == nil {
		t.Error("expected an absolute path outside the root to be rejected")
	}
	if _, err := handlePackFilesSession(context.Background(), map[string]any{
		"files": []any{map[string]any{"path": filepath.Join(outside, "secret.go")}},
	}, s); err == nil {
		t.Error("expected pack_files to reject a file outside the jail")
	}
}

func TestRootsListChangedRefreshesRoots(t *testing.T) {
	rootA := t.TempDir()
	rootB := t.TempDir()
	if err := os.WriteFile(filepath.Join(rootA, "a.go"), []byte("package a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rootB, "b.go"), []byte("package b\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := testConfig()
	cfg.RootDir = rootA
	reply := func(id, root string) string {
		uri := (&url.URL{Scheme: "file", Path: filepath.ToSlash(root)}).String()
		b, _ := json.Marshal(map[string]any{
			"jsonrpc": "2.0", "id": id,
			"result": map[string]any{"roots": []map[string]any{{"uri": uri}}},
		})
		return string(b)
	}
	in := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"capabilities":{"roots":{}}}}`,
		reply("sift-roots-1", rootA),
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"list_tree"}}`,
		`{"jsonrpc":"2.0","method":"notifications/roots/list_changed"}`,
		reply("sift-roots-2", rootB),
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"list_tree"}}`,
	}, "\n")
	var out bytes.Buffer
	if err := Run(context.Background(), strings.NewReader(in), &out, cfg); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(out.String(), "\n")
	var calls []string
	for _, line := range lines {
		if strings.Contains(line, `"id":2`) || strings.Contains(line, `"id":3`) {
			calls = append(calls, line)
		}
	}
	if len(calls) != 2 {
		t.Fatalf("tool responses = %d, want 2:\n%s", len(calls), out.String())
	}
	if !strings.Contains(calls[0], "a.go") || strings.Contains(calls[0], "b.go") {
		t.Fatalf("first list_tree used the wrong root: %s", calls[0])
	}
	if !strings.Contains(calls[1], "b.go") || strings.Contains(calls[1], "a.go") {
		t.Fatalf("refreshed list_tree used the wrong root: %s", calls[1])
	}
}
