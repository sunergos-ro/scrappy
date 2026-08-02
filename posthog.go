package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime/debug"
	"strings"
	"time"

	"github.com/posthog/posthog-go"
)

const posthogDistinctID = "scrappy"

// initPostHog returns a PostHog client when POSTHOG_PROJECT_API_KEY is set.
// When unset, observability is disabled and the service runs normally (OSS default).
// POSTHOG_HOST is optional; empty uses the PostHog Go SDK default (PostHog Cloud US).
// Self-hosted instances should set POSTHOG_HOST to their ingestion URL.
func initPostHog(logger *log.Logger) posthog.Client {
	if strings.TrimSpace(os.Getenv("SENTRY_DSN")) != "" {
		logger.Printf("warning: SENTRY_DSN is set but Sentry support was removed in v0.5.0; set POSTHOG_PROJECT_API_KEY (and optionally POSTHOG_HOST) instead, or remove SENTRY_DSN")
	}

	apiKey := strings.TrimSpace(os.Getenv("POSTHOG_PROJECT_API_KEY"))
	if apiKey == "" {
		return nil
	}

	cfg := posthog.Config{
		ShutdownTimeout: 2 * time.Second,
	}
	if host := strings.TrimSpace(os.Getenv("POSTHOG_HOST")); host != "" {
		cfg.Endpoint = host
	}

	client, err := posthog.NewWithConfig(apiKey, cfg)
	if err != nil {
		logger.Printf("warning: posthog init failed (continuing without error tracking): %v", err)
		return nil
	}

	endpoint := cfg.Endpoint
	if endpoint == "" {
		endpoint = posthog.DefaultEndpoint
	}
	logger.Printf("PostHog error tracking enabled (endpoint=%s)", endpoint)
	return client
}

func closePostHog(client posthog.Client) {
	if client == nil {
		return
	}
	_ = client.Close()
}

func capturePanic(client posthog.Client, r any, path string) {
	if client == nil {
		return
	}
	description := fmt.Sprintf("%v", r)
	exc := posthog.NewDefaultException(time.Now(), posthogDistinctID, "panic", description)
	exc.Properties = posthog.NewProperties().
		Set("path", path).
		Set("stack", string(debug.Stack()))
	_ = client.Enqueue(exc)
}

// withPanicRecovery recovers panics, reports them to PostHog when configured,
// and returns HTTP 500. When PostHog is unset this still prevents process crashes.
func withPanicRecovery(client posthog.Client, logger *log.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				logger.Printf("panic recovered path=%s err=%v", r.URL.Path, recovered)
				capturePanic(client, recovered, r.URL.Path)
				writeError(w, http.StatusInternalServerError, "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}
