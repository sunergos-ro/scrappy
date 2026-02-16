package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func readErrorMessage(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var payload map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}
	return payload["error"]
}

func TestRequireMethod(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		wantOK     bool
		wantStatus int
		wantError  string
	}{
		{
			name:       "accepts matching method",
			method:     http.MethodPost,
			wantOK:     true,
			wantStatus: http.StatusOK,
		},
		{
			name:       "rejects mismatched method",
			method:     http.MethodGet,
			wantOK:     false,
			wantStatus: http.StatusMethodNotAllowed,
			wantError:  "method not allowed",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(tc.method, "/markdown", nil)

			got := requireMethod(rec, req, http.MethodPost)
			if got != tc.wantOK {
				t.Fatalf("requireMethod() = %v, want %v", got, tc.wantOK)
			}

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tc.wantStatus)
			}

			if tc.wantError != "" {
				if msg := readErrorMessage(t, rec); msg != tc.wantError {
					t.Fatalf("error message = %q, want %q", msg, tc.wantError)
				}
			}
		})
	}
}

func TestRequireNonEmptyURL(t *testing.T) {
	tests := []struct {
		name       string
		value      string
		wantOK     bool
		wantStatus int
		wantError  string
	}{
		{
			name:       "accepts non-empty value",
			value:      "https://example.com",
			wantOK:     true,
			wantStatus: http.StatusOK,
		},
		{
			name:       "rejects blank value",
			value:      " \t\n ",
			wantOK:     false,
			wantStatus: http.StatusBadRequest,
			wantError:  "url is required",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			got := requireNonEmptyURL(rec, tc.value)
			if got != tc.wantOK {
				t.Fatalf("requireNonEmptyURL() = %v, want %v", got, tc.wantOK)
			}
			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
			if tc.wantError != "" {
				if msg := readErrorMessage(t, rec); msg != tc.wantError {
					t.Fatalf("error message = %q, want %q", msg, tc.wantError)
				}
			}
		})
	}
}

func TestDecodeJSONBody(t *testing.T) {
	type request struct {
		URL string `json:"url"`
	}

	tests := []struct {
		name       string
		body       io.ReadCloser
		maxBody    int64
		wantOK     bool
		wantStatus int
		wantError  string
		wantURL    string
	}{
		{
			name:       "decodes valid json",
			body:       io.NopCloser(strings.NewReader(`{"url":"https://example.com"}`)),
			maxBody:    1024,
			wantOK:     true,
			wantStatus: http.StatusOK,
			wantURL:    "https://example.com",
		},
		{
			name:       "rejects invalid json",
			body:       io.NopCloser(strings.NewReader(`{`)),
			maxBody:    1024,
			wantOK:     false,
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid json body",
		},
		{
			name:       "rejects unknown fields",
			body:       io.NopCloser(strings.NewReader(`{"url":"https://example.com","extra":"x"}`)),
			maxBody:    1024,
			wantOK:     false,
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid json body",
		},
		{
			name:       "rejects large body",
			body:       io.NopCloser(strings.NewReader(`{"url":"https://example.com"}`)),
			maxBody:    10,
			wantOK:     false,
			wantStatus: http.StatusBadRequest,
			wantError:  "body too large",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			var req request

			got := decodeJSONBody(rec, tc.body, &req, tc.maxBody)
			if got != tc.wantOK {
				t.Fatalf("decodeJSONBody() = %v, want %v", got, tc.wantOK)
			}
			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tc.wantStatus)
			}

			if tc.wantError != "" {
				if msg := readErrorMessage(t, rec); msg != tc.wantError {
					t.Fatalf("error message = %q, want %q", msg, tc.wantError)
				}
			}

			if tc.wantURL != "" && req.URL != tc.wantURL {
				t.Fatalf("decoded url = %q, want %q", req.URL, tc.wantURL)
			}
		})
	}
}

func TestRequireAdminAccess(t *testing.T) {
	h := &Handlers{
		cfg:    Config{AdminToken: "secret-token"},
		logger: log.New(os.Stdout, "", 0),
	}

	tests := []struct {
		name           string
		authHeader     string
		xAdminToken    string
		wantAuthorized bool
		wantStatus     int
	}{
		{
			name:           "rejects missing token",
			wantAuthorized: false,
			wantStatus:     http.StatusUnauthorized,
		},
		{
			name:           "accepts bearer token",
			authHeader:     "Bearer secret-token",
			wantAuthorized: true,
			wantStatus:     http.StatusOK,
		},
		{
			name:           "accepts x admin token",
			xAdminToken:    "secret-token",
			wantAuthorized: true,
			wantStatus:     http.StatusOK,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/stats", nil)
			if tc.authHeader != "" {
				req.Header.Set("Authorization", tc.authHeader)
			}
			if tc.xAdminToken != "" {
				req.Header.Set("X-Admin-Token", tc.xAdminToken)
			}

			ok := h.requireAdminAccess(rec, req)
			if ok != tc.wantAuthorized {
				t.Fatalf("requireAdminAccess() = %v, want %v", ok, tc.wantAuthorized)
			}

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
		})
	}
}
