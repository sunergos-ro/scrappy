package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"scrappy/pkg/client"
)

const (
	defaultMCPProtocolVersion = "2024-11-05"
	serverName                = "scrappy-mcp"
	serverVersion             = "0.4.0"
)

const (
	toolHTML       = "scrappy_html"
	toolMarkdown   = "scrappy_markdown"
	toolScreenshot = "scrappy_screenshot"
	toolStats      = "scrappy_stats"
	toolScale      = "scrappy_scale"
)

type mcpServer struct {
	client *client.Client
}

type toolDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	InputSchema map[string]any `json:"inputSchema"`
}

type toolCallResult struct {
	Content           []toolContent `json:"content"`
	StructuredContent any           `json:"structuredContent,omitempty"`
	IsError           bool          `json:"isError,omitempty"`
}

type toolContent struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

func newMCPServer(c *client.Client) *mcpServer {
	return &mcpServer{client: c}
}

func (s *mcpServer) handleRequest(ctx context.Context, req rpcRequest) *rpcResponse {
	if req.JSONRPC != "" && req.JSONRPC != jsonRPCVersion {
		return s.errorResponse(req.ID, -32600, "invalid request: jsonrpc must be 2.0", nil)
	}

	switch req.Method {
	case "initialize":
		if !req.hasID() {
			return nil
		}
		return s.handleInitialize(req)
	case "notifications/initialized":
		return nil
	case "ping":
		if !req.hasID() {
			return nil
		}
		return s.resultResponse(req.ID, map[string]any{})
	case "tools/list":
		if !req.hasID() {
			return nil
		}
		return s.resultResponse(req.ID, map[string]any{"tools": toolDefinitions()})
	case "tools/call":
		if !req.hasID() {
			return nil
		}
		return s.handleToolCall(ctx, req)
	default:
		if !req.hasID() {
			return nil
		}
		return s.errorResponse(req.ID, -32601, "method not found", nil)
	}
}

func (s *mcpServer) handleInitialize(req rpcRequest) *rpcResponse {
	type initializeParams struct {
		ProtocolVersion string `json:"protocolVersion"`
	}

	params := initializeParams{}
	if len(req.Params) > 0 {
		if err := decodeStrict(req.Params, &params); err != nil {
			return s.errorResponse(req.ID, -32602, "invalid params", err.Error())
		}
	}

	version := strings.TrimSpace(params.ProtocolVersion)
	if version == "" {
		version = defaultMCPProtocolVersion
	}

	result := map[string]any{
		"protocolVersion": version,
		"capabilities": map[string]any{
			"tools": map[string]any{},
		},
		"serverInfo": map[string]any{
			"name":    serverName,
			"version": serverVersion,
		},
		"instructions": "Use scrappy_* tools to render HTML, extract markdown, take screenshots, and manage pool stats/size.",
	}
	return s.resultResponse(req.ID, result)
}

func (s *mcpServer) handleToolCall(ctx context.Context, req rpcRequest) *rpcResponse {
	type callParams struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments,omitempty"`
	}

	params := callParams{}
	if err := decodeStrict(req.Params, &params); err != nil {
		return s.errorResponse(req.ID, -32602, "invalid params", err.Error())
	}
	if strings.TrimSpace(params.Name) == "" {
		return s.errorResponse(req.ID, -32602, "invalid params", "name is required")
	}

	switch params.Name {
	case toolHTML:
		return s.callHTML(ctx, req.ID, params.Arguments)
	case toolMarkdown:
		return s.callMarkdown(ctx, req.ID, params.Arguments)
	case toolScreenshot:
		return s.callScreenshot(ctx, req.ID, params.Arguments)
	case toolStats:
		return s.callStats(ctx, req.ID, params.Arguments)
	case toolScale:
		return s.callScale(ctx, req.ID, params.Arguments)
	default:
		return s.errorResponse(req.ID, -32602, "invalid params", "unknown tool name")
	}
}

func (s *mcpServer) callHTML(ctx context.Context, id json.RawMessage, rawArgs json.RawMessage) *rpcResponse {
	args := renderToolArgs{}
	if err := decodeStrictObject(rawArgs, &args); err != nil {
		return s.errorResponse(id, -32602, "invalid params", err.Error())
	}
	urlValue := strings.TrimSpace(args.URL)
	if urlValue == "" {
		return s.errorResponse(id, -32602, "invalid params", "url is required")
	}

	resp, err := s.client.HTML(ctx, client.RenderRequest{
		URL:       urlValue,
		Viewport:  buildViewport(args.Viewport, args.ViewportWidth, args.ViewportHeight),
		UserAgent: strings.TrimSpace(args.UserAgent),
		WaitMS:    args.WaitMS,
		TimeoutMS: args.TimeoutMS,
	})
	if err != nil {
		return s.resultResponse(id, errorToolResult(err))
	}
	return s.resultResponse(id, successToolResult(resp))
}

