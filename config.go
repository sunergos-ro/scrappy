package main

import (
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Addr              string
	AllowedIPs        []string
	TrustedProxyCIDRs []string

	AllowedTargetHosts   []string
	BlockPrivateNetworks bool
	AllowLoopbackTargets bool
	AdminToken           string
	MaxRequestBodyBytes  int64
	MaxWaitMS            int
	MaxTimeoutMS         int
	MaxViewportWidth     int
	MaxViewportHeight    int

	PoolEnabled             bool
	PoolMinSize             int
	PoolMaxSize             int
	PoolLeaseTimeout        time.Duration
	PoolIdleTTL             time.Duration
	PoolMaxReuse            int
	PoolSpawnTimeout        time.Duration
	PoolHangTimeout         time.Duration
	PoolSupervisorInterval  time.Duration
	AllowStandaloneFallback bool

	DefaultViewportWidth  int
	DefaultViewportHeight int
	DefaultUserAgent      string
	DefaultWait           time.Duration
	DefaultTimeout        time.Duration
	DefaultFormat         string
	DefaultQuality        int

	ChromeBin       string
	ChromeNoSandbox bool

	R2Endpoint      string
	R2AccessKey     string
	R2SecretKey     string
	R2Bucket        string
	R2PublicBaseURL string
	R2Region        string
}

