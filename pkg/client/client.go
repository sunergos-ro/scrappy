package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	defaultHTTPTimeout  = 80 * time.Second
	maxResponseBodySize = 64 << 20 // 64 MiB
)

type Client struct {
	baseURL    string
	httpClient *http.Client
	adminToken string
}

type Option func(*Client)

type APIError struct {
	StatusCode int
	Message    string
	Body       string
}

func (e *APIError) Error() string {
	if e == nil {
		return ""
	}
	if e.Message != "" {
		return fmt.Sprintf("api error %d: %s", e.StatusCode, e.Message)
	}
	return fmt.Sprintf("api error %d", e.StatusCode)
}

type Viewport struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

type RenderRequest struct {
	URL       string    `json:"url"`
	Viewport  *Viewport `json:"viewport,omitempty"`
	UserAgent string    `json:"user_agent,omitempty"`
	WaitMS    int       `json:"wait_ms,omitempty"`
	TimeoutMS int       `json:"timeout_ms,omitempty"`
}

type RenderResponse struct {
	URL    string `json:"url"`
	HTML   string `json:"html"`
	TookMS int64  `json:"took_ms"`
}

type MarkdownRequest struct {
	URL       string    `json:"url"`
	Viewport  *Viewport `json:"viewport,omitempty"`
	UserAgent string    `json:"user_agent,omitempty"`
	WaitMS    int       `json:"wait_ms,omitempty"`
	TimeoutMS int       `json:"timeout_ms,omitempty"`
}

type MarkdownResponse struct {
	URL      string `json:"url"`
	Markdown string `json:"markdown"`
	TookMS   int64  `json:"took_ms"`
}

type ScreenshotRequest struct {
	URL               string    `json:"url"`
	Viewport          *Viewport `json:"viewport,omitempty"`
	UserAgent         string    `json:"user_agent,omitempty"`
	Format            string    `json:"format,omitempty"`
	Quality           int       `json:"quality,omitempty"`
	DeviceScaleFactor float64   `json:"device_scale_factor,omitempty"`
	WaitMS            int       `json:"wait_ms,omitempty"`
	TimeoutMS         int       `json:"timeout_ms,omitempty"`
}

type ScreenshotResponse struct {
	URL       string `json:"url"`
	Bucket    string `json:"bucket"`
	Key       string `json:"key"`
	PublicURL string `json:"public_url"`
	Bytes     int    `json:"bytes"`
	Format    string `json:"format"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	TookMS    int64  `json:"took_ms"`
}

type ScaleRequest struct {
	Size int `json:"size"`
}

func New(baseURL string, options ...Option) (*Client, error) {
	trimmed := strings.TrimSpace(baseURL)
	if trimmed == "" {
		return nil, errors.New("base URL is required")
	}

	parsed, err := url.Parse(trimmed)
	if err != nil || parsed == nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, errors.New("base URL must be an absolute URL")
	}

	c := &Client{
		baseURL: strings.TrimRight(parsed.String(), "/"),
		httpClient: &http.Client{
			Timeout: defaultHTTPTimeout,
		},
	}

	for _, option := range options {
		if option != nil {
			option(c)
		}
	}

	if c.httpClient == nil {
		c.httpClient = &http.Client{
			Timeout: defaultHTTPTimeout,
		}
	}

	return c, nil
}

func WithHTTPClient(httpClient *http.Client) Option {
	return func(c *Client) {
		c.httpClient = httpClient
	}
}

func WithAdminToken(token string) Option {
	return func(c *Client) {
		c.adminToken = strings.TrimSpace(token)
	}
}

func (c *Client) HTML(ctx context.Context, req RenderRequest) (RenderResponse, error) {
	var out RenderResponse
	if err := c.doJSON(ctx, http.MethodPost, "/html", req, &out); err != nil {
		return RenderResponse{}, err
	}
	return out, nil
}

func (c *Client) Markdown(ctx context.Context, req MarkdownRequest) (MarkdownResponse, error) {
	var out MarkdownResponse
	if err := c.doJSON(ctx, http.MethodPost, "/markdown", req, &out); err != nil {
		return MarkdownResponse{}, err
	}
	return out, nil
}

func (c *Client) Screenshot(ctx context.Context, req ScreenshotRequest) (ScreenshotResponse, error) {
	var out ScreenshotResponse
	if err := c.doJSON(ctx, http.MethodPost, "/screenshot", req, &out); err != nil {
		return ScreenshotResponse{}, err
	}
	return out, nil
}

func (c *Client) Stats(ctx context.Context) (map[string]any, error) {
	var out map[string]any
	if err := c.doJSON(ctx, http.MethodGet, "/stats", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) Scale(ctx context.Context, req ScaleRequest) (map[string]any, error) {
	var out map[string]any
	if err := c.doJSON(ctx, http.MethodPost, "/pool/scale", req, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) doJSON(ctx context.Context, method string, path string, payload any, target any) error {
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("encode request: %w", err)
		}
		body = bytes.NewReader(encoded)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.adminToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.adminToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBodySize))
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return decodeAPIError(resp.StatusCode, respBody)
	}

	if target == nil || len(respBody) == 0 {
		return nil
	}

	if err := json.Unmarshal(respBody, target); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	return nil
}

func decodeAPIError(statusCode int, body []byte) error {
	message := strings.TrimSpace(http.StatusText(statusCode))
	bodyText := strings.TrimSpace(string(body))

	var payload struct {
		Error string `json:"error"`
	}
	if len(body) > 0 && json.Unmarshal(body, &payload) == nil && strings.TrimSpace(payload.Error) != "" {
		message = strings.TrimSpace(payload.Error)
	} else if bodyText != "" {
		message = bodyText
	}

	return &APIError{
		StatusCode: statusCode,
		Message:    message,
		Body:       bodyText,
	}
}
