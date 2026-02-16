package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewRejectsInvalidBaseURL(t *testing.T) {
	t.Parallel()

	_, err := New("localhost:3000")
	if err == nil {
		t.Fatal("expected an error for non-absolute base URL")
	}
}

func TestHTMLSendsRequestAndDecodesResponse(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected method POST, got %s", r.Method)
		}
		if r.URL.Path != "/html" {
			t.Fatalf("expected path /html, got %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer top-secret" {
			t.Fatalf("expected Authorization header, got %q", got)
		}

		var req RenderRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if req.URL != "https://example.com" {
			t.Fatalf("unexpected request URL: %s", req.URL)
		}

		_ = json.NewEncoder(w).Encode(RenderResponse{
			URL:    req.URL,
			HTML:   "<html></html>",
			TookMS: 42,
		})
	}))
	defer server.Close()

	c, err := New(server.URL, WithAdminToken("top-secret"), WithHTTPClient(server.Client()))
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	resp, err := c.HTML(context.Background(), RenderRequest{URL: "https://example.com"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if resp.HTML != "<html></html>" {
		t.Fatalf("unexpected HTML response: %s", resp.HTML)
	}
}

func TestMarkdownReturnsAPIError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"url is required"}`))
	}))
	defer server.Close()

	c, err := New(server.URL, WithHTTPClient(server.Client()))
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	_, err = c.Markdown(context.Background(), MarkdownRequest{})
	if err == nil {
		t.Fatal("expected an error")
	}

	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", apiErr.StatusCode)
	}
	if !strings.Contains(apiErr.Message, "url is required") {
		t.Fatalf("unexpected error message: %q", apiErr.Message)
	}
}
