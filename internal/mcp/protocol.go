package mcp

import "encoding/json"

// Protocol surfaces: initialize, notifications/initialized, tools/*, prompts/*, resources/*.

const protocolVersion = "2024-11-05"

// Request is a single newline-delimited JSON-RPC 2.0 message. An absent ID
// marks a notification, which receives no reply.
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// Response is the reply to a Request carrying an ID.
type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

// RPCError is a JSON-RPC 2.0 error object with the standard code space:
// -32601 method not found, -32602 invalid params, -32603 internal error.
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// HasID reports whether the request expects a reply.
func (r Request) HasID() bool {
	return len(r.ID) > 0
}

// Tool describes one callable tool for tools/list.
type Tool struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	InputSchema Schema `json:"inputSchema"`
}

// Schema is the JSON Schema object constraining a tool's arguments.
type Schema struct {
	Type       string              `json:"type"`
	Properties map[string]Property `json:"properties,omitempty"`
	Required   []string            `json:"required,omitempty"`
}

// Property is a single named argument's schema.
type Property struct {
	Type        string   `json:"type"`
	Description string   `json:"description,omitempty"`
	Enum        []string `json:"enum,omitempty"`
}

// InitializeResult is the response payload for initialize.
type InitializeResult struct {
	ProtocolVersion string       `json:"protocolVersion"`
	Capabilities    Capabilities `json:"capabilities"`
	ServerInfo      ServerInfo   `json:"serverInfo"`
	Instructions    string       `json:"instructions,omitempty"`
}

// Capabilities advertises the server capabilities.
type Capabilities struct {
	Tools     map[string]any `json:"tools"`
	Prompts   map[string]any `json:"prompts"`
	Resources map[string]any `json:"resources"`
}

// ServerInfo identifies this server to clients.
type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// ToolsListResult wraps tools/list output.
type ToolsListResult struct {
	Tools []Tool `json:"tools"`
}

// CallParams are the tools/call parameters: which tool plus its arguments.
type CallParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments,omitempty"`
}

// CallResult is the tools/call payload: human-readable content blocks.
type CallResult struct {
	Content []ContentBlock `json:"content"`
	IsError bool           `json:"isError,omitempty"`
}

// ContentBlock is one text block in a tool result.
type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// Prompt describes one available prompt template for prompts/list.
type Prompt struct {
	Name        string           `json:"name"`
	Description string           `json:"description,omitempty"`
	Arguments   []PromptArgument `json:"arguments,omitempty"`
}

// PromptArgument describes an argument for a prompt template.
type PromptArgument struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
}

// PromptsListResult wraps prompts/list output.
type PromptsListResult struct {
	Prompts []Prompt `json:"prompts"`
}

// PromptGetParams are the prompts/get parameters.
type PromptGetParams struct {
	Name      string            `json:"name"`
	Arguments map[string]string `json:"arguments,omitempty"`
}

// PromptGetResult is the response payload for prompts/get.
type PromptGetResult struct {
	Description string          `json:"description,omitempty"`
	Messages    []PromptMessage `json:"messages"`
}

// PromptMessage is one message in a prompt result.
type PromptMessage struct {
	Role    string       `json:"role"`
	Content ContentBlock `json:"content"`
}

// Resource describes an inspectable resource for resources/list.
type Resource struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	MIMEType    string `json:"mimeType,omitempty"`
}

// ResourcesListResult wraps resources/list output.
type ResourcesListResult struct {
	Resources []Resource `json:"resources"`
}

// ResourceReadParams are the resources/read parameters.
type ResourceReadParams struct {
	URI string `json:"uri"`
}

// ResourceReadResult is the response payload for resources/read.
type ResourceReadResult struct {
	Contents []ResourceContent `json:"contents"`
}

// ResourceContent contains the data of a read resource.
type ResourceContent struct {
	URI      string `json:"uri"`
	MIMEType string `json:"mimeType,omitempty"`
	Text     string `json:"text,omitempty"`
}
