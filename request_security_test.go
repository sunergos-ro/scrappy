package main

import "testing"

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
