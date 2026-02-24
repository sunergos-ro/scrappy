package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"scrappy/pkg/client"
)

func TestRunHTMLCommand(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/html" {
			t.Fatalf("expected /html, got %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"url":     "https://example.com",
			"html":    "<html>ok</html>",
			"took_ms": 12,
		})
	}))
	defer server.Close()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run(context.Background(), []string{
		"--base-url", server.URL,
		"html",
		"--url", "https://example.com",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d (stderr=%s)", code, stderr.String())
	}

	var output map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		t.Fatalf("failed to decode output: %v", err)
	}
	if got := output["html"]; got != "<html>ok</html>" {
		t.Fatalf("unexpected html output: %#v", got)
	}
}

func TestRunStatsCommandUsesAdminToken(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/stats" {
			t.Fatalf("expected /stats, got %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Fatalf("expected bearer token, got %q", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
	}))
	defer server.Close()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run(context.Background(), []string{
		"--base-url", server.URL,
		"--admin-token", "test-token",
		"stats",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d (stderr=%s)", code, stderr.String())
	}
}

func TestRunScreenshotPassesDeviceScaleFactor(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/screenshot" {
			t.Fatalf("expected /screenshot, got %s", r.URL.Path)
		}

		var req client.ScreenshotRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if req.URL != "https://example.com" {
			t.Fatalf("unexpected URL: %q", req.URL)
		}
		if req.DeviceScaleFactor != 2.5 {
			t.Fatalf("expected device scale factor 2.5, got %v", req.DeviceScaleFactor)
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"url":        req.URL,
			"bucket":     "bucket",
			"key":        "key",
			"public_url": "https://cdn.example.com/key",
			"bytes":      12345,
			"format":     "png",
			"width":      1440,
			"height":     756,
			"took_ms":    99,
		})
	}))
	defer server.Close()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run(context.Background(), []string{
		"--base-url", server.URL,
		"screenshot",
		"--url", "https://example.com",
		"--format", "png",
		"--device-scale-factor", "2.5",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d (stderr=%s)", code, stderr.String())
	}
}

func TestRunScaleRequiresSize(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run(context.Background(), []string{"scale"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected exit code 2, got %d", code)
	}
	if !strings.Contains(stderr.String(), "--size is required") {
		t.Fatalf("expected size error, got: %s", stderr.String())
	}
}
