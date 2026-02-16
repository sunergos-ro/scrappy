package main

import (
	"bytes"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

type Handlers struct {
	cfg    Config
	pool   *BrowserPool
	r2     *R2Client
	logger *log.Logger
}

func NewHandlers(cfg Config, pool *BrowserPool, r2 *R2Client, logger *log.Logger) *Handlers {
	return &Handlers{cfg: cfg, pool: pool, r2: r2, logger: logger}
}

func (h *Handlers) Health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handlers) Stats(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdminAccess(w, r) {
		return
	}
	writeJSON(w, http.StatusOK, h.pool.Stats())
}

func (h *Handlers) Scale(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	if !h.requireAdminAccess(w, r) {
		return
	}

	var req ScaleRequest
	if !decodeJSONBody(w, r.Body, &req, h.cfg.MaxRequestBodyBytes) {
		return
	}

	stats, err := h.pool.Scale(req.Size)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, stats)
}

func (h *Handlers) Screenshot(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	if h.r2 == nil {
		writeError(w, http.StatusServiceUnavailable, "r2 not configured")
		return
	}

	var req ScreenshotRequest
	if !decodeJSONBody(w, r.Body, &req, h.cfg.MaxRequestBodyBytes) {
		return
	}

	normalizedURL, ok := validateTargetURL(w, h.cfg, req.URL)
	if !ok {
		return
	}
	req.URL = normalizedURL

	options := resolveScreenshotOptions(h.cfg, req)
	start := time.Now()

	result, err := h.pool.Screenshot(r.Context(), options)
	if err != nil {
		h.logf("screenshot failed for %s: %v", options.URL, err)
		writeError(w, http.StatusBadGateway, "failed to capture screenshot")
		return
	}

	key := buildObjectKey("screenshots", result.Format)
	publicURL, err := h.r2.Upload(r.Context(), key, result.Bytes, result.ContentType)
	if err != nil {
		h.logf("r2 upload failed for %s key=%s: %v", options.URL, key, err)
		writeError(w, http.StatusBadGateway, "failed to upload screenshot")
		return
	}

	resp := ScreenshotResponse{
		URL:       options.URL,
		Bucket:    h.r2.Bucket,
		Key:       key,
		PublicURL: publicURL,
		Bytes:     len(result.Bytes),
		Format:    result.Format,
		Width:     result.ViewportWidth,
		Height:    result.ViewportHeight,
		TookMS:    time.Since(start).Milliseconds(),
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handlers) Render(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	var req RenderRequest
	if !decodeJSONBody(w, r.Body, &req, h.cfg.MaxRequestBodyBytes) {
		return
	}

	normalizedURL, ok := validateTargetURL(w, h.cfg, req.URL)
	if !ok {
		return
	}
	req.URL = normalizedURL

	options := resolveRenderOptions(h.cfg, req)
	start := time.Now()

	html, err := h.pool.Render(r.Context(), options)
	if err != nil {
		h.logf("render failed for %s: %v", options.URL, err)
		writeError(w, http.StatusBadGateway, "failed to render page")
		return
	}

	resp := RenderResponse{
		URL:    options.URL,
		HTML:   html,
		TookMS: time.Since(start).Milliseconds(),
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handlers) Markdown(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	var req MarkdownRequest
	if !decodeJSONBody(w, r.Body, &req, h.cfg.MaxRequestBodyBytes) {
		return
	}

	normalizedURL, ok := validateTargetURL(w, h.cfg, req.URL)
	if !ok {
		return
	}
	req.URL = normalizedURL

	options := resolveMarkdownOptions(h.cfg, req)
	start := time.Now()

	markdown, err := h.pool.Markdown(r.Context(), options)
	if err != nil {
		h.logf("markdown failed for %s: %v", options.URL, err)
		writeError(w, http.StatusBadGateway, "failed to extract markdown")
		return
	}

	resp := MarkdownResponse{
		URL:      options.URL,
		Markdown: markdown,
		TookMS:   time.Since(start).Milliseconds(),
	}
	writeJSON(w, http.StatusOK, resp)
}

func decodeJSON(body io.ReadCloser, target any, maxBodyBytes int64) error {
	defer body.Close()

	if maxBodyBytes <= 0 {
		maxBodyBytes = 1024 * 1024
	}

	limited := &io.LimitedReader{R: body, N: maxBodyBytes + 1}
	payload, err := io.ReadAll(limited)
	if err != nil {
		return errors.New("failed to read body")
	}
	if int64(len(payload)) > maxBodyBytes {
		return errors.New("body too large")
	}
	if len(payload) == 0 {
		return errors.New("empty body")
	}

	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return errors.New("invalid json body")
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return errors.New("invalid json body")
	}
	return nil
}

func requireMethod(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method != method {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return false
	}
	return true
}

func decodeJSONBody(w http.ResponseWriter, body io.ReadCloser, target any, maxBodyBytes int64) bool {
	if err := decodeJSON(body, target, maxBodyBytes); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return false
	}
	return true
}

func requireNonEmptyURL(w http.ResponseWriter, value string) bool {
	if strings.TrimSpace(value) == "" {
		writeError(w, http.StatusBadRequest, "url is required")
		return false
	}
	return true
}

func validateTargetURL(w http.ResponseWriter, cfg Config, value string) (string, bool) {
	normalizedURL, err := validateAndNormalizeTargetURL(cfg, value)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return "", false
	}
	return normalizedURL, true
}

func (h *Handlers) requireAdminAccess(w http.ResponseWriter, r *http.Request) bool {
	if strings.TrimSpace(h.cfg.AdminToken) == "" {
		return true
	}
	if r == nil {
		writeError(w, http.StatusUnauthorized, "admin token required")
		return false
	}

	token := strings.TrimSpace(r.Header.Get("X-Admin-Token"))
	if token == "" {
		authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
		if len(authHeader) > 7 && strings.EqualFold(authHeader[:7], "Bearer ") {
			token = strings.TrimSpace(authHeader[7:])
		}
	}

	if subtle.ConstantTimeCompare([]byte(token), []byte(h.cfg.AdminToken)) != 1 {
		writeError(w, http.StatusUnauthorized, "admin token required")
		return false
	}

	return true
}

func (h *Handlers) logf(format string, args ...any) {
	if h.logger != nil {
		h.logger.Printf(format, args...)
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
