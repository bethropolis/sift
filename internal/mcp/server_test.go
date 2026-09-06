package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/bethropolis/sift/internal/config"
)

func testConfig() *config.Config {
	cfg := config.New()
	cfg.Version = "test"
	cfg.Quiet = true
	return cfg
}

func decodeResponses(t *testing.T, out string) []map[string]any {
	t.Helper()
	var resps []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if line == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("response line is not JSON: %v\n%s", err, line)
		}
		resps = append(resps, m)
	}
	return resps
}

func TestInitializeReply(t *testing.T) {
	in := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}`
	var out bytes.Buffer
	if err := Run(context.Background(), strings.NewReader(in), &out, testConfig()); err != nil {
		t.Fatal(err)
	}
	resps := decodeResponses(t, out.String())
	if len(resps) != 1 {
		t.Fatalf("responses = %d, want 1", len(resps))
	}
	result, _ := resps[0]["result"].(map[string]any)
	if result["protocolVersion"] != "2024-11-05" {
		t.Errorf("protocolVersion = %v", result["protocolVersion"])
	}
	info, _ := result["serverInfo"].(map[string]any)
	if info["name"] != "sift" {
		t.Errorf("serverInfo.name = %v", info["name"])
	}
}

func TestToolsListHasThreeTools(t *testing.T) {
	in := `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`
	var out bytes.Buffer
	if err := Run(context.Background(), strings.NewReader(in), &out, testConfig()); err != nil {
		t.Fatal(err)
	}
	resps := decodeResponses(t, out.String())
	result, _ := resps[0]["result"].(map[string]any)
	tools, _ := result["tools"].([]any)
	names := map[string]bool{}
	for _, tl := range tools {
		names[tl.(map[string]any)["name"].(string)] = true
	}
	for _, want := range []string{"pack_context", "pack_diff", "list_tree"} {
		if !names[want] {
			t.Errorf("tools/list missing %q", want)
		}
	}
}

func TestUnknownMethodAndTool(t *testing.T) {
	in := strings.Join([]string{
		`{"jsonrpc":"2.0","id":3,"method":"resources/list"}`,
		`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"nope"}}`,
		`{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{}}`,
	}, "\n")
	var out bytes.Buffer
	if err := Run(context.Background(), strings.NewReader(in), &out, testConfig()); err != nil {
		t.Fatal(err)
	}
	resps := decodeResponses(t, out.String())
	if len(resps) != 3 {
		t.Fatalf("responses = %d, want 3", len(resps))
	}
	for _, r := range resps {
		errObj, _ := r["error"].(map[string]any)
		if errObj == nil {
			t.Errorf("response has no error: %v", r)
			continue
		}
		if code, _ := errObj["code"].(float64); code != -32601 && code != -32602 {
			t.Errorf("code = %v, want -32601 or -32602", code)
		}
	}
}

func TestMalformedLinesSkippedAndNotificationsSilent(t *testing.T) {
	in := strings.Join([]string{
		`not json at all`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":6,"method":"tools/list"}`,
	}, "\n")
	var out bytes.Buffer
	if err := Run(context.Background(), strings.NewReader(in), &out, testConfig()); err != nil {
		t.Fatal(err)
	}
	resps := decodeResponses(t, out.String())
	if len(resps) != 1 {
		t.Fatalf("responses = %d, want 1 (malformed skipped, notification silent)", len(resps))
	}
}
