package main

import (
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

type requestFlags struct {
	url            string
	userAgent      string
	waitMS         int
	timeoutMS      int
	viewportWidth  int
	viewportHeight int
}

type markdownRequestFlags struct {
	requestFlags
	primeLazyContent bool
}

func main() {
	os.Exit(run(context.Background(), os.Args[1:], os.Stdout, os.Stderr))
}

func run(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	global := flag.NewFlagSet("scrappy", flag.ContinueOnError)
	global.SetOutput(stderr)

	baseURL := global.String("base-url", firstNonEmpty(strings.TrimSpace(os.Getenv("SCRAPPY_BASE_URL")), defaultBaseURL), "Scrappy server base URL")
	adminToken := global.String("admin-token", strings.TrimSpace(os.Getenv("SCRAPPY_ADMIN_TOKEN")), "Admin token for protected endpoints")
	httpTimeoutMS := global.Int("http-timeout-ms", envInt("SCRAPPY_HTTP_TIMEOUT_MS", defaultHTTPTimeoutMS), "HTTP timeout in milliseconds")
	pretty := global.Bool("pretty", false, "Pretty-print JSON output")
	global.Usage = func() {
		printUsage(stderr)
	}

	if err := global.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}

	remaining := global.Args()
	if len(remaining) == 0 {
		printUsage(stderr)
		return 2
	}

	command := remaining[0]
	if command == "help" {
		printUsage(stdout)
		return 0
	}

	httpClient := &http.Client{
		Timeout: durationFromMS(*httpTimeoutMS, defaultHTTPTimeoutMS),
	}

	c, err := client.New(*baseURL, client.WithHTTPClient(httpClient), client.WithAdminToken(*adminToken))
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "error: %v\n", err)
		return 2
	}

	switch command {
	case "html":
		return runHTML(ctx, c, remaining[1:], *pretty, stdout, stderr)
	case "markdown":
		return runMarkdown(ctx, c, remaining[1:], *pretty, stdout, stderr)
	case "screenshot":
		return runScreenshot(ctx, c, remaining[1:], *pretty, stdout, stderr)
	case "stats":
		return runStats(ctx, c, remaining[1:], *pretty, stdout, stderr)
	case "scale":
		return runScale(ctx, c, remaining[1:], *pretty, stdout, stderr)
	default:
		_, _ = fmt.Fprintf(stderr, "error: unknown command %q\n", command)
		printUsage(stderr)
		return 2
	}
}

func runHTML(ctx context.Context, c *client.Client, args []string, pretty bool, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("html", flag.ContinueOnError)
	fs.SetOutput(stderr)

	opts := requestFlags{}
	bindRequestFlags(fs, &opts)
	fs.Usage = func() {
		_, _ = fmt.Fprintln(stderr, "Usage: scrappy [global options] html --url <url> [options]")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if fs.NArg() != 0 {
		_, _ = fmt.Fprintln(stderr, "error: unexpected positional arguments")
		fs.Usage()
		return 2
	}

	req, err := buildRenderRequest(opts)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "error: %v\n", err)
		return 2
	}

	resp, err := c.HTML(ctx, req)
	if err != nil {
		return printCommandError(stderr, err)
	}

	return printJSON(stdout, stderr, resp, pretty)
}

func runMarkdown(ctx context.Context, c *client.Client, args []string, pretty bool, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("markdown", flag.ContinueOnError)
	fs.SetOutput(stderr)

	opts := markdownRequestFlags{}
	bindRequestFlags(fs, &opts.requestFlags)
	fs.BoolVar(&opts.primeLazyContent, "prime-lazy-content", false, "Scroll through the page before markdown extraction to trigger lazy-loaded content")
	fs.Usage = func() {
		_, _ = fmt.Fprintln(stderr, "Usage: scrappy [global options] markdown --url <url> [options]")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if fs.NArg() != 0 {
		_, _ = fmt.Fprintln(stderr, "error: unexpected positional arguments")
		fs.Usage()
		return 2
	}

	req, err := buildMarkdownRequest(opts)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "error: %v\n", err)
		return 2
	}

	resp, err := c.Markdown(ctx, req)
	if err != nil {
		return printCommandError(stderr, err)
	}

	return printJSON(stdout, stderr, resp, pretty)
}