func (s *mcpServer) callMarkdown(ctx context.Context, id json.RawMessage, rawArgs json.RawMessage) *rpcResponse {
	args := renderToolArgs{}
	if err := decodeStrictObject(rawArgs, &args); err != nil {
		return s.errorResponse(id, -32602, "invalid params", err.Error())
	}
	urlValue := strings.TrimSpace(args.URL)
	if urlValue == "" {
		return s.errorResponse(id, -32602, "invalid params", "url is required")
	}

	resp, err := s.client.Markdown(ctx, client.MarkdownRequest{
		URL:       urlValue,
		Viewport:  buildViewport(args.Viewport, args.ViewportWidth, args.ViewportHeight),
		UserAgent: strings.TrimSpace(args.UserAgent),
		WaitMS:    args.WaitMS,
		TimeoutMS: args.TimeoutMS,
	})
	if err != nil {
		return s.resultResponse(id, errorToolResult(err))
	}
	return s.resultResponse(id, successToolResult(resp))
}

func (s *mcpServer) callScreenshot(ctx context.Context, id json.RawMessage, rawArgs json.RawMessage) *rpcResponse {
	args := screenshotToolArgs{}
	if err := decodeStrictObject(rawArgs, &args); err != nil {
		return s.errorResponse(id, -32602, "invalid params", err.Error())
	}
	urlValue := strings.TrimSpace(args.URL)
	if urlValue == "" {
		return s.errorResponse(id, -32602, "invalid params", "url is required")
	}

	resp, err := s.client.Screenshot(ctx, client.ScreenshotRequest{
		URL:               urlValue,
		Viewport:          buildViewport(args.Viewport, args.ViewportWidth, args.ViewportHeight),
		UserAgent:         strings.TrimSpace(args.UserAgent),
		Format:            strings.TrimSpace(args.Format),
		Quality:           args.Quality,
		DeviceScaleFactor: args.DeviceScaleFactor,
		WaitMS:            args.WaitMS,
		TimeoutMS:         args.TimeoutMS,
	})
	if err != nil {
		return s.resultResponse(id, errorToolResult(err))
	}
	return s.resultResponse(id, successToolResult(resp))
}

func (s *mcpServer) callStats(ctx context.Context, id json.RawMessage, rawArgs json.RawMessage) *rpcResponse {
	if err := rejectNonEmptyArgs(rawArgs); err != nil {
		return s.errorResponse(id, -32602, "invalid params", err.Error())
	}

	resp, err := s.client.Stats(ctx)
	if err != nil {
		return s.resultResponse(id, errorToolResult(err))
	}
	return s.resultResponse(id, successToolResult(resp))
}

func (s *mcpServer) callScale(ctx context.Context, id json.RawMessage, rawArgs json.RawMessage) *rpcResponse {
	args := scaleToolArgs{}
	if err := decodeStrictObject(rawArgs, &args); err != nil {
		return s.errorResponse(id, -32602, "invalid params", err.Error())
	}
	if args.Size < 0 {
		return s.errorResponse(id, -32602, "invalid params", "size must be >= 0")
	}

	resp, err := s.client.Scale(ctx, client.ScaleRequest{Size: args.Size})
	if err != nil {
		return s.resultResponse(id, errorToolResult(err))
	}
	return s.resultResponse(id, successToolResult(resp))
}

type renderToolArgs struct {
	URL            string           `json:"url"`
	UserAgent      string           `json:"user_agent,omitempty"`
	WaitMS         int              `json:"wait_ms,omitempty"`
	TimeoutMS      int              `json:"timeout_ms,omitempty"`
	ViewportWidth  int              `json:"viewport_width,omitempty"`
	ViewportHeight int              `json:"viewport_height,omitempty"`
	Viewport       *client.Viewport `json:"viewport,omitempty"`
}

type screenshotToolArgs struct {
	URL               string           `json:"url"`
	UserAgent         string           `json:"user_agent,omitempty"`
	Format            string           `json:"format,omitempty"`
	Quality           int              `json:"quality,omitempty"`
	DeviceScaleFactor float64          `json:"device_scale_factor,omitempty"`
	WaitMS            int              `json:"wait_ms,omitempty"`
	TimeoutMS         int              `json:"timeout_ms,omitempty"`
	ViewportWidth     int              `json:"viewport_width,omitempty"`
	ViewportHeight    int              `json:"viewport_height,omitempty"`
	Viewport          *client.Viewport `json:"viewport,omitempty"`
}

type scaleToolArgs struct {
	Size int `json:"size"`
}

func buildViewport(existing *client.Viewport, width int, height int) *client.Viewport {
	if existing != nil {
		return existing
	}
	if width <= 0 && height <= 0 {
		return nil
	}
	return &client.Viewport{
		Width:  width,
		Height: height,
	}
}

