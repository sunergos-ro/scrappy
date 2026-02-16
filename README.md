# Scrappy

Scrappy is a Go HTTP service that uses a warm Chrome pool to provide fast rendering and extraction APIs.

## Endpoints

| Method | Path | Purpose |
| --- | --- | --- |
| `POST` | `/html` | Return page HTML |
| `POST` | `/markdown` | Return extracted markdown-like content |
| `POST` | `/screenshot` | Capture screenshot and upload to R2 |
| `POST` | `/pool/scale` | Resize browser pool (admin token required when configured) |
| `GET` | `/stats` | Inspect pool health/utilization (admin token required when configured) |
| `GET` | `/health` | Liveness endpoint (bypasses IP allowlist) |

## Requirements

- Go 1.26+
- Chrome/Chromium available (or let Rod launcher manage it)
- Optional: Cloudflare R2 credentials for `/screenshot`

## Quick Start

1. Install dependencies:

```bash
go mod download
```

2. Configure environment:

```bash
cp .env.example .env
```

Then edit `.env` and set required values.

3. Start service:

```bash
go run .
```

Default bind address is `:3000` (`SCRAPPY_ADDR`).

If calling from a non-local IP in development, set `SCRAPPY_ALLOWED_IPS` accordingly.

## API Usage

### Render HTML

```bash
curl -X POST http://localhost:3000/html \
  -H "Content-Type: application/json" \
  -d '{"url":"https://example.com","viewport":{"width":1280,"height":800}}'
```

### Extract Markdown

```bash
curl -X POST http://localhost:3000/markdown \
  -H "Content-Type: application/json" \
  -d '{"url":"https://example.com"}'
```

### Capture Screenshot

```bash
curl -X POST http://localhost:3000/screenshot \
  -H "Content-Type: application/json" \
  -d '{"url":"https://example.com","viewport":{"width":1440,"height":756},"format":"jpeg","quality":90}'
```

### Scale Pool

```bash
curl -X POST http://localhost:3000/pool/scale \
  -H "Content-Type: application/json" \
  -d '{"size":3}'
```

## CLI

A local CLI is available at `cmd/scrappy` for script and agent workflows.

Run via Go:

```bash
go run ./cmd/scrappy --help
```

Example commands:

```bash
go run ./cmd/scrappy --base-url http://localhost:3000 html \
  --url https://example.com

go run ./cmd/scrappy --base-url http://localhost:3000 markdown \
  --url https://example.com --wait-ms 1500

go run ./cmd/scrappy --base-url http://localhost:3000 screenshot \
  --url https://example.com --format webp --quality 90

go run ./cmd/scrappy --base-url http://localhost:3000 stats
go run ./cmd/scrappy --base-url http://localhost:3000 scale --size 3
```

Global CLI flags:

- `--base-url` (env: `SCRAPPY_BASE_URL`, default `http://localhost:3000`)
- `--admin-token` (env: `SCRAPPY_ADMIN_TOKEN`)
- `--http-timeout-ms` (env: `SCRAPPY_HTTP_TIMEOUT_MS`, default `80000`)
- `--pretty` (pretty-print JSON output)

## Request Fields

Common request fields for `/html` and `/markdown`:

- `url` (required)
- `viewport.width` / `viewport.height` (optional)
- `user_agent` (optional)
- `wait_ms` (optional)
- `timeout_ms` (optional)

Request constraints:

- URL must be absolute `http://` or `https://`.
- URL credentials (`https://user:pass@...`) are rejected.
- Private/local network targets are blocked by default.
- `wait_ms` / `timeout_ms` / viewport are capped by server limits.

Additional fields for `/screenshot`:

- `format` (`jpeg`, `png`, `webp`)
- `quality` (ignored for png)

## Configuration

Key environment variables:

### Server

- `SCRAPPY_ADDR` (default `:3000`)
- `SCRAPPY_ALLOWED_IPS` (comma-separated IPs/CIDRs)
- `SCRAPPY_TRUSTED_PROXY_CIDRS` (comma-separated proxy CIDRs allowed to set `X-Forwarded-For` / `X-Real-IP`)

### Security Controls

- `SCRAPPY_ALLOWED_TARGET_HOSTS` (optional comma-separated host allowlist; supports exact host, `.example.com`, `*.example.com`, and CIDR for IP targets)
- `SCRAPPY_BLOCK_PRIVATE_NETWORKS` (default `true`; blocks localhost/private/link-local/reserved targets)
- `SCRAPPY_ADMIN_TOKEN` (optional; protects `/stats` and `/pool/scale` when set; use `Authorization: Bearer <token>` or `X-Admin-Token`)
- `SCRAPPY_MAX_REQUEST_BODY_BYTES` (default `1048576`)
- `SCRAPPY_MAX_WAIT_MS` (default `20000`)
- `SCRAPPY_MAX_TIMEOUT_MS` (default `60000`)
- `SCRAPPY_MAX_VIEWPORT_WIDTH` (default `2560`)
- `SCRAPPY_MAX_VIEWPORT_HEIGHT` (default `2560`)

