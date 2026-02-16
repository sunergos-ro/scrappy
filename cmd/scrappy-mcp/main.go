package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"scrappy/pkg/client"
)

const (
	defaultBaseURL       = "http://localhost:3000"
	defaultHTTPTimeoutMS = 80000
)

func main() {
	os.Exit(run(context.Background(), os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(ctx context.Context, args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("scrappy-mcp", flag.ContinueOnError)
	flags.SetOutput(stderr)

	baseURL := flags.String("base-url", firstNonEmpty(strings.TrimSpace(os.Getenv("SCRAPPY_BASE_URL")), defaultBaseURL), "Scrappy server base URL")
	adminToken := flags.String("admin-token", strings.TrimSpace(os.Getenv("SCRAPPY_ADMIN_TOKEN")), "Admin token for protected endpoints")
	httpTimeoutMS := flags.Int("http-timeout-ms", envInt("SCRAPPY_HTTP_TIMEOUT_MS", defaultHTTPTimeoutMS), "HTTP timeout in milliseconds")
	flags.Usage = func() {
		printUsage(stderr)
	}

	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if flags.NArg() != 0 {
		_, _ = fmt.Fprintln(stderr, "error: unexpected positional arguments")
		printUsage(stderr)
		return 2
	}

	httpClient := &http.Client{
		Timeout: durationFromMS(*httpTimeoutMS, defaultHTTPTimeoutMS),
	}

	c, err := client.New(*baseURL, client.WithHTTPClient(httpClient), client.WithAdminToken(*adminToken))
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "error: %v\n", err)
		return 2
	}

	srv := newMCPServer(c)
	reader := bufio.NewReader(stdin)
	writer := bufio.NewWriter(stdout)

	for {
		payload, err := readRPCMessage(reader)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return 0
			}
			_ = writeRPCMessage(writer, rpcResponse{
				JSONRPC: jsonRPCVersion,
				ID:      json.RawMessage("null"),
				Error: &rpcError{
					Code:    -32700,
					Message: "parse error",
					Data:    err.Error(),
				},
			})
			return 1
		}

		var req rpcRequest
		if err := json.Unmarshal(payload, &req); err != nil {
			if writeErr := writeRPCMessage(writer, rpcResponse{
				JSONRPC: jsonRPCVersion,
				ID:      json.RawMessage("null"),
				Error: &rpcError{
					Code:    -32700,
					Message: "parse error",
					Data:    err.Error(),
				},
			}); writeErr != nil {
				_, _ = fmt.Fprintf(stderr, "error: failed to write parse error response: %v\n", writeErr)
				return 1
			}
			continue
		}

		if strings.TrimSpace(req.Method) == "" {
			continue
		}

		resp := srv.handleRequest(ctx, req)
		if resp == nil {
			continue
		}
		if err := writeRPCMessage(writer, resp); err != nil {
			_, _ = fmt.Fprintf(stderr, "error: failed to write response: %v\n", err)
			return 1
		}
	}
}

func envInt(name string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func durationFromMS(rawMS int, fallbackMS int) time.Duration {
	ms := rawMS
	if ms <= 0 {
		ms = fallbackMS
	}
	return time.Duration(ms) * time.Millisecond
}

func printUsage(w io.Writer) {
	_, _ = fmt.Fprintln(w, "Usage: scrappy-mcp [options]")
	_, _ = fmt.Fprintln(w, "")
	_, _ = fmt.Fprintln(w, "Options:")
	_, _ = fmt.Fprintln(w, "  --base-url         Scrappy server base URL (env: SCRAPPY_BASE_URL)")
	_, _ = fmt.Fprintln(w, "  --admin-token      Admin token (env: SCRAPPY_ADMIN_TOKEN)")
	_, _ = fmt.Fprintln(w, "  --http-timeout-ms  HTTP timeout in milliseconds (env: SCRAPPY_HTTP_TIMEOUT_MS)")
}
