# Architecture

This document explains how a request flows through Scrappy.

## Request Flow

1. `main.go` receives HTTP request and applies middleware:
   - request logging
   - IP allowlist (`/health` bypasses allowlist)
   - trusted-proxy-aware client IP extraction (`X-Forwarded-For` / `X-Real-IP` are trusted only from configured proxy CIDRs)
2. `handlers.go` validates method/body, admin token (for `/stats` and `/pool/scale` when configured), and target URL policy.
3. `options.go` resolves defaults into runtime options.
4. Pool execution (`pool_render.go`) runs one of:
   - HTML render
   - Markdown extraction
   - Screenshot capture
5. `pool_page.go` acquires/creates a browser page and applies defaults.
6. `pool_navigation.go` drives navigation/wait/stability/extraction.
7. For screenshots, `r2.go` uploads bytes and returns a public URL.

## Pool Responsibilities

### `pool_types.go`

- Shared data structures and constants.

### `pool_admin.go`

- Pool creation/preload.
- Pool resize and stats.
- Graceful pool shutdown.

### `pool_manager.go`

- Browser instance checkout/release.
- Spawn/reap/hang detection.
- Retains Rod launcher state so browser shutdown can remove temp user-data dirs.
- Event logging and utilization tracking.

### `pool_page.go`

- Builds stealth pages.
- Applies viewport/user-agent defaults (and screenshot DPR when requested).
- Executes common page lifecycle for pooled and standalone modes.
- Standalone fallback on pool checkout failure is controlled by config (`BROWSER_POOL_ALLOW_STANDALONE_FALLBACK`).

### `browser_profiles.go`

- Closes launched browsers with launcher-aware cleanup.
- Periodically removes stale Scrappy-owned temp profile directories left behind by crashed or older Scrappy processes.

### `pool_navigation.go`

- Navigation and settle logic.
- Markdown extraction invocation + fallback to body text.
- Extraction normalizes absolute URLs and skips hidden/invisible nodes.

### `pool_render.go`

- Public methods consumed by handlers (`Render`, `Markdown`, `Screenshot`).

### `extraction_scripts.go`

- Browser-side JS for markdown/text extraction.
- Shared root-selection helper to avoid drift between extraction modes.
- Hidden-node filters cover common dynamic-site patterns such as
  `aria-hidden`, `display:none`, `.w-condition-invisible`, and `.hide`.

## Config Notes

- Render defaults (`SCRAPPY_DEFAULT_*`) are millisecond-based for wait/timeout values.
- Pool timeout values (`BROWSER_POOL_*`) are currently interpreted in seconds.
- Request limits (`SCRAPPY_MAX_*`) cap body size, wait/timeout, viewport, and screenshot device scale factor.
- Chrome profile janitor settings (`SCRAPPY_CHROME_PROFILE_CLEANUP_*`) are interpreted in seconds.
- URL target controls (`SCRAPPY_BLOCK_PRIVATE_NETWORKS`, `SCRAPPY_ALLOWED_TARGET_HOSTS`) apply before browser navigation.

## Operational Notes

- `/stats` is the primary diagnostics endpoint for pool behavior.
- Screenshot endpoint requires R2 config; /html and /markdown do not.
- Kamal deployment templates live in `config/deploy.example.yml` and `.kamal/secrets.example`.

## Tooling Interfaces

- `pkg/client`: typed HTTP client for Scrappy endpoints.
- `cmd/scrappy`: CLI wrapper over `pkg/client` for script and local workflows.
- `cmd/scrappy-mcp`: MCP stdio server that exposes Scrappy operations as MCP tools.
