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
	caps, _ := result["capabilities"].(map[string]any)
	if caps["tools"] == nil || caps["prompts"] == nil || caps["resources"] == nil {
		t.Errorf("capabilities missing tools, prompts, or resources: %v", caps)
	}
	if instr, _ := result["instructions"].(string); !strings.Contains(instr, "sift-project-dotfile") {
		t.Errorf("expected instructions to mention sift-project-dotfile: %v", instr)
	}
}

func TestToolsListHasExpectedTools(t *testing.T) {
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
	for _, want := range []string{"pack_context", "pack_diff", "pack_follow", "list_tree", "get_skill"} {
		if !names[want] {
			t.Errorf("tools/list missing %q", want)
		}
	}
}

func TestPromptsListAndGet(t *testing.T) {
	in := strings.Join([]string{
		`{"jsonrpc":"2.0","id":10,"method":"prompts/list"}`,
		`{"jsonrpc":"2.0","id":11,"method":"prompts/get","params":{"name":"sift-project-dotfile"}}`,
	}, "\n")
	var out bytes.Buffer
	if err := Run(context.Background(), strings.NewReader(in), &out, testConfig()); err != nil {
		t.Fatal(err)
	}
	resps := decodeResponses(t, out.String())
	if len(resps) != 2 {
		t.Fatalf("responses = %d, want 2", len(resps))
	}

	// Check prompts/list
	resList, _ := resps[0]["result"].(map[string]any)
	prompts, _ := resList["prompts"].([]any)
	if len(prompts) == 0 {
		t.Fatalf("expected at least 1 prompt in prompts/list")
	}
	p0 := prompts[0].(map[string]any)
	if p0["name"] != "sift-project-dotfile" {
		t.Errorf("prompt[0].name = %v", p0["name"])
	}

	// Check prompts/get
	resGet, _ := resps[1]["result"].(map[string]any)
	msgs, _ := resGet["messages"].([]any)
	if len(msgs) == 0 {
		t.Fatalf("expected messages in prompts/get response")
	}
	m0 := msgs[0].(map[string]any)
	content := m0["content"].(map[string]any)
	text := content["text"].(string)
	if !strings.Contains(text, "Sift Project Dotfile Authoring") {
		t.Errorf("prompt text missing expected content")
	}
}

func TestResourcesListAndRead(t *testing.T) {
	in := strings.Join([]string{
		`{"jsonrpc":"2.0","id":20,"method":"resources/list"}`,
		`{"jsonrpc":"2.0","id":21,"method":"resources/read","params":{"uri":"skill://sift-project-dotfile"}}`,
	}, "\n")
	var out bytes.Buffer
	if err := Run(context.Background(), strings.NewReader(in), &out, testConfig()); err != nil {
		t.Fatal(err)
	}
	resps := decodeResponses(t, out.String())
	if len(resps) != 2 {
		t.Fatalf("responses = %d, want 2", len(resps))
	}

	// Check resources/list
	resList, _ := resps[0]["result"].(map[string]any)
	resources, _ := resList["resources"].([]any)
	if len(resources) == 0 {
		t.Fatalf("expected at least 1 resource in resources/list")
	}
	r0 := resources[0].(map[string]any)
	if r0["uri"] != "skill://sift-project-dotfile" {
		t.Errorf("resource[0].uri = %v", r0["uri"])
	}

	// Check resources/read
	resRead, _ := resps[1]["result"].(map[string]any)
	contents, _ := resRead["contents"].([]any)
	if len(contents) == 0 {
		t.Fatalf("expected contents in resources/read response")
	}
	c0 := contents[0].(map[string]any)
	text := c0["text"].(string)
	if !strings.Contains(text, "name: sift-project-dotfile") {
		t.Errorf("resource text missing expected content")
	}
}

func TestToolCallGetSkill(t *testing.T) {
	in := `{"jsonrpc":"2.0","id":30,"method":"tools/call","params":{"name":"get_skill","arguments":{}}}`
	var out bytes.Buffer
	if err := Run(context.Background(), strings.NewReader(in), &out, testConfig()); err != nil {
		t.Fatal(err)
	}
	resps := decodeResponses(t, out.String())
	if len(resps) != 1 {
		t.Fatalf("responses = %d, want 1", len(resps))
	}
	result, _ := resps[0]["result"].(map[string]any)
	contentArr, _ := result["content"].([]any)
	if len(contentArr) == 0 {
		t.Fatalf("expected content blocks in get_skill call")
	}
	block := contentArr[0].(map[string]any)
	text := block["text"].(string)
	if !strings.Contains(text, "Sift Project Dotfile Authoring") {
		t.Errorf("get_skill content missing expected title")
	}
}

func TestUnknownMethodAndTool(t *testing.T) {
	in := strings.Join([]string{
		`{"jsonrpc":"2.0","id":3,"method":"unknown/method"}`,
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
