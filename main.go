package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Printf("warning: .env file not found or could not be loaded: %v", err)
	}

	if err := sentry.Init(sentry.ClientOptions{
		Dsn: os.Getenv("SENTRY_DSN"),
	}); err != nil {
		log.Fatalf("sentry.Init: %s", err)
	}
	defer sentry.Flush(2 * time.Second)

	cfg := LoadConfig()

	logger := log.New(os.Stdout, "", log.LstdFlags)

	pool := NewBrowserPool(cfg, logger)
	if cfg.PoolEnabled {
		pool.Preload()
	}

	r2, err := NewR2Client(cfg)
	if err != nil {
		logger.Printf("R2 disabled: %v", err)
	}

	allowlist := NewIPAllowlist(cfg.AllowedIPs)
	trustedProxies := NewIPAllowlist(cfg.TrustedProxyCIDRs)
	logger.Printf("IP allowlist enabled: %v", cfg.AllowedIPs)
	logger.Printf("trusted proxy CIDRs: %v", cfg.TrustedProxyCIDRs)

	handlers := NewHandlers(cfg, pool, r2, logger)
	mux := http.NewServeMux()
	registerRoutes(mux, handlers)

	// Apply middleware: logging -> IP allowlist -> handler
	handler := withRequestLogging(logger, withIPAllowlist(allowlist, trustedProxies, logger, mux))

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      75 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		logger.Printf("scrappy listening on %s", cfg.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("server error: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool.Shutdown()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Printf("shutdown error: %v", err)
	}
}

func withRequestLogging(logger *log.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		logger.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}

func withIPAllowlist(allowlist *IPAllowlist, trustedProxies *IPAllowlist, logger *log.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Allow health checks to bypass IP allowlist (needed for kamal-proxy)
		if r.URL.Path == "/health" {
			next.ServeHTTP(w, r)
			return
		}

		clientIP := getClientIP(r, trustedProxies)
		if !allowlist.IsAllowed(clientIP) {
			logger.Printf("blocked request from %s to %s", clientIP, r.URL.Path)
			writeError(w, http.StatusForbidden, "forbidden")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func getRemoteIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		if ip := parseClientIPCandidate(r.RemoteAddr); ip != "" {
			return ip
		}
		return strings.TrimSpace(r.RemoteAddr)
	}
	if ip := parseClientIPCandidate(host); ip != "" {
		return ip
	}
	return strings.TrimSpace(host)
}

func getClientIP(r *http.Request, trustedProxies *IPAllowlist) string {
	remoteIP := getRemoteIP(r)
	if trustedProxies == nil || !trustedProxies.IsAllowed(remoteIP) {
		return remoteIP
	}

	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		if len(parts) > 0 {
			if ip := parseClientIPCandidate(parts[0]); ip != "" {
				return ip
			}
		}
	}

	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		if ip := parseClientIPCandidate(xri); ip != "" {
			return ip
		}
	}

	return remoteIP
}

func parseClientIPCandidate(value string) string {
	candidate := strings.TrimSpace(value)
	if candidate == "" {
		return ""
	}

	if ip := net.ParseIP(candidate); ip != nil {
		return ip.String()
	}

	host, _, err := net.SplitHostPort(candidate)
	if err != nil {
		return ""
	}
	if ip := net.ParseIP(strings.TrimSpace(host)); ip != nil {
		return ip.String()
	}
	return ""
}

func registerRoutes(mux *http.ServeMux, handlers *Handlers) {
	mux.HandleFunc("/health", handlers.Health) // Health check bypasses IP allowlist
	mux.HandleFunc("/stats", handlers.Stats)
	mux.HandleFunc("/pool/scale", handlers.Scale)
	mux.HandleFunc("/screenshot", handlers.Screenshot)
	mux.HandleFunc("/html", handlers.Render)
	mux.HandleFunc("/markdown", handlers.Markdown)
}
