package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"scrappy/pkg/client"
)

func TestHandleInitialize(t *testing.T) {
	t.Parallel()

	srv := newMCPServer(mustClient(t, httptest.NewServer(http.NotFoundHandler())))

	resp := srv.handleRequest(context.Background(), rpcRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage("1"),
		Method:  "initialize",
		Params:  mustJSON(t, map[string]any{"protocolVersion": "2024-11-05"}),
	})
	if resp == nil {
		t.Fatal("expected response")
	}
	if resp.Error != nil {
		t.Fatalf("expected no error, got %v", resp.Error)
	}

	result, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatalf("expected map result, got %T", resp.Result)
	}
	if got := result["protocolVersion"]; got != "2024-11-05" {
		t.Fatalf("unexpected protocol version: %#v", got)
	}
	serverInfo, ok := result["serverInfo"].(map[string]any)
	if !ok {
		t.Fatalf("expected serverInfo map, got %T", result["serverInfo"])
	}
	if got := serverInfo["version"]; got != serverVersion {
		t.Fatalf("server version = %#v, want %#v", got, serverVersion)
	}
}

func TestToolsListContainsAllTools(t *testing.T) {
	t.Parallel()

	srv := newMCPServer(mustClient(t, httptest.NewServer(http.NotFoundHandler())))

	resp := srv.handleRequest(context.Background(), rpcRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage("1"),
		Method:  "tools/list",
	})
	if resp == nil || resp.Error != nil {
		t.Fatalf("expected successful response, got %#v", resp)
	}

	result, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatalf("expected map result, got %T", resp.Result)
	}
	tools, ok := result["tools"].([]toolDefinition)
	if !ok {
		t.Fatalf("expected []toolDefinition, got %T", result["tools"])
	}
	if len(tools) != 5 {
		t.Fatalf("expected 5 tools, got %d", len(tools))
	}
}

func TestToolsCallHTMLSuccess(t *testing.T) {
	t.Parallel()

	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/html" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"url":     "https://example.com",
			"html":    "<html>ok</html>",
			"took_ms": 10,
		})
	}))
	defer api.Close()

	srv := newMCPServer(mustClient(t, api))

	resp := srv.handleRequest(context.Background(), rpcRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage("99"),
		Method:  "tools/call",
		Params: mustJSON(t, map[string]any{
			"name": toolHTML,
			"arguments": map[string]any{
				"url": "https://example.com",
			},
		}),
	})
	if resp == nil || resp.Error != nil {
		t.Fatalf("expected successful response, got %#v", resp)
	}

	result, ok := resp.Result.(toolCallResult)
	if !ok {
		t.Fatalf("expected toolCallResult, got %T", resp.Result)
	}
	if result.IsError {
		t.Fatalf("expected non-error result: %#v", result)
	}
}

func TestToolsCallStatsRejectsUnexpectedArgs(t *testing.T) {
	t.Parallel()

	srv := newMCPServer(mustClient(t, httptest.NewServer(http.NotFoundHandler())))

	resp := srv.handleRequest(context.Background(), rpcRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage("4"),
		Method:  "tools/call",
		Params: mustJSON(t, map[string]any{
			"name":      toolStats,
			"arguments": map[string]any{"unexpected": true},
		}),
	})
	if resp == nil {
		t.Fatal("expected response")
	}
	if resp.Error == nil {
		t.Fatalf("expected error response, got %#v", resp)
	}
	if resp.Error.Code != -32602 {
		t.Fatalf("expected error code -32602, got %d", resp.Error.Code)
	}
}

func TestToolsCallMarkdownAPIFailureReturnsToolError(t *testing.T) {
	t.Parallel()

	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/markdown" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"url is required"}`))
	}))
	defer api.Close()

	srv := newMCPServer(mustClient(t, api))

	resp := srv.handleRequest(context.Background(), rpcRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage("7"),
		Method:  "tools/call",
		Params: mustJSON(t, map[string]any{
			"name": toolMarkdown,
			"arguments": map[string]any{
				"url": "https://example.com",
			},
		}),
	})
	if resp == nil || resp.Error != nil {
		t.Fatalf("expected tool response, got %#v", resp)
	}

	result, ok := resp.Result.(toolCallResult)
	if !ok {
		t.Fatalf("expected toolCallResult, got %T", resp.Result)
	}
	if !result.IsError {
		t.Fatalf("expected tool-level error result, got %#v", result)
	}
}

func TestToolsCallMarkdownPassesPrimeLazyContent(t *testing.T) {
	t.Parallel()

	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/markdown" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		var req client.MarkdownRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if !req.PrimeLazyContent {
			t.Fatal("expected prime_lazy_content to be true")
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"url":      req.URL,
			"markdown": "# Example",
			"took_ms":  7,
		})
	}))
	defer api.Close()

	srv := newMCPServer(mustClient(t, api))

	resp := srv.handleRequest(context.Background(), rpcRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage("8"),
		Method:  "tools/call",
		Params: mustJSON(t, map[string]any{
			"name": toolMarkdown,
			"arguments": map[string]any{
				"url":                "https://example.com",
				"prime_lazy_content": true,
			},
		}),
	})
	if resp == nil || resp.Error != nil {
		t.Fatalf("expected successful response, got %#v", resp)
	}
}

func TestToolsCallScreenshotPassesDeviceScaleFactor(t *testing.T) {
	t.Parallel()

	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/screenshot" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		var req client.ScreenshotRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if req.DeviceScaleFactor != 2 {
			t.Fatalf("expected device scale factor 2, got %v", req.DeviceScaleFactor)
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"url":        req.URL,
			"bucket":     "bucket",
			"key":        "key",
			"public_url": "https://cdn.example.com/key",
			"bytes":      100,
			"format":     "jpeg",
			"width":      1200,
			"height":     700,
			"took_ms":    5,
		})
	}))
	defer api.Close()

	srv := newMCPServer(mustClient(t, api))

	resp := srv.handleRequest(context.Background(), rpcRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage("13"),
		Method:  "tools/call",
		Params: mustJSON(t, map[string]any{
			"name": toolScreenshot,
			"arguments": map[string]any{
				"url":                 "https://example.com",
				"device_scale_factor": 2,
			},
		}),
	})
	if resp == nil || resp.Error != nil {
		t.Fatalf("expected successful response, got %#v", resp)
	}
}

func TestToolDefinitionsMarkdownSchemaIncludesPrimeLazyContent(t *testing.T) {
	t.Parallel()

	for _, tool := range toolDefinitions() {
		if tool.Name != toolMarkdown {
			continue
		}

		props, ok := tool.InputSchema["properties"].(map[string]any)
		if !ok {
			t.Fatalf("expected properties map, got %T", tool.InputSchema["properties"])
		}
		if _, ok := props["prime_lazy_content"]; !ok {
			t.Fatal("expected markdown schema to include prime_lazy_content")
		}
		return
	}

	t.Fatal("markdown tool definition not found")
}

func mustClient(t *testing.T, server *httptest.Server) *client.Client {
	t.Helper()
	t.Cleanup(server.Close)

	c, err := client.New(server.URL, client.WithHTTPClient(server.Client()))
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	return c
}

func mustJSON(t *testing.T, value any) json.RawMessage {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("failed to marshal JSON: %v", err)
	}
	return json.RawMessage(encoded)
}
