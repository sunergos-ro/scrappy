# AGENTS.md

Guidelines for contributors (human or AI agents) working in this repository.

## Purpose

Scrappy provides browser-backed scraping endpoints with a managed Chrome pool. Prioritize reliability and deterministic behavior over cleverness.

## Core Principles

- Keep endpoint contracts stable.
- Prefer small, focused files and composable helpers.
- Add tests for behavior changes, especially in request normalization and extraction logic.
- Avoid hidden side effects in pool lifecycle code.

## Working Agreement

1. Understand the target flow before editing (request -> options -> pool -> response).
2. Make incremental changes and keep diffs easy to review.
3. Run formatting and tests before finalizing.
4. Do not introduce breaking API changes without explicit approval.
5. Update docs when behavior, env vars, or file ownership changes.

## Required Commands

Run after any code change:

```bash
gofmt -w *.go
go test ./...
```

Useful checks:

```bash
rg --files
rg -n "symbol_or_text"
```

## Architecture Map

### HTTP Layer

- `main.go` wires middleware/routes and service boot/shutdown.
- `handlers.go` validates requests and maps to pool operations.
- `models.go` defines API request/response DTOs.
- `options.go` resolves defaults into runtime options.

### Tooling Layer

- `pkg/client/client.go`: typed HTTP client for Scrappy endpoints.
- `cmd/scrappy/main.go`: CLI interface over the typed client.
- `cmd/scrappy-mcp/*.go`: MCP stdio server exposing Scrappy as MCP tools.

### Pool Layer

- `pool_types.go`: pool types/status/constants.
- `pool_admin.go`: constructor, preload, shutdown, scale, stats.
- `pool_render.go`: public render/markdown/screenshot execution.
- `pool_navigation.go`: navigation/stability/extraction helpers.
- `pool_page.go`: page setup and pooled/standalone execution lifecycle.
- `pool_manager.go`: internal pool operations (checkout/spawn/reap/logging).
- `extraction_scripts.go`: JS extraction scripts evaluated in browser.

### External Services

- `r2.go`: screenshot upload and URL composition.
- `config.go`: environment loading and defaults.

## Testing Guidance

Focus tests on:

- Option/default resolution (`options_test.go`).
- Handler helper validation behavior (`handlers_test.go`).
- Any new edge-case extraction behavior when changing scripts.

When touching extraction scripts, validate against at least one dynamic page and one static page.

Suggested smoke checks:

1. `/html` returns expected HTML for a known page.
2. `/markdown` includes expected job/article links for that same page.
3. `/screenshot` succeeds only when R2 is configured.

## Style Rules

- Keep functions small and single-purpose.
- Use explicit names over abbreviations.
- Keep logging informative but not noisy.
- Prefer early returns for validation paths.
- Keep JSON error messages consistent (`{"error":"..."}`).

## Safety

- Do not commit secrets or `.env` contents.
- Do not commit private deployment files (`config/deploy.yml`, `.kamal/secrets`).
- Avoid destructive git operations unless explicitly requested.
- If behavior change is uncertain, add test coverage first, then refactor.

## Documentation Expectations

Update docs when changing:

- Endpoint contract or payload shape.
- Environment variables.
- Pool architecture / file ownership.

At minimum, keep `README.md` and this file accurate.

Also keep `ARCHITECTURE.md` accurate when:

- request flow changes,
- pool files are split/merged,
- extraction script strategy changes.