func runScreenshot(ctx context.Context, c *client.Client, args []string, pretty bool, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("screenshot", flag.ContinueOnError)
	fs.SetOutput(stderr)

	opts := requestFlags{}
	bindRequestFlags(fs, &opts)

	format := fs.String("format", "", "Screenshot format (jpeg, png, webp)")
	quality := fs.Int("quality", 0, "JPEG/WebP quality")
	deviceScaleFactor := fs.Float64("device-scale-factor", 0, "Device scale factor (DPR) for screenshot rendering")
	fs.Usage = func() {
		_, _ = fmt.Fprintln(stderr, "Usage: scrappy [global options] screenshot --url <url> [options]")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if fs.NArg() != 0 {
		_, _ = fmt.Fprintln(stderr, "error: unexpected positional arguments")
		fs.Usage()
		return 2
	}

	req, err := buildScreenshotRequest(opts, *format, *quality, *deviceScaleFactor)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "error: %v\n", err)
		return 2
	}

	resp, err := c.Screenshot(ctx, req)
	if err != nil {
		return printCommandError(stderr, err)
	}

	return printJSON(stdout, stderr, resp, pretty)
}

func runStats(ctx context.Context, c *client.Client, args []string, pretty bool, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("stats", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		_, _ = fmt.Fprintln(stderr, "Usage: scrappy [global options] stats")
	}

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if fs.NArg() != 0 {
		_, _ = fmt.Fprintln(stderr, "error: unexpected positional arguments")
		fs.Usage()
		return 2
	}

	resp, err := c.Stats(ctx)
	if err != nil {
		return printCommandError(stderr, err)
	}

	return printJSON(stdout, stderr, resp, pretty)
}

func runScale(ctx context.Context, c *client.Client, args []string, pretty bool, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("scale", flag.ContinueOnError)
	fs.SetOutput(stderr)

	size := fs.Int("size", -1, "Desired browser pool size")
	fs.Usage = func() {
		_, _ = fmt.Fprintln(stderr, "Usage: scrappy [global options] scale --size <n>")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if fs.NArg() != 0 {
		_, _ = fmt.Fprintln(stderr, "error: unexpected positional arguments")
		fs.Usage()
		return 2
	}
	if *size < 0 {
		_, _ = fmt.Fprintln(stderr, "error: --size is required and must be >= 0")
		fs.Usage()
		return 2
	}

	resp, err := c.Scale(ctx, client.ScaleRequest{Size: *size})
	if err != nil {
		return printCommandError(stderr, err)
	}

	return printJSON(stdout, stderr, resp, pretty)
}

func bindRequestFlags(fs *flag.FlagSet, opts *requestFlags) {
	fs.StringVar(&opts.url, "url", "", "Target URL")
	fs.StringVar(&opts.userAgent, "user-agent", "", "User-Agent override")
	fs.IntVar(&opts.waitMS, "wait-ms", 0, "Additional wait time before extraction")
	fs.IntVar(&opts.timeoutMS, "timeout-ms", 0, "Per-request timeout")
	fs.IntVar(&opts.viewportWidth, "viewport-width", 0, "Viewport width")
	fs.IntVar(&opts.viewportHeight, "viewport-height", 0, "Viewport height")
}

func buildRenderRequest(opts requestFlags) (client.RenderRequest, error) {
	urlValue, err := requireURL(opts.url)
	if err != nil {
		return client.RenderRequest{}, err
	}

	return client.RenderRequest{
		URL:       urlValue,
		Viewport:  makeViewport(opts.viewportWidth, opts.viewportHeight),
		UserAgent: strings.TrimSpace(opts.userAgent),
		WaitMS:    opts.waitMS,
		TimeoutMS: opts.timeoutMS,
	}, nil
}

func buildMarkdownRequest(opts markdownRequestFlags) (client.MarkdownRequest, error) {
	urlValue, err := requireURL(opts.requestFlags.url)
	if err != nil {
		return client.MarkdownRequest{}, err
	}

	return client.MarkdownRequest{
		URL:              urlValue,
		Viewport:         makeViewport(opts.requestFlags.viewportWidth, opts.requestFlags.viewportHeight),
		UserAgent:        strings.TrimSpace(opts.requestFlags.userAgent),
		WaitMS:           opts.requestFlags.waitMS,
		TimeoutMS:        opts.requestFlags.timeoutMS,
		PrimeLazyContent: opts.primeLazyContent,
	}, nil
}

func buildScreenshotRequest(opts requestFlags, format string, quality int, deviceScaleFactor float64) (client.ScreenshotRequest, error) {
	urlValue, err := requireURL(opts.url)
	if err != nil {
		return client.ScreenshotRequest{}, err
	}

	return client.ScreenshotRequest{
		URL:               urlValue,
		Viewport:          makeViewport(opts.viewportWidth, opts.viewportHeight),
		UserAgent:         strings.TrimSpace(opts.userAgent),
		Format:            strings.TrimSpace(format),
		Quality:           quality,
		DeviceScaleFactor: deviceScaleFactor,
		WaitMS:            opts.waitMS,
		TimeoutMS:         opts.timeoutMS,
	}, nil
}

func requireURL(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", errors.New("--url is required")
	}
	return trimmed, nil
}

