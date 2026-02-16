# Contributing

Thanks for contributing to Scrappy.

## Setup

1. Install Go (1.26+), Chrome/Chromium, and git.
2. Clone the repository.
3. Copy env template and set local values:

```bash
cp .env.example .env
```

4. Run tests:

```bash
go test ./...
```

## Development Rules

- Keep endpoint contracts stable.
- Prefer small, focused changes.
- Add or update tests when behavior changes.
- Keep error JSON shape consistent: `{"error":"..."}`

## Required Commands Before PR

```bash
gofmt -w *.go
go test ./...
```

## Pull Requests

- Describe the problem and solution clearly.
- Mention behavior changes and affected endpoints.
- Update docs when changing env vars, request/response shape, or architecture flow.
- Include tests for request normalization and extraction changes when relevant.

## Deployment Templates

- Copy `config/deploy.example.yml` to `config/deploy.yml` for local deployment setup.
- Copy `.kamal/secrets.example` to `.kamal/secrets` and source values from a secret manager or environment.
- Never commit `config/deploy.yml` or `.kamal/secrets`.
- Use `Dockerfile.sample` as a baseline if you do not need custom runtime fonts.

## Security

Do not commit secrets or `.env` contents.

If you find a vulnerability, follow `SECURITY.md`.
