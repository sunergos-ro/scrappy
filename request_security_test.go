package main

import (
	"net"
	"testing"
)

func TestValidateAndNormalizeTargetURL(t *testing.T) {
	tests := []struct {
		name      string
		cfg       Config
		inputURL  string
		wantURL   string
		wantError string
	}{
		{
			name: "accepts valid https and strips fragment",
			cfg: Config{
				BlockPrivateNetworks: false,
			},
			inputURL: "https://example.com/path#section",
			wantURL:  "https://example.com/path",
		},
		{
			name: "rejects non-http scheme",
			cfg: Config{
				BlockPrivateNetworks: false,
			},
			inputURL:  "file:///etc/passwd",
			wantError: "url scheme must be http or https",
		},
		{
			name: "rejects localhost",
			cfg: Config{
				BlockPrivateNetworks: true,
			},
			inputURL:  "http://localhost:8080",
			wantError: "url host is not allowed",
		},
		{
			name: "rejects private ip",
			cfg: Config{
				BlockPrivateNetworks: true,
			},
			inputURL:  "http://10.0.0.5",
			wantError: "url host is not allowed",
		},
		{
			name: "allows localhost when loopback targets are enabled",
			cfg: Config{
				BlockPrivateNetworks: true,
				AllowLoopbackTargets: true,
			},
			inputURL: "http://localhost:8080/preview",
			wantURL:  "http://localhost:8080/preview",
		},
		{
			name: "still rejects private ip even when loopback targets are enabled",
			cfg: Config{
				BlockPrivateNetworks: true,
				AllowLoopbackTargets: true,
			},
			inputURL:  "http://10.0.0.5/internal",
			wantError: "url host is not allowed",
		},
		{
			name: "accepts wildcard host allowlist",
			cfg: Config{
				BlockPrivateNetworks: false,
				AllowedTargetHosts:   []string{"*.example.com"},
			},
			inputURL: "https://api.example.com/v1",
			wantURL:  "https://api.example.com/v1",
		},
		{
			name: "rejects host outside allowlist",
			cfg: Config{
				BlockPrivateNetworks: false,
				AllowedTargetHosts:   []string{"example.com"},
			},
			inputURL:  "https://evil.com",
			wantError: "url host is not in allowlist",
		},
		{
			name: "rejects this-network ipv4",
			cfg: Config{
				BlockPrivateNetworks: true,
			},
			inputURL:  "http://0.0.0.1/",
			wantError: "url host is not allowed",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotURL, err := validateAndNormalizeTargetURL(tc.cfg, tc.inputURL)
			if tc.wantError != "" {
				if err == nil {
					t.Fatalf("expected error %q, got nil", tc.wantError)
				}
				if err.Error() != tc.wantError {
					t.Fatalf("error = %q, want %q", err.Error(), tc.wantError)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gotURL != tc.wantURL {
				t.Fatalf("url = %q, want %q", gotURL, tc.wantURL)
			}
		})
	}
}

func TestHostAllowedByPolicyCIDR(t *testing.T) {
	if !hostAllowedByPolicy("203.0.113.7", []string{"203.0.113.0/24"}) {
		t.Fatalf("expected CIDR allowlist to match host IP")
	}
}

func TestIsRestrictedIPIncludesThisNetwork(t *testing.T) {
	if !isRestrictedIP(mustParseIP(t, "0.0.0.1")) {
		t.Fatalf("expected 0.0.0.1 to be restricted")
	}
	if !isRestrictedIP(mustParseIP(t, "0.255.255.255")) {
		t.Fatalf("expected 0.255.255.255 to be restricted")
	}
	if isRestrictedIP(mustParseIP(t, "203.0.113.10")) {
		t.Fatalf("did not expect documentation IP to be restricted")
	}
}

func TestFetchRequestPolicyRevalidatesRedirectTarget(t *testing.T) {
	privateCfg := Config{
		BlockPrivateNetworks: true,
		AllowedTargetHosts:   []string{"example.com"},
	}
	allowCfg := Config{
		BlockPrivateNetworks: false,
		AllowedTargetHosts:   []string{"example.com"},
	}

	if err := fetchRequestPolicy(privateCfg, "http://10.0.0.5/secret", true); err == nil {
		t.Fatal("expected document redirect to a private IP to be blocked")
	}

	if err := fetchRequestPolicy(allowCfg, "https://evil.com/", true); err == nil {
		t.Fatal("expected document redirect outside the host allowlist to be blocked")
	}

	if err := fetchRequestPolicy(privateCfg, "https://203.0.113.10/app.js", false); err != nil {
		t.Fatalf("public subresource should be allowed: %v", err)
	}

	if err := fetchRequestPolicy(privateCfg, "http://169.254.169.254/latest/meta-data", false); err == nil {
		t.Fatal("expected subresource to a link-local IP to be blocked")
	}

	if err := fetchRequestPolicy(privateCfg, "http://0.0.0.1/", false); err == nil {
		t.Fatal("expected subresource to this-network IP to be blocked")
	}
}

func TestValidateDocumentURLAfterRedirect(t *testing.T) {
	privateCfg := Config{
		BlockPrivateNetworks: true,
		AllowedTargetHosts:   []string{"example.com"},
	}
	if _, err := validateAndNormalizeTargetURL(privateCfg, "http://127.0.0.1/admin"); err == nil {
		t.Fatal("expected final loopback document URL to be rejected")
	}

	allowCfg := Config{
		BlockPrivateNetworks: false,
		AllowedTargetHosts:   []string{"example.com"},
	}
	if _, err := validateAndNormalizeTargetURL(allowCfg, "https://other.example.net/"); err == nil {
		t.Fatal("expected final document URL outside allowlist to be rejected")
	}

	got, err := validateAndNormalizeTargetURL(allowCfg, "https://example.com/after-redirect#frag")
	if err != nil {
		t.Fatalf("unexpected error for allowed final URL: %v", err)
	}
	if got != "https://example.com/after-redirect" {
		t.Fatalf("url = %q, want %q", got, "https://example.com/after-redirect")
	}
}

func mustParseIP(t *testing.T, value string) net.IP {
	t.Helper()
	ip := net.ParseIP(value)
	if ip == nil {
		t.Fatalf("invalid test IP %q", value)
	}
	return ip
}