func makeViewport(width int, height int) *client.Viewport {
	if width <= 0 && height <= 0 {
		return nil
	}
	return &client.Viewport{
		Width:  width,
		Height: height,
	}
}

func printCommandError(stderr io.Writer, err error) int {
	var apiErr *client.APIError
	if errors.As(err, &apiErr) {
		_, _ = fmt.Fprintf(stderr, "error: %s (status %d)\n", apiErr.Message, apiErr.StatusCode)
		return 1
	}
	_, _ = fmt.Fprintf(stderr, "error: %v\n", err)
	return 1
}

func printJSON(stdout io.Writer, stderr io.Writer, value any, pretty bool) int {
	encoder := json.NewEncoder(stdout)
	if pretty {
		encoder.SetIndent("", "  ")
	}
	if err := encoder.Encode(value); err != nil {
		_, _ = fmt.Fprintf(stderr, "error: failed to encode output: %v\n", err)
		return 1
	}
	return 0
}

func durationFromMS(rawMS int, fallbackMS int) time.Duration {
	ms := rawMS
	if ms <= 0 {
		ms = fallbackMS
	}
	return time.Duration(ms) * time.Millisecond
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

func printUsage(w io.Writer) {
	_, _ = fmt.Fprintln(w, "Usage: scrappy [global options] <command> [command options]")
	_, _ = fmt.Fprintln(w, "")
	_, _ = fmt.Fprintln(w, "Commands:")
	_, _ = fmt.Fprintln(w, "  html        Render page HTML")
	_, _ = fmt.Fprintln(w, "  markdown    Extract markdown-like content")
	_, _ = fmt.Fprintln(w, "  screenshot  Capture screenshot")
	_, _ = fmt.Fprintln(w, "  stats       Show pool stats")
	_, _ = fmt.Fprintln(w, "  scale       Resize pool")
	_, _ = fmt.Fprintln(w, "")
	_, _ = fmt.Fprintln(w, "Global options:")
	_, _ = fmt.Fprintln(w, "  --base-url         Scrappy server base URL (env: SCRAPPY_BASE_URL)")
	_, _ = fmt.Fprintln(w, "  --admin-token      Admin token (env: SCRAPPY_ADMIN_TOKEN)")
	_, _ = fmt.Fprintln(w, "  --http-timeout-ms  HTTP timeout in milliseconds (env: SCRAPPY_HTTP_TIMEOUT_MS)")
	_, _ = fmt.Fprintln(w, "  --pretty           Pretty-print JSON output")
}