func decodeStrict(raw json.RawMessage, target any) error {
	if len(raw) == 0 {
		raw = []byte("{}")
	}

	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != nil && !errors.Is(err, io.EOF) {
		return errors.New("unexpected trailing data")
	}
	return nil
}

func decodeStrictObject(raw json.RawMessage, target any) error {
	if len(raw) == 0 {
		raw = []byte("{}")
	}
	if !json.Valid(raw) {
		return errors.New("arguments must be valid JSON")
	}
	return decodeStrict(raw, target)
}

func rejectNonEmptyArgs(raw json.RawMessage) error {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil
	}
	var decoded map[string]any
	if err := decodeStrict(raw, &decoded); err != nil {
		return err
	}
	if len(decoded) > 0 {
		return errors.New("this tool does not accept arguments")
	}
	return nil
}

func successToolResult(payload any) toolCallResult {
	return toolCallResult{
		Content: []toolContent{
			{
				Type: "text",
				Text: stringifyJSON(payload),
			},
		},
		StructuredContent: payload,
	}
}

func errorToolResult(err error) toolCallResult {
	message := strings.TrimSpace(err.Error())
	if message == "" {
		message = "unknown error"
	}

	structured := map[string]any{
		"error": message,
	}

	var apiErr *client.APIError
	if errors.As(err, &apiErr) {
		structured["status_code"] = apiErr.StatusCode
		structured["body"] = apiErr.Body
	}

	return toolCallResult{
		Content: []toolContent{
			{
				Type: "text",
				Text: stringifyJSON(structured),
			},
		},
		StructuredContent: structured,
		IsError:           true,
	}
}

func stringifyJSON(value any) string {
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Sprintf("%v", value)
	}
	return string(encoded)
}

func toolDefinitions() []toolDefinition {
	return []toolDefinition{
		{
			Name:        toolHTML,
			Description: "Render page HTML from a URL.",
			InputSchema: renderInputSchema(),
		},
		{
			Name:        toolMarkdown,
			Description: "Extract markdown-like content from a URL.",
			InputSchema: renderInputSchema(),
		},
		{
			Name:        toolScreenshot,
			Description: "Capture a screenshot and return the uploaded URL.",
			InputSchema: screenshotInputSchema(),
		},
		{
			Name:        toolStats,
			Description: "Get browser pool stats.",
			InputSchema: map[string]any{
				"type":                 "object",
				"additionalProperties": false,
			},
		},
		{
			Name:        toolScale,
			Description: "Scale browser pool size.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"size": map[string]any{
						"type":    "integer",
						"minimum": 0,
					},
				},
				"required":             []string{"size"},
				"additionalProperties": false,
			},
		},
	}
}

func renderInputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"url": map[string]any{
				"type":        "string",
				"description": "Absolute URL (http/https).",
			},
			"user_agent": map[string]any{
				"type":        "string",
				"description": "Optional User-Agent override.",
			},
			"wait_ms": map[string]any{
				"type":        "integer",
				"minimum":     0,
				"description": "Optional wait time before extraction.",
			},
			"timeout_ms": map[string]any{
				"type":        "integer",
				"minimum":     1,
				"description": "Optional request timeout.",
			},
			"viewport_width": map[string]any{
				"type":        "integer",
				"minimum":     1,
				"description": "Optional viewport width.",
			},
			"viewport_height": map[string]any{
				"type":        "integer",
				"minimum":     1,
				"description": "Optional viewport height.",
			},
			"viewport": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"width": map[string]any{
						"type":    "integer",
						"minimum": 1,
					},
					"height": map[string]any{
						"type":    "integer",
						"minimum": 1,
					},
				},
				"additionalProperties": false,
			},
		},
		"required":             []string{"url"},
		"additionalProperties": false,
	}
}

func screenshotInputSchema() map[string]any {
	schema := renderInputSchema()
	props, _ := schema["properties"].(map[string]any)
	props["format"] = map[string]any{
		"type":        "string",
		"enum":        []string{"jpeg", "png", "webp"},
		"description": "Screenshot format.",
	}
	props["quality"] = map[string]any{
		"type":        "integer",
		"minimum":     1,
		"maximum":     100,
		"description": "JPEG/WebP quality.",
	}
	props["device_scale_factor"] = map[string]any{
		"type":        "number",
		"minimum":     1,
		"description": "Optional screenshot device scale factor (DPR).",
	}
	return schema
}

func (s *mcpServer) resultResponse(id json.RawMessage, result any) *rpcResponse {
	return &rpcResponse{
		JSONRPC: jsonRPCVersion,
		ID:      id,
		Result:  result,
	}
}

func (s *mcpServer) errorResponse(id json.RawMessage, code int, message string, data any) *rpcResponse {
	return &rpcResponse{
		JSONRPC: jsonRPCVersion,
		ID:      id,
		Error: &rpcError{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}
}
