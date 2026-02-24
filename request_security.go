package main

import (
	"context"
	"errors"
	"net"
	"net/url"
	"strings"
	"time"
)

var extraRestrictedCIDRs = mustParseCIDRs([]string{
	"100.64.0.0/10", // carrier-grade NAT
	"198.18.0.0/15", // benchmarking
	"240.0.0.0/4",   // reserved
})

func validateAndNormalizeTargetURL(cfg Config, rawURL string) (string, error) {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return "", errors.New("url is required")
	}

	parsed, err := url.Parse(trimmed)
	if err != nil || parsed == nil {
		return "", errors.New("url must be a valid absolute URL")
	}

	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", errors.New("url scheme must be http or https")
	}
	if parsed.Host == "" || parsed.Hostname() == "" {
		return "", errors.New("url must include a valid host")
	}
	if parsed.User != nil {
		return "", errors.New("url must not include credentials")
	}

	host := strings.ToLower(strings.TrimSpace(parsed.Hostname()))
	hostAllowed := hostAllowedByPolicy(host, cfg.AllowedTargetHosts)
	allowLoopback := cfg.AllowLoopbackTargets && isLoopbackHost(host)

	if cfg.BlockPrivateNetworks {
		if !allowLoopback {
			if isBlockedHostname(host) {
				return "", errors.New("url host is not allowed")
			}
			if err := ensureHostResolvesToPublicIPs(host); err != nil {
				return "", err
			}
		}
	}

	if !hostAllowed {
		return "", errors.New("url host is not in allowlist")
	}

	parsed.Fragment = ""
	return parsed.String(), nil
}

func isLoopbackHost(host string) bool {
	normalized := strings.Trim(strings.ToLower(host), ".")
	if normalized == "" {
		return false
	}

	if normalized == "localhost" || strings.HasSuffix(normalized, ".localhost") {
		return true
	}

	if ip := net.ParseIP(normalized); ip != nil && ip.IsLoopback() {
		return true
	}

	return false
}

func isBlockedHostname(host string) bool {
	host = strings.Trim(strings.ToLower(host), ".")
	if host == "" {
		return true
	}
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return true
	}
	if host == "local" || strings.HasSuffix(host, ".local") {
		return true
	}
	return false
}

func ensureHostResolvesToPublicIPs(host string) error {
	if ip := net.ParseIP(host); ip != nil {
		if isRestrictedIP(ip) {
			return errors.New("url host is not allowed")
		}
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil || len(ips) == 0 {
		return errors.New("url host could not be resolved")
	}

	for _, ipAddr := range ips {
		if isRestrictedIP(ipAddr.IP) {
			return errors.New("url host is not allowed")
		}
	}

	return nil
}

func isRestrictedIP(ip net.IP) bool {
	if ip == nil {
		return true
	}

	if v4 := ip.To4(); v4 != nil {
		ip = v4
	}

	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsInterfaceLocalMulticast() || ip.IsUnspecified() {
		return true
	}

	for _, cidr := range extraRestrictedCIDRs {
		if cidr.Contains(ip) {
			return true
		}
	}

	return false
}

func hostAllowedByPolicy(host string, allowlist []string) bool {
	if len(allowlist) == 0 {
		return true
	}

	host = strings.ToLower(strings.TrimSpace(host))
	hostIP := net.ParseIP(host)

	for _, entry := range allowlist {
		item := strings.ToLower(strings.TrimSpace(entry))
		if item == "" {
			continue
		}

		if strings.Contains(item, "/") && hostIP != nil {
			if _, cidr, err := net.ParseCIDR(item); err == nil && cidr.Contains(hostIP) {
				return true
			}
			continue
		}

		if strings.HasPrefix(item, "*.") {
			suffix := strings.TrimPrefix(item, "*.")
			if host == suffix || strings.HasSuffix(host, "."+suffix) {
				return true
			}
			continue
		}

		if strings.HasPrefix(item, ".") {
			suffix := strings.TrimPrefix(item, ".")
			if host == suffix || strings.HasSuffix(host, "."+suffix) {
				return true
			}
			continue
		}

		if host == item {
			return true
		}
	}

	return false
}

func mustParseCIDRs(values []string) []*net.IPNet {
	result := make([]*net.IPNet, 0, len(values))
	for _, value := range values {
		_, cidr, err := net.ParseCIDR(value)
		if err == nil {
			result = append(result, cidr)
		}
	}
	return result
}
