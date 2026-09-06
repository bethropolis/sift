package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/bethropolis/sift/internal/config"
)

// Run reads newline-delimited JSON-RPC requests from r and writes responses
// to w until EOF. No session state is retained across calls beyond the
// caller's *config.Config, which handlers treat as read-only per call —
// this server never mutates the process working directory or dump-state
// files (see the statelessness rule in tools.go).
func Run(ctx context.Context, r io.Reader, w io.Writer, cfg *config.Config) error {
	scanner := bufio.NewScanner(r)
	// Tool params can carry paths and prompts; keep the ceiling generous.
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	enc := json.NewEncoder(w)

	fmt.Fprintf(os.Stderr, "sift mcp %s — root: %s, %d tools ready\n", cfg.Version, cfg.RootDir, len(toolDefinitions()))

	for scanner.Scan() {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		var req Request
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
			continue // malformed line: keep the loop alive
		}
		resp := dispatch(ctx, req, cfg)
		if resp == nil {
			continue // notification (e.g. notifications/initialized): no reply
		}
		if err := enc.Encode(resp); err != nil {
			return err
		}
	}
	return scanner.Err()
}

// dispatch routes one request to its handler. It returns nil for
// notifications, which receive no reply.
func dispatch(ctx context.Context, req Request, cfg *config.Config) *Response {
	if !req.HasID() {
		return nil
	}
	reply := func(result any) *Response {
		return &Response{JSONRPC: "2.0", ID: req.ID, Result: result}
	}
	fail := func(code int, msg string) *Response {
		return &Response{JSONRPC: "2.0", ID: req.ID, Error: &RPCError{Code: code, Message: msg}}
	}

	switch req.Method {
	case "initialize":
		return reply(InitializeResult{
			ProtocolVersion: protocolVersion,
			Capabilities:    Capabilities{Tools: map[string]any{}},
			ServerInfo:      ServerInfo{Name: "sift", Version: cfg.Version},
		})
	case "tools/list":
		return reply(ToolsListResult{Tools: toolDefinitions()})
	case "tools/call":
		var params CallParams
		if len(req.Params) > 0 {
			if err := json.Unmarshal(req.Params, &params); err != nil {
				return fail(-32602, "invalid params: "+err.Error())
			}
		}
		if params.Name == "" {
			return fail(-32602, "missing tool name")
		}
		text, toolErr, ok := callTool(ctx, params.Name, params.Arguments, cfg)
		if !ok {
			return fail(-32601, "unknown tool: "+params.Name)
		}
		res := CallResult{Content: []ContentBlock{{Type: "text", Text: text}}}
		if toolErr != nil {
			res.Content[0].Text = toolErr.Error()
			res.IsError = true
		}
		return reply(res)
	default:
		return fail(-32601, "method not found: "+req.Method)
	}
}
