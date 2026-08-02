package main

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestInitPostHogDisabledWithoutAPIKey(t *testing.T) {
	t.Setenv("POSTHOG_PROJECT_API_KEY", "")
	t.Setenv("SENTRY_DSN", "")

	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)
	client := initPostHog(logger)
	if client != nil {
		t.Fatal("expected nil client when POSTHOG_PROJECT_API_KEY is unset")
	}
}

func TestInitPostHogWarnsOnLegacySentryDSN(t *testing.T) {
	t.Setenv("POSTHOG_PROJECT_API_KEY", "")
	t.Setenv("SENTRY_DSN", "https://example@sentry.example/1")

	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)
	_ = initPostHog(logger)

	got := buf.String()
	if !strings.Contains(got, "SENTRY_DSN") || !strings.Contains(got, "POSTHOG_PROJECT_API_KEY") {
		t.Fatalf("expected Sentry deprecation warning, got %q", got)
	}
}

func TestWithPanicRecoveryDoesNotCrashWithoutPostHog(t *testing.T) {
	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)
	handler := withPanicRecovery(nil, logger, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/html", nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if msg := readErrorMessage(t, rec); msg != "internal server error" {
		t.Fatalf("error message = %q", msg)
	}
	if !strings.Contains(buf.String(), "panic recovered") {
		t.Fatalf("expected panic log, got %q", buf.String())
	}
}

func TestClosePostHogNilSafe(t *testing.T) {
	closePostHog(nil)
}