func LoadConfig() Config {
	cfg := Config{}
	cfg.Addr = envString("SCRAPPY_ADDR", ":3000")
	cfg.AllowedIPs = envStringSlice("SCRAPPY_ALLOWED_IPS", []string{"127.0.0.1", "::1", "172.16.0.0/12", "10.0.0.0/8", "192.168.0.0/16"})
	cfg.TrustedProxyCIDRs = envStringSlice("SCRAPPY_TRUSTED_PROXY_CIDRS", []string{})

	cfg.AllowedTargetHosts = envStringSlice("SCRAPPY_ALLOWED_TARGET_HOSTS", []string{})
	cfg.BlockPrivateNetworks = envBool("SCRAPPY_BLOCK_PRIVATE_NETWORKS", true)
	cfg.AllowLoopbackTargets = envBool("SCRAPPY_ALLOW_LOOPBACK_TARGETS", false)
	cfg.AdminToken = envString("SCRAPPY_ADMIN_TOKEN", "")
	cfg.MaxRequestBodyBytes = envInt64("SCRAPPY_MAX_REQUEST_BODY_BYTES", 1024*1024)
	cfg.MaxWaitMS = envInt("SCRAPPY_MAX_WAIT_MS", 20000)
	cfg.MaxTimeoutMS = envInt("SCRAPPY_MAX_TIMEOUT_MS", 60000)
	cfg.MaxViewportWidth = envInt("SCRAPPY_MAX_VIEWPORT_WIDTH", 2560)
	cfg.MaxViewportHeight = envInt("SCRAPPY_MAX_VIEWPORT_HEIGHT", 2560)

	cfg.PoolEnabled = envBool("BROWSER_POOL_ENABLED", true)
	cfg.PoolMinSize = envIntWithFallback("BROWSER_POOL_MIN_SIZE", "SCRAPPY_POOL_MIN_SIZE", 2)
	cfg.PoolMaxSize = envIntWithFallback("BROWSER_POOL_MAX_SIZE", "SCRAPPY_POOL_MAX_SIZE", 4)
	cfg.PoolLeaseTimeout = envDurationWithFallback("BROWSER_POOL_LEASE_TIMEOUT", "SCRAPPY_POOL_LEASE_TIMEOUT", 5*time.Second)
	cfg.PoolIdleTTL = envDurationWithFallback("BROWSER_POOL_IDLE_TTL", "SCRAPPY_POOL_IDLE_TTL", 120*time.Second)
	cfg.PoolMaxReuse = envIntWithFallback("BROWSER_POOL_MAX_REUSE", "SCRAPPY_POOL_MAX_REUSE", 20)
	cfg.PoolSpawnTimeout = envDurationWithFallback("BROWSER_POOL_SPAWN_TIMEOUT", "SCRAPPY_POOL_SPAWN_TIMEOUT", 30*time.Second)
	cfg.PoolHangTimeout = envDurationWithFallback("BROWSER_POOL_HANG_TIMEOUT", "SCRAPPY_POOL_HANG_TIMEOUT", 120*time.Second)
	cfg.PoolSupervisorInterval = envDurationWithFallback("BROWSER_POOL_SUPERVISOR_INTERVAL", "SCRAPPY_POOL_SUPERVISOR_INTERVAL", 10*time.Second)
	cfg.AllowStandaloneFallback = envBool("BROWSER_POOL_ALLOW_STANDALONE_FALLBACK", false)

	cfg.DefaultViewportWidth = envInt("SCRAPPY_DEFAULT_VIEWPORT_WIDTH", 1440)
	cfg.DefaultViewportHeight = envInt("SCRAPPY_DEFAULT_VIEWPORT_HEIGHT", 756)
	cfg.DefaultUserAgent = envString("SCRAPPY_DEFAULT_USER_AGENT", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	cfg.DefaultWait = envDuration("SCRAPPY_DEFAULT_WAIT_MS", 2000*time.Millisecond)
	cfg.DefaultTimeout = envDuration("SCRAPPY_DEFAULT_TIMEOUT_MS", 30*time.Second)
	cfg.DefaultFormat = envString("SCRAPPY_DEFAULT_FORMAT", "jpeg")
	cfg.DefaultQuality = envInt("SCRAPPY_DEFAULT_QUALITY", 100)

	cfg.ChromeBin = envString("SCRAPPY_CHROME_BIN", "")
	cfg.ChromeNoSandbox = envBool("SCRAPPY_CHROME_NO_SANDBOX", false)

	cfg.R2Endpoint = envString("R2_ENDPOINT", "")
	cfg.R2AccessKey = envString("R2_ACCESS_KEY_ID", "")
	cfg.R2SecretKey = envString("R2_SECRET_ACCESS_KEY", "")
	cfg.R2Bucket = envString("R2_BUCKET", "")
	cfg.R2PublicBaseURL = envString("R2_PUBLIC_BASE_URL", "")
	cfg.R2Region = envString("R2_REGION", "auto")

	return cfg
}

func envString(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func envInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envInt64(key string, fallback int64) int64 {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return fallback
	}
	return parsed
}

func envIntWithFallback(primary, secondary string, fallback int) int {
	value := os.Getenv(primary)
	if value == "" {
		value = os.Getenv(secondary)
	}
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envBool(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envDuration(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return time.Duration(parsed) * time.Millisecond
}

func envDurationWithFallback(primary, secondary string, fallback time.Duration) time.Duration {
	value := os.Getenv(primary)
	if value == "" {
		value = os.Getenv(secondary)
	}
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return time.Duration(parsed) * time.Second
}

func envStringSlice(key string, fallback []string) []string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// IPAllowlist checks if an IP is in the allowed list (supports CIDR notation)
type IPAllowlist struct {
	cidrs []*net.IPNet
	ips   map[string]bool
}

func NewIPAllowlist(allowed []string) *IPAllowlist {
	al := &IPAllowlist{
		cidrs: make([]*net.IPNet, 0),
		ips:   make(map[string]bool),
	}
	for _, entry := range allowed {
		if strings.Contains(entry, "/") {
			_, cidr, err := net.ParseCIDR(entry)
			if err == nil {
				al.cidrs = append(al.cidrs, cidr)
			}
		} else {
			al.ips[entry] = true
		}
	}
	return al
}

func (al *IPAllowlist) IsAllowed(ip string) bool {
	// Check exact match
	if al.ips[ip] {
		return true
	}
	// Check CIDR ranges
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}
	for _, cidr := range al.cidrs {
		if cidr.Contains(parsed) {
			return true
		}
	}
	return false
}