### Browser Pool

- `BROWSER_POOL_ENABLED` (default `true`)
- `BROWSER_POOL_MIN_SIZE`, `BROWSER_POOL_MAX_SIZE`
- `BROWSER_POOL_LEASE_TIMEOUT`
- `BROWSER_POOL_IDLE_TTL`
- `BROWSER_POOL_MAX_REUSE`
- `BROWSER_POOL_SPAWN_TIMEOUT`
- `BROWSER_POOL_HANG_TIMEOUT`
- `BROWSER_POOL_SUPERVISOR_INTERVAL`
- `BROWSER_POOL_ALLOW_STANDALONE_FALLBACK` (default `false`)

Note: pool timeout vars above are interpreted as seconds (legacy behavior in config loader).  
Legacy aliases (`SCRAPPY_POOL_*`) are still supported for pool size/timeouts.

### Render Defaults

- `SCRAPPY_DEFAULT_VIEWPORT_WIDTH`
- `SCRAPPY_DEFAULT_VIEWPORT_HEIGHT`
- `SCRAPPY_DEFAULT_USER_AGENT`
- `SCRAPPY_DEFAULT_WAIT_MS`
- `SCRAPPY_DEFAULT_TIMEOUT_MS`
- `SCRAPPY_DEFAULT_FORMAT`
- `SCRAPPY_DEFAULT_QUALITY`

### Browser Binary

- `SCRAPPY_CHROME_BIN` (optional explicit Chrome/Chromium binary)
- `SCRAPPY_CHROME_NO_SANDBOX` (default `false`; keep disabled unless strictly required)

### R2 (required only for `/screenshot`)

- `R2_ENDPOINT`
- `R2_ACCESS_KEY_ID`
- `R2_SECRET_ACCESS_KEY`
- `R2_BUCKET`
- `R2_PUBLIC_BASE_URL`
- `R2_REGION` (default `auto`)

### Observability

- `SENTRY_DSN` (optional)

## Project Layout

Pool code is now split by responsibility:

- `pool_types.go` - types/constants
- `pool_admin.go` - constructor/stats/scale/shutdown
- `pool_render.go` - public render/markdown/screenshot methods
- `pool_navigation.go` - navigation, settle, extraction helpers
- `pool_page.go` - page lifecycle/setup
- `pool_manager.go` - pool internals (spawn/checkout/reap/logging)
- `extraction_scripts.go` - browser-evaluated extraction scripts

Request parsing/defaults:

- `handlers.go`
- `options.go`
- `models.go`

For flow-level detail, see `ARCHITECTURE.md`.

## Development

Format and test before committing:

```bash
gofmt -w *.go
go test ./...
```

## Security Notes

- Do not expose this service publicly without network controls and authentication.
- Keep `SCRAPPY_ALLOWED_IPS` restricted to trusted callers.
- Configure `SCRAPPY_TRUSTED_PROXY_CIDRS` when running behind a reverse proxy.
- Keep `SCRAPPY_CHROME_NO_SANDBOX=false` in production.
- Report vulnerabilities using the process in `SECURITY.md`.

## Troubleshooting Markdown Extraction

For dynamic pages (for example Webflow job pages), extraction can fail if links only exist in hidden or late-rendered nodes.

- The extractor ignores hidden content (`display:none`, `visibility:hidden`, `aria-hidden="true"`, `.w-condition-invisible`, `.hide`).
- Root selection prefers semantic/main containers and falls back to `document.body`.
- Link extraction converts relative URLs to absolute URLs using `document.baseURI`.

Debug sequence for extraction issues:

1. Call `/html` for the same URL and confirm the target `<a href>` exists in rendered HTML.
2. Verify the link is not inside hidden variants/duplicated mobile or desktop nav containers.
3. Re-run `/markdown` with a larger `wait_ms` if content is injected after initial paint.
4. Check `/stats` for pool errors, stale pages, or repeated timeouts.

## Deployment

- Dockerized via:
  - `Dockerfile` (current project image with extra custom fonts)
  - `Dockerfile.sample` (generic baseline image without custom font bundle)
- Kamal templates are included:
  - `config/deploy.example.yml`
  - `.kamal/secrets.example`
- Create local deployment files before running Kamal:

```bash
cp config/deploy.example.yml config/deploy.yml
cp .kamal/secrets.example .kamal/secrets
```

- Edit local copies with your registry, hosts, secrets source, and SSH user.
- Keep `config/deploy.yml` and `.kamal/secrets` private; both are gitignored by default.

## License

MIT. See `LICENSE`.
