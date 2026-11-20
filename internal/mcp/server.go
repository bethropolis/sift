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

	sess := newSession(cfg)
	fmt.Fprintf(os.Stderr, "sift mcp %s — root: %s, %d tools ready\n", cfg.Version, cfg.RootDir, len(toolDefinitions()))

	for scanner.Scan() {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		var req Request
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
			continue // malformed line: keep the loop alive
		}
		// A message without a method is a reply to a server-initiated
		// request. The only one sift sends is roots/list.
		if req.Method == "" {
			var replyID string
			_ = json.Unmarshal(req.ID, &replyID)
			if sess.pendingRootsID != "" && replyID == sess.pendingRootsID {
				if err := sess.handleRootsResult(rawResult(scanner.Bytes())); err != nil {
					return err
				}
			}
			continue
		}
		if req.Method == "notifications/roots/list_changed" {
			if sess.rootsSupported {
				if err := sess.sendRootsRequest(enc); err != nil {
					return err
				}
			}
			continue
		}
		resp := dispatch(ctx, req, sess)
		if resp == nil {
			continue // notification (e.g. notifications/initialized): no reply
		}
		if err := enc.Encode(resp); err != nil {
			return err
		}
		if sess.rootsSupported && !sess.rootsRequested {
			if err := sess.sendRootsRequest(enc); err != nil {
				return err
			}
		}
	}
	return scanner.Err()
}

// rawResult extracts the JSON-RPC "result" field from a raw message.
func rawResult(msg []byte) json.RawMessage {
	var envelope struct {
		Result json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(msg, &envelope); err != nil {
		return nil
	}
	return envelope.Result
}

// dispatch routes one request to its handler. It returns nil for
// notifications, which receive no reply.
func dispatch(ctx context.Context, req Request, s *session) *Response {
	cfg := s.cfg
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
		var params InitializeParams
		if len(req.Params) > 0 {
			_ = json.Unmarshal(req.Params, &params)
		}
		s.rootsSupported = params.Capabilities.Roots != nil
		return reply(InitializeResult{
			ProtocolVersion: protocolVersion,
			Capabilities: Capabilities{
				Tools:     map[string]any{},
				Prompts:   map[string]any{},
				Resources: map[string]any{},
			},
			ServerInfo:   ServerInfo{Name: "sift", Version: cfg.Version},
			Instructions: "Sift packs repository context for LLMs. If you need to author or update a .sift.toml configuration file for this project, use the 'sift-project-dotfile' prompt or call the 'get_skill' tool.",
		})
	case "ping":
		return reply(struct{}{})
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
		text, toolErr, ok := callTool(ctx, params.Name, params.Arguments, s)
		if !ok {
			return fail(-32601, "unknown tool: "+params.Name)
		}
		res := CallResult{Content: []ContentBlock{{Type: "text", Text: text}}}
		if toolErr != nil {
			res.Content[0].Text = toolErr.Error()
			res.IsError = true
		}
		return reply(res)
	case "prompts/list":
		return reply(PromptsListResult{Prompts: promptDefinitions()})
	case "prompts/get":
		var params PromptGetParams
		if len(req.Params) > 0 {
			if err := json.Unmarshal(req.Params, &params); err != nil {
				return fail(-32602, "invalid params: "+err.Error())
			}
		}
		if params.Name == "" {
			return fail(-32602, "missing prompt name")
		}
		res, err := handleGetPrompt(params.Name)
		if err != nil {
			return fail(-32602, err.Error())
		}
		return reply(res)
	case "resources/list":
		return reply(ResourcesListResult{Resources: resourceDefinitions()})
	case "resources/read":
		var params ResourceReadParams
		if len(req.Params) > 0 {
			if err := json.Unmarshal(req.Params, &params); err != nil {
				return fail(-32602, "invalid params: "+err.Error())
			}
		}
		if params.URI == "" {
			return fail(-32602, "missing resource URI")
		}
		res, err := handleReadResource(params.URI)
		if err != nil {
			return fail(-32602, err.Error())
		}
		return reply(res)
	default:
		return fail(-32601, "method not found: "+req.Method)
	}
}
