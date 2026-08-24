package main

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRegisterRoutesUsesHTMLEndpoint(t *testing.T) {
	mux := http.NewServeMux()
	registerRoutes(mux, &Handlers{})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/html", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}

	if msg := readErrorMessage(t, rec); msg != "method not allowed" {
		t.Fatalf("error message = %q, want %q", msg, "method not allowed")
	}
}

func TestRegisterRoutesRenderEndpointRemoved(t *testing.T) {
	mux := http.NewServeMux()
	registerRoutes(mux, &Handlers{})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/render", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestGetClientIPTrustsForwardedHeadersOnlyFromTrustedProxy(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/html", nil)
	req.RemoteAddr = "10.0.0.12:4567"
	req.Header.Set("X-Forwarded-For", "203.0.113.77, 10.0.0.12")

	trustedProxies := NewIPAllowlist([]string{"10.0.0.0/8"})
	got := getClientIP(req, trustedProxies)
	if got != "203.0.113.77" {
		t.Fatalf("client ip = %q, want %q", got, "203.0.113.77")
	}
}

func TestGetClientIPIgnoresForwardedHeadersFromUntrustedSource(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/html", nil)
	req.RemoteAddr = "198.51.100.8:8080"
	req.Header.Set("X-Forwarded-For", "203.0.113.77")
	req.Header.Set("X-Real-IP", "203.0.113.88")

	trustedProxies := NewIPAllowlist([]string{"10.0.0.0/8"})
	got := getClientIP(req, trustedProxies)
	if got != "198.51.100.8" {
		t.Fatalf("client ip = %q, want %q", got, "198.51.100.8")
	}
}

func TestGetClientIPUsesRightmostUntrustedHop(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/html", nil)
	req.RemoteAddr = "10.0.0.12:4567"
	// Leftmost 10.0.0.1 is a spoofed allowlisted hop; the first untrusted hop
	// from the right is the client.
	req.Header.Set("X-Forwarded-For", "10.0.0.1, 203.0.113.77, 10.0.0.5")

	trustedProxies := NewIPAllowlist([]string{"10.0.0.0/8"})
	got := getClientIP(req, trustedProxies)
	if got != "203.0.113.77" {
		t.Fatalf("client ip = %q, want %q", got, "203.0.113.77")
	}
}

func TestGetClientIPFallsBackWhenAllXFFHopsAreTrusted(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/html", nil)
	req.RemoteAddr = "10.0.0.12:4567"
	req.Header.Set("X-Forwarded-For", "10.0.0.5, 10.0.0.12")
	req.Header.Set("X-Real-IP", "10.0.0.9")

	trustedProxies := NewIPAllowlist([]string{"10.0.0.0/8"})
	got := getClientIP(req, trustedProxies)
	if got != "10.0.0.12" {
		t.Fatalf("client ip = %q, want %q", got, "10.0.0.12")
	}
}

func TestGetClientIPStopsAtUnparseableXFFHop(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/html", nil)
	req.RemoteAddr = "10.0.0.12:4567"
	req.Header.Set("X-Forwarded-For", "203.0.113.77, not-an-ip, 10.0.0.12")

	trustedProxies := NewIPAllowlist([]string{"10.0.0.0/8"})
	got := getClientIP(req, trustedProxies)
	if got != "10.0.0.12" {
		t.Fatalf("client ip = %q, want %q", got, "10.0.0.12")
	}
}

func TestWithIPAllowlistAllowsHealthFromAnyPeer(t *testing.T) {
	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)
	allowlist := NewIPAllowlist([]string{"203.0.113.50"})
	trustedProxies := NewIPAllowlist([]string{"10.0.0.0/8"})
	handler := withIPAllowlist(allowlist, trustedProxies, logger, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.RemoteAddr = "198.51.100.8:80"
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestWithRequestLoggingIncludesStatusAndBytes(t *testing.T) {
	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)
	handler := withRequestLogging(logger, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("ok"))
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/markdown", nil)
	handler.ServeHTTP(rec, req)

	logLine := buf.String()
	if logLine == "" {
		t.Fatal("expected a log line")
	}
	if !containsAll(logLine, []string{"POST /markdown", "status=201", "bytes=2", "duration="}) {
		t.Fatalf("unexpected log line: %q", logLine)
	}
}

func containsAll(s string, parts []string) bool {
	for _, part := range parts {
		if !bytes.Contains([]byte(s), []byte(part)) {
			return false
		}
	}
	return true
}
